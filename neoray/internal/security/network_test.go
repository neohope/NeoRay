package security

import (
	"net/netip"
	"testing"
)

func TestIsPrivate(t *testing.T) {
	// Reset to default blocked networks for testing
	initBlockedNetworks()
	allowedNetworks = nil

	tests := []struct {
		name     string
		addr     string
		expected bool
	}{
		{"loopback v4", "127.0.0.1", true},
		{"loopback v6", "::1", true},
		{"private 10.x", "10.0.0.1", true},
		{"private 172.x", "172.16.0.1", true},
		{"private 192.168.x", "192.168.1.1", true},
		{"link-local", "169.254.1.1", true},
		{"multicast v4", "224.0.0.1", true},
		{"multicast v6", "ff02::1", true},
		{"cgnet", "100.64.0.1", true},
		{"public 8.8.8.8", "8.8.8.8", false},
		{"public 1.1.1.1", "1.1.1.1", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			addr := netip.MustParseAddr(tt.addr)
			result := isPrivate(addr)
			if result != tt.expected {
				t.Errorf("isPrivate(%s) = %v, want %v", tt.addr, result, tt.expected)
			}
		})
	}
}

func TestConfigureSSRFWhitelist(t *testing.T) {
	// Reset state
	initBlockedNetworks()

	// Configure with valid and invalid CIDRs
	ConfigureSSRFWhitelist([]string{
		"8.8.8.8/32",
		"invalid-cidr",
		"1.1.1.1/32",
	})

	allowed := GetAllowedNetworks()
	if len(allowed) != 2 {
		t.Errorf("Expected 2 allowed networks, got %d", len(allowed))
	}
}

func TestNormalizeAddr(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{"ipv4", "192.168.1.1", "192.168.1.1"},
		{"ipv6 loopback", "::1", "::1"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			addr := netip.MustParseAddr(tt.input)
			result := normalizeAddr(addr)
			if result.String() != tt.expected {
				t.Errorf("normalizeAddr(%s) = %s, want %s", tt.input, result.String(), tt.expected)
			}
		})
	}
}
