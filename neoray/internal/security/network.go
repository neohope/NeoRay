package security

import (
	"context"
	"fmt"
	"net"
	"net/netip"
	"net/url"
	"regexp"
	"strings"
	"sync"
	"time"

	"neoray/internal/logger"
)

var (
	// P1-4: 支持 IPv6 URL（如 http://[::1]:8080/），同时防止 SSRF 绕过
	urlRe = regexp.MustCompile(`(?i)https?://(?:\[[\da-fA-F:]+\]|[^\s"'` + "`|<>\\[\\]]+)")

	blockedNetworks []netip.Prefix
	allowedNetworks []netip.Prefix

	netMu sync.RWMutex
)

func init() {
	initBlockedNetworks()
}

func initBlockedNetworks() {
	netMu.Lock()
	defer netMu.Unlock()

	// Blocked networks from reference implementation + additional SSRF vectors
	blockedNetworks = []netip.Prefix{
		// Loopback
		netip.MustParsePrefix("0.0.0.0/8"),
		netip.MustParsePrefix("127.0.0.0/8"),
		netip.MustParsePrefix("::1/128"),
		// Private / reserved
		netip.MustParsePrefix("10.0.0.0/8"),
		netip.MustParsePrefix("100.64.0.0/10"),  // CGNAT
		netip.MustParsePrefix("172.16.0.0/12"),
		netip.MustParsePrefix("192.168.0.0/16"),
		// Link-local
		netip.MustParsePrefix("169.254.0.0/16"),
		netip.MustParsePrefix("fe80::/10"),
		// IPv6 unique local
		netip.MustParsePrefix("fc00::/7"),
		// Multicast
		netip.MustParsePrefix("224.0.0.0/4"),
		netip.MustParsePrefix("ff00::/8"),
		// Broadcast / reserved
		netip.MustParsePrefix("255.255.255.255/32"),
		// Documentation / test nets
		netip.MustParsePrefix("192.0.2.0/24"),    // TEST-NET-1
		netip.MustParsePrefix("198.51.100.0/24"), // TEST-NET-2
		netip.MustParsePrefix("203.0.113.0/24"),  // TEST-NET-3
		netip.MustParsePrefix("192.0.0.0/24"),    // IETF Protocol Assignments
		netip.MustParsePrefix("198.18.0.0/15"),   // Benchmarking
	}
}

// ConfigureSSRFWhitelist configures the whitelist of CIDR ranges that bypass SSRF protection.
// Invalid CIDR entries are logged as warnings rather than silently discarded.
func ConfigureSSRFWhitelist(cidrs []string) {
	netMu.Lock()
	defer netMu.Unlock()

	var newAllowed []netip.Prefix
	for _, cidr := range cidrs {
		p, err := netip.ParsePrefix(cidr)
		if err != nil {
			logger.Warn("Ignoring invalid CIDR in SSRF whitelist",
				logger.String("cidr", cidr),
				logger.String("error", err.Error()),
			)
			continue
		}
		newAllowed = append(newAllowed, p)
	}
	allowedNetworks = newAllowed
}

// normalizeAddr normalizes IPv6-mapped IPv4 addresses
func normalizeAddr(addr netip.Addr) netip.Addr {
	if addr.Is6() && addr.Is4In6() {
		ip4 := addr.As4()
		return netip.AddrFrom4(ip4)
	}
	return addr
}

// isPrivate checks if an address is in blocked private networks
func isPrivate(addr netip.Addr) bool {
	addr = normalizeAddr(addr)

	netMu.RLock()
	defer netMu.RUnlock()

	for _, allowed := range allowedNetworks {
		if allowed.Contains(addr) {
			return false
		}
	}

	for _, blocked := range blockedNetworks {
		if blocked.Contains(addr) {
			return true
		}
	}

	return false
}

// isAllowedLoopbackTarget checks if hostname and addresses are allowed for loopback
func isAllowedLoopbackTarget(hostname string, addrs []netip.Addr) bool {
	if len(addrs) == 0 {
		return false
	}

	for _, addr := range addrs {
		if !normalizeAddr(addr).IsLoopback() {
			return false
		}
	}

	normalized := strings.ToLower(strings.TrimRight(hostname, "."))
	if normalized == "localhost" {
		return true
	}

	if addr, err := netip.ParseAddr(hostname); err == nil {
		return normalizeAddr(addr).IsLoopback()
	}

	return false
}

// resolveHost resolves a hostname to IP addresses
func resolveHost(hostname string) ([]netip.Addr, error) {
	host, _, err := net.SplitHostPort(hostname)
	if err == nil {
		hostname = host
	}

	ips, err := net.LookupIP(hostname)
	if err != nil {
		return nil, err
	}

	var addrs []netip.Addr
	for _, ip := range ips {
		if addr, ok := netip.AddrFromSlice(ip); ok {
			addrs = append(addrs, addr)
		}
	}
	return addrs, nil
}

// ValidateURLTarget validates a URL is safe to fetch, checking scheme, hostname, and resolved IPs.
// It returns the resolved addresses alongside the validation result so callers can connect
// directly to a pre-resolved address, eliminating DNS rebinding TOCTOU windows.
func ValidateURLTarget(urlStr string, allowLoopback bool) (bool, string, []netip.Addr) {
	u, err := url.Parse(urlStr)
	if err != nil {
		return false, err.Error(), nil
	}

	if u.Scheme != "http" && u.Scheme != "https" {
		return false, fmt.Sprintf("Only http/https allowed, got '%s'", u.Scheme), nil
	}
	if u.Host == "" {
		return false, "Missing domain", nil
	}

	hostname := u.Hostname()
	if hostname == "" {
		return false, "Missing hostname", nil
	}

	addrs, err := resolveHost(hostname)
	if err != nil {
		return false, fmt.Sprintf("Cannot resolve hostname: %s", hostname), nil
	}

	if allowLoopback && isAllowedLoopbackTarget(hostname, addrs) {
		return true, "", addrs
	}

	for _, addr := range addrs {
		if isPrivate(addr) {
			return false, fmt.Sprintf("Blocked: %s resolves to private/internal address %s", hostname, addr), nil
		}
	}

	return true, "", addrs
}

// ValidateResolvedURL validates an already-fetched URL (e.g., after redirect).
// P1-5: added allowLoopback parameter to enforce loopback policy consistently with ValidateURLTarget.
func ValidateResolvedURL(urlStr string, allowLoopback bool) (bool, string) {
	u, err := url.Parse(urlStr)
	if err != nil {
		return false, fmt.Sprintf("Redirect target URL is malformed: %v", err)
	}

	hostname := u.Hostname()
	if hostname == "" {
		return false, "Redirect target URL has no hostname"
	}

	addrs, err := resolveHost(hostname)
	if err != nil {
		return false, fmt.Sprintf("Failed to resolve redirect target hostname %q: %v", hostname, err)
	}

	// P1-5: 检查 loopback 策略，与 ValidateURLTarget 保持一致
	if allowLoopback && isAllowedLoopbackTarget(hostname, addrs) {
		return true, ""
	}

	for _, a := range addrs {
		if isPrivate(a) {
			return false, fmt.Sprintf("Redirect target %s resolves to private address %s", hostname, a)
		}
	}

	return true, ""
}

// ContainsInternalURL checks if the command string contains a URL targeting an internal/private address
func ContainsInternalURL(command string, allowLoopback bool) bool {
	matches := urlRe.FindAllString(command, -1)
	for _, urlStr := range matches {
		ok, _, _ := ValidateURLTarget(urlStr, allowLoopback)
		if !ok {
			return true
		}
	}
	return false
}

// GetBlockedNetworks returns the list of blocked networks for debugging
func GetBlockedNetworks() []string {
	netMu.RLock()
	defer netMu.RUnlock()

	var result []string
	for _, p := range blockedNetworks {
		result = append(result, p.String())
	}
	return result
}

// GetAllowedNetworks returns the list of allowed networks from whitelist for debugging
func GetAllowedNetworks() []string {
	netMu.RLock()
	defer netMu.RUnlock()

	var result []string
	for _, p := range allowedNetworks {
		result = append(result, p.String())
	}
	return result
}

// NewPreResolvedDialer creates a net.Dialer that resolves DNS once via ValidateURLTarget
// and connects directly to the pre-resolved IP address. This eliminates the TOCTOU window
// between DNS validation and TCP connection (DNS rebinding attack).
//
// Usage:
//
//	addrs, dialer, err := security.NewPreResolvedDialer(urlStr, allowLoopback, 10*time.Second)
//	if err != nil { return err }
//	transport := &http.Transport{DialContext: dialer.DialContext}
//	client := &http.Client{Transport: transport}
func NewPreResolvedDialer(urlStr string, allowLoopback bool, timeout time.Duration) ([]netip.Addr, *PreResolvedDialer, error) {
	valid, errMsg, addrs := ValidateURLTarget(urlStr, allowLoopback)
	if !valid {
		return nil, nil, fmt.Errorf("URL validation failed: %s", errMsg)
	}
	if len(addrs) == 0 {
		return nil, nil, fmt.Errorf("no addresses resolved for %s", urlStr)
	}

	u, err := url.Parse(urlStr)
	if err != nil {
		return nil, nil, fmt.Errorf("parse URL: %w", err)
	}

	port := u.Port()
	if port == "" {
		switch u.Scheme {
		case "https":
			port = "443"
		default:
			port = "80"
		}
	}

	return addrs, &PreResolvedDialer{
		addrs:   addrs,
		port:    port,
		timeout: timeout,
	}, nil
}

// PreResolvedDialer dials using pre-resolved IP addresses, preventing DNS rebinding.
type PreResolvedDialer struct {
	addrs   []netip.Addr
	port    string
	timeout time.Duration
}

// DialContext implements a dialer that connects to pre-resolved IPs without re-resolving DNS.
func (d *PreResolvedDialer) DialContext(ctx context.Context, network, addr string) (net.Conn, error) {
	var dialer net.Dialer
	if d.timeout > 0 {
		dialer.Timeout = d.timeout
	}

	var lastErr error
	for _, addr := range d.addrs {
		tcpAddr := net.TCPAddrFromAddrPort(netip.AddrPortFrom(addr, mustParsePort(d.port)))
		conn, err := dialer.DialContext(ctx, "tcp", tcpAddr.String())
		if err == nil {
			return conn, nil
		}
		lastErr = err
	}

	if lastErr != nil {
		return nil, lastErr
	}
	return nil, fmt.Errorf("no addresses to dial")
}

// mustParsePort parses a port string to uint16, panicking on failure.
func mustParsePort(s string) uint16 {
	var port uint16
	for _, c := range s {
		if c >= '0' && c <= '9' {
			port = port*10 + uint16(c-'0')
		}
	}
	return port
}
