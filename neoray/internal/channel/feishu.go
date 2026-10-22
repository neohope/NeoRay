package channel

import (
	"bytes"
	"container/list"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	lark "github.com/larksuite/oapi-sdk-go/v3"
	"github.com/larksuite/oapi-sdk-go/v3/event/dispatcher"
	larkim "github.com/larksuite/oapi-sdk-go/v3/service/im/v1"
	larkws "github.com/larksuite/oapi-sdk-go/v3/ws"
	"neoray/internal/agent"
	"neoray/internal/bus"
	"neoray/internal/config"
	"neoray/internal/logger"
	"neoray/internal/session"
)

// extractReceiveIDFromChatID 从会话 ID 中提取真实的接收者 ID
func extractReceiveIDFromChatID(chatID string) string {
	if !strings.HasPrefix(chatID, "feishu:") {
		return chatID
	}
	parts := strings.SplitN(chatID[len("feishu:"):], ":", 2)
	return parts[0]
}

const (
	feishuDomain      = "https://open.feishu.cn"
	larkDomain        = "https://open.larksuite.com"
	maxFeishuBodySize = 10 * 1024 * 1024 // 10 MB max response body
	// defaultDedupCapacity 已处理消息 ID 的默认缓存容量。
	// 1000 在繁忙群聊里可能只覆盖几分钟，飞书重投时会重复回复。
	defaultDedupCapacity = 5000
)

var (
	tableRegex            = regexp.MustCompile(`(?m)((?:^[ \t]*\|.+\|[ \t]*\n)(?:^[ \t]*\|[-:\s|]+\|[ \t]*\n)(?:^[ \t]*\|.+\|[ \t]*\n?)+)`)
	headingRegex          = regexp.MustCompile(`(?m)^(#{1,6})\s+(.+)$`)
	codeBlockRegex        = regexp.MustCompile("(?s)(```.*?```)")
	mdBoldRegex           = regexp.MustCompile(`\*\*(.+?)\*\*`)
	mdBoldUnderscoreRegex = regexp.MustCompile(`__(.+?)__`)
	mdItalicRegex         = regexp.MustCompile(`(?m)(^|[^*])\*([^*\n]+?)\*([^*]|$)`)
	mdStrikeRegex         = regexp.MustCompile(`~~(.+?)~~`)
	mdLinkRegex           = regexp.MustCompile(`\[([^\]]+)\]\((https?://[^\)]+)\)`)
	listRegex             = regexp.MustCompile(`(?m)^[\s]*[-*+]\s+`)
	orderedListRegex      = regexp.MustCompile(`(?m)^[\s]*\d+\.\s+`)

	imageExts   = map[string]bool{".png": true, ".jpg": true, ".jpeg": true, ".gif": true, ".bmp": true, ".webp": true, ".ico": true, ".tiff": true, ".tif": true}
	audioExts   = map[string]bool{".opus": true}
	videoExts   = map[string]bool{".mp4": true, ".mov": true, ".avi": true}
	fileTypeMap = map[string]string{
		".opus": "opus",
		".mp4":  "mp4",
		".pdf":  "pdf",
		".doc":  "doc",
		".docx": "doc",
		".xls":  "xls",
		".xlsx": "xls",
		".ppt":  "ppt",
		".pptx": "ppt",
	}
)

type FeishuConfig struct {
	AppID             string
	AppSecret         string
	Enabled           bool
	EncryptKey        string
	VerificationToken string
	Domain            string
	GroupPolicy       string
	ReplyToMessage    bool
	TopicIsolation    bool
	ReactEmoji        string
	DoneEmoji         string
	ToolHintPrefix    string
	Streaming         bool
	// DedupCapacity 已处理消息 ID 的缓存容量（默认 5000）
	DedupCapacity int
	// MaxMessageChars 单条回复的字符上限，超出按段落分多条发送（默认 8000）
	MaxMessageChars int
}

type FeishuChannel struct {
	cfg                 *FeishuConfig
	appConfig           *config.Config
	agent               agent.AgentInterface
	sessionMgr          *session.Manager
	tenantToken         string
	tokenExpiry         time.Time
	tokenTTL            time.Duration
	httpClient          *http.Client
	larkClient          *lark.Client // 飞书 SDK 客户端
	wsClient            *larkws.Client
	wsCancel            context.CancelFunc
	botOpenID           string
	processedMessageIDs *orderedMap
	reactionIDs         map[string]string
	mu                  sync.RWMutex
	running             bool
	stopChan            chan struct{}
	streamBufs          map[string]*streamBuf
}

// orderedMap 是一个线程安全的 LRU 去重集合。
// 使用 container/list 实现 O(1) 的访问提升和淘汰。
type orderedMap struct {
	mu       sync.RWMutex
	saveMu   sync.Mutex // 串行化文件写入，避免并发写坏持久化文件
	data     map[string]*list.Element
	order    *list.List
	cap      int
	filePath string // 持久化文件路径，为空则不持久化

	// 批量落盘：以前每来一条新消息就整表重写一次文件（O(n) 写盘/消息，
	// 群聊繁忙时 goroutine 还会排队堆积）。现在只标记 dirty，
	// 由单个后台 goroutine 每秒合并一次；Stop 时再刷一次保证不丢。
	dirty       atomic.Bool
	flushOnce   sync.Once
	flushStop   chan struct{}
	flushDone   chan struct{}
	flushPeriod time.Duration
}

func newOrderedMap(capacity int) *orderedMap {
	return &orderedMap{
		data:        make(map[string]*list.Element),
		order:       list.New(),
		cap:         capacity,
		flushStop:   make(chan struct{}),
		flushDone:   make(chan struct{}),
		flushPeriod: time.Second,
	}
}

// startFlusher 启动唯一的后台落盘 goroutine（幂等）
func (o *orderedMap) startFlusher() {
	if o.filePath == "" {
		return
	}
	o.flushOnce.Do(func() {
		go func() {
			defer close(o.flushDone)
			period := o.flushPeriod
			if period <= 0 {
				period = time.Second
			}
			ticker := time.NewTicker(period)
			defer ticker.Stop()
			for {
				select {
				case <-o.flushStop:
					// 退出前把未落盘的改动刷掉
					if o.dirty.Swap(false) {
						o.saveToFile()
					}
					return
				case <-ticker.C:
					if o.dirty.Swap(false) {
						o.saveToFile()
					}
				}
			}
		}()
	})
}

// stop 停止后台落盘并做最后一次刷盘
func (o *orderedMap) stop() {
	o.startFlusher() // 确保 flushDone 会被关闭
	select {
	case <-o.flushStop:
		// 已停止
	default:
		close(o.flushStop)
	}
	select {
	case <-o.flushDone:
	case <-time.After(3 * time.Second):
		logger.Warn("Timed out waiting for processed message IDs flush")
	}
}

// markDirty 标记有变更，交由后台 goroutine 合并落盘
func (o *orderedMap) markDirty() {
	if o.filePath == "" {
		return
	}
	o.dirty.Store(true)
	o.startFlusher()
}

// loadFromFile 从文件加载已处理的消息 ID
func (o *orderedMap) loadFromFile(path string) {
	o.filePath = path
	data, err := os.ReadFile(path)
	if err != nil {
		if !os.IsNotExist(err) {
			logger.Warn("Failed to load processed message IDs", logger.ErrorField(err))
		}
		return
	}
	var ids []string
	if err := json.Unmarshal(data, &ids); err != nil {
		logger.Warn("Failed to parse processed message IDs", logger.ErrorField(err))
		return
	}
	o.mu.Lock()
	for _, id := range ids {
		if _, exists := o.data[id]; !exists {
			elem := o.order.PushBack(id)
			o.data[id] = elem
		}
	}
	// 超出容量时淘汰最旧的
	for o.order.Len() > o.cap {
		oldest := o.order.Front()
		if oldest != nil {
			if key, ok := oldest.Value.(string); ok {
				delete(o.data, key)
			}
			o.order.Remove(oldest)
		}
	}
	o.mu.Unlock()
	logger.Info("Loaded processed message IDs", logger.Int("count", len(o.data)))
}

// saveToFile 将已处理的消息 ID 持久化到文件。
// 写入前先落临时文件再 rename，保证读端永远看到完整 JSON；并用 saveMu 串行化，
// 避免多个 goroutine 并发写同一文件导致内容损坏（损坏后重启会丢失去重记录）。
func (o *orderedMap) saveToFile() {
	if o.filePath == "" {
		return
	}

	o.saveMu.Lock()
	defer o.saveMu.Unlock()

	o.mu.RLock()
	ids := make([]string, 0, len(o.data))
	for e := o.order.Front(); e != nil; e = e.Next() {
		if id, ok := e.Value.(string); ok {
			ids = append(ids, id)
		}
	}
	o.mu.RUnlock()
	data, err := json.Marshal(ids)
	if err != nil {
		logger.Warn("Failed to marshal processed message IDs", logger.ErrorField(err))
		return
	}

	tmpPath := o.filePath + ".tmp"
	if err := os.WriteFile(tmpPath, data, 0644); err != nil {
		logger.Warn("Failed to write processed message IDs", logger.ErrorField(err))
		return
	}
	if err := os.Rename(tmpPath, o.filePath); err != nil {
		logger.Warn("Failed to save processed message IDs", logger.ErrorField(err))
	}
}

// addIfNew 原子地"检查并加入"，返回 false 表示该 key 已存在（重复消息）。
// 取代原先 has() + add() 两步调用，消除并发下两个 goroutine 同时通过去重的竞态。
func (o *orderedMap) addIfNew(key string) bool {
	o.mu.Lock()
	if elem, ok := o.data[key]; ok {
		// 已存在，提升到末尾并报告重复
		o.order.MoveToBack(elem)
		o.mu.Unlock()
		return false
	}
	// 容量已满，淘汰最久未使用的（链表头部）
	if o.order.Len() >= o.cap {
		oldest := o.order.Front()
		if oldest != nil {
			if oldKey, ok := oldest.Value.(string); ok {
				delete(o.data, oldKey)
			}
			o.order.Remove(oldest)
		}
	}
	elem := o.order.PushBack(key)
	o.data[key] = elem
	o.mu.Unlock()

	o.markDirty()
	return true
}

type streamBuf struct {
	text      string
	messageID string // 使用消息ID替代卡片ID
	lastEdit  float64
	createdAt time.Time
}

func NewFeishuChannel(cfg *FeishuConfig, appConfig *config.Config, aiAgent agent.AgentInterface, sessionMgr *session.Manager) *FeishuChannel {
	// 创建 lark SDK 客户端
	larkClient := lark.NewClient(cfg.AppID, cfg.AppSecret)

	dedupCapacity := cfg.DedupCapacity
	if dedupCapacity <= 0 {
		dedupCapacity = defaultDedupCapacity
	}
	processedIDs := newOrderedMap(dedupCapacity)

	// 从文件加载已处理的消息 ID，防止重启后重复处理
	if appConfig != nil && appConfig.HomeDir != "" {
		dataDir := filepath.Join(appConfig.HomeDir, "data")
		if mkErr := os.MkdirAll(dataDir, 0755); mkErr == nil {
			processedIDs.loadFromFile(filepath.Join(dataDir, "feishu_processed_messages.json"))
		}
	}

	return &FeishuChannel{
		cfg:                 cfg,
		appConfig:           appConfig,
		agent:               aiAgent,
		sessionMgr:          sessionMgr,
		httpClient:          &http.Client{Timeout: 30 * time.Second},
		larkClient:          larkClient,
		stopChan:            make(chan struct{}),
		processedMessageIDs: processedIDs,
		reactionIDs:         make(map[string]string),
		streamBufs:          make(map[string]*streamBuf),
	}
}

func (f *FeishuChannel) Name() string {
	return "feishu"
}

func (f *FeishuChannel) getDomain() string {
	if f.cfg.Domain == "lark" {
		return larkDomain
	}
	return feishuDomain
}

func (f *FeishuChannel) Start() error {
	if !f.cfg.Enabled {
		logger.Info("Feishu channel disabled")
		return nil
	}

	f.mu.Lock()
	if f.running {
		f.mu.Unlock()
		return nil
	}
	f.running = true
	f.mu.Unlock()

	logger.Info("Starting Feishu channel with WebSocket long connection")

	if err := f.refreshToken(); err != nil {
		logger.Warn("Failed to get feishu token on startup", logger.ErrorField(err))
	}

	go func() {
		if err := f.fetchBotOpenID(); err != nil {
			logger.Warn("Failed to fetch bot open_id", logger.ErrorField(err))
		}
	}()

	go f.tokenRefreshLoop()
	go f.streamBufCleanupLoop()

	wsCtx, cancel := context.WithCancel(context.Background())
	f.mu.Lock()
	f.wsCancel = cancel
	f.mu.Unlock()
	f.startSDKWebSocket(wsCtx)

	logger.Info("Feishu channel started")
	return nil
}

func (f *FeishuChannel) Stop() error {
	f.mu.Lock()
	defer f.mu.Unlock()

	if !f.running {
		return nil
	}

	logger.Info("Stopping Feishu channel")
	close(f.stopChan)
	// 停止后台落盘并做最后一次刷盘，避免重启后丢失去重记录导致重复回复
	if f.processedMessageIDs != nil {
		f.processedMessageIDs.stop()
	}

	if f.wsCancel != nil {
		f.wsCancel()
		f.wsCancel = nil
	}
	if f.wsClient != nil {
		f.wsClient.Close()
		f.wsClient = nil
	}

	f.running = false
	logger.Info("Feishu channel stopped")
	return nil
}

// botInfoResponse 兼容 Feishu /open-apis/bot/v3/info 的两种返回形态：
//   - 官方：{"code":0,"msg":"ok","bot":{"open_id":"ou_..."}}   ← bot 在顶层
//   - 旧文档/部分网关：{"data":{"bot":{"open_id":...}}}
//
// 历史上这里只解析了 data.bot，导致 botOpenID 永远是空字符串。
type botInfoResponse struct {
	Code int    `json:"code"`
	Msg  string `json:"msg"`
	Bot  *struct {
		OpenID string `json:"open_id"`
	} `json:"bot"`
	Data *struct {
		Bot *struct {
			OpenID string `json:"open_id"`
		} `json:"bot"`
	} `json:"data"`
}

// parseBotInfoResponse 从响应体里取出 open_id / code / msg
func parseBotInfoResponse(body []byte) (openID string, code int, msg string, err error) {
	var result botInfoResponse
	if err := json.Unmarshal(body, &result); err != nil {
		return "", 0, "", err
	}

	openID = ""
	if result.Bot != nil {
		openID = result.Bot.OpenID
	}
	if openID == "" && result.Data != nil && result.Data.Bot != nil {
		openID = result.Data.Bot.OpenID
	}
	return openID, result.Code, result.Msg, nil
}

// tokenRefreshMargin 计算提前刷新 token 的窗口。
//
// 之前固定 10 分钟：若上游下发的有效期本来就只有 10~20 分钟，
// 相当于每拿到 token 就立刻判定"需要刷新"，形成刷新风暴。
// 现在按剩余有效期的 1/10 提前刷新，并夹在 [1min, 10min]。
func tokenRefreshMarginForTTL(ttl time.Duration) time.Duration {
	if ttl <= 0 {
		return 0
	}
	margin := ttl / 10
	if margin < time.Minute {
		margin = time.Minute
	}
	if margin > 10*time.Minute {
		margin = 10 * time.Minute
	}
	return margin
}

func (f *FeishuChannel) fetchBotOpenID() error {
	token, err := f.ensureToken()
	if err != nil {
		return err
	}

	req, err := http.NewRequest("GET", f.getDomain()+"/open-apis/bot/v3/info", nil)
	if err != nil {
		return err
	}

	req.Header.Set("Authorization", "Bearer "+token)

	resp, err := f.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(io.LimitReader(resp.Body, maxFeishuBodySize))

	openID, code, msg, err := parseBotInfoResponse(respBody)
	if err != nil {
		return err
	}
	if code != 0 {
		return fmt.Errorf("get bot info error: %s (code: %d)", msg, code)
	}

	f.mu.Lock()
	f.botOpenID = openID
	f.mu.Unlock()

	if openID == "" {
		// 空 open_id 会让群聊 @ 判断退化（见 isBotMentioned 的回退分支），必须显式告警
		logger.Warn("Bot open_id is empty — group @-mention detection will fall back to heuristics")
		return nil
	}
	logger.Info("Bot open_id fetched", logger.String("open_id", openID))
	return nil
}

func (f *FeishuChannel) Send(ctx context.Context, msg bus.OutboundMessage) error {
	if msg.Metadata == nil {
		msg.Metadata = make(map[string]interface{})
	}

	if hint, ok := msg.Metadata["_tool_hint"].(bool); ok && hint {
		if f.cfg.Streaming {
			return f.sendToolHintDelta(ctx, msg)
		}
		return f.sendToolHint(ctx, msg)
	}

	// 其余进度类消息（Processing... / Done 等）不面向用户，直接丢弃，
	// 避免把内部状态当聊天内容发出去污染对话。
	if msg.Type == bus.MessageTypeProgress {
		return nil
	}

	return f.sendMessageWithFormat(ctx, msg)
}

func (f *FeishuChannel) SendDelta(ctx context.Context, chatID string, delta string, metadata map[string]interface{}) error {
	if metadata == nil {
		metadata = make(map[string]interface{})
	}

	if streamEnd, ok := metadata["_stream_end"].(bool); ok && streamEnd {
		if strings.TrimSpace(delta) != "" {
			if err := f.sendStreamDelta(ctx, chatID, delta, metadata); err != nil {
				return err
			}
		}
		return f.finalizeStream(ctx, chatID, metadata)
	}

	return f.sendStreamDelta(ctx, chatID, delta, metadata)
}

func (f *FeishuChannel) SendMessage(ctx context.Context, receiveID, message string) error {
	return f.sendTextMessage(ctx, receiveID, message, "", false)
}

func (f *FeishuChannel) sendTextMessage(ctx context.Context, receiveID, message string, replyToMsgID string, replyInThread bool) error {
	var url string
	var msgBody map[string]interface{}

	if replyToMsgID != "" {
		url = f.getDomain() + fmt.Sprintf("/open-apis/im/v1/messages/%s/reply", replyToMsgID)
		msgBody = map[string]interface{}{
			"msg_type": "text",
			"content":  fmt.Sprintf(`{"text":"%s"}`, escapeJSON(message)),
			"uuid":     fmt.Sprintf("%d", time.Now().UnixNano()),
		}
		if replyInThread {
			msgBody["reply_in_thread"] = true
		}
	} else {
		actualID := extractReceiveIDFromChatID(receiveID)
		receiveIDType := "open_id"
		if strings.HasPrefix(actualID, "oc_") {
			receiveIDType = "chat_id"
		}
		url = f.getDomain() + "/open-apis/im/v1/messages?receive_id_type=" + receiveIDType
		msgBody = map[string]interface{}{
			"receive_id": actualID,
			"msg_type":   "text",
			"content":    fmt.Sprintf(`{"text":"%s"}`, escapeJSON(message)),
			"uuid":       fmt.Sprintf("%d", time.Now().UnixNano()),
		}
	}

	if err := f.doAPIRequest(ctx, "POST", url, msgBody, nil); err != nil {
		return err
	}

	logger.Debug("Sent Feishu text message", logger.String("receive_id", receiveID))
	return nil
}

func (f *FeishuChannel) sendToolHint(ctx context.Context, msg bus.OutboundMessage) error {
	hint := strings.TrimSpace(msg.Content)
	if hint == "" {
		return nil
	}

	// 从会话ID中提取真实的接收者ID（处理 "feishu:ou_xxx" 格式）
	actualChatID := extractReceiveIDFromChatID(msg.ChatID)

	streamKey := f.streamKey(actualChatID, msg.Metadata)
	f.mu.RLock()
	buf := f.streamBufs[streamKey]
	f.mu.RUnlock()

	if buf != nil && buf.messageID != "" {
		return f.SendDelta(ctx, actualChatID, "\n\n"+f.formatToolHintDelta(hint)+"\n\n", msg.Metadata)
	}

	card := map[string]interface{}{
		"config": map[string]interface{}{"wide_screen_mode": true},
		"elements": []map[string]interface{}{
			{"tag": "markdown", "content": f.formatToolHintDelta(hint)},
		},
	}
	cardContent, _ := json.Marshal(card)

	return f.sendInteractiveMessage(ctx, actualChatID, string(cardContent), msg.Metadata)
}

// sendToolHintDelta 在流式模式下将工具提示追加到流式消息中。
// 复用 streamBuf 基础设施，工具提示和最终回复在同一条消息中显示。
// 当 finalizeStream 到达时，整条消息会被更新为最终内容，工具提示自然被替换。
func (f *FeishuChannel) sendToolHintDelta(ctx context.Context, msg bus.OutboundMessage) error {
	hint := strings.TrimSpace(msg.Content)
	if hint == "" {
		return nil
	}

	actualChatID := extractReceiveIDFromChatID(msg.ChatID)
	streamKey := f.streamKey(actualChatID, msg.Metadata)
	formatted := f.formatToolHintDelta(hint) + "\n\n"

	// Phase 1: 快照阶段（持有锁）—— 读取/创建 buffer，追加 hint
	type snapshot struct {
		text       string
		messageID  string
		needNewMsg bool
	}

	var snap snapshot
	func() {
		f.mu.Lock()
		defer f.mu.Unlock()

		buf := f.streamBufs[streamKey]
		if buf == nil {
			buf = &streamBuf{createdAt: time.Now()}
			f.streamBufs[streamKey] = buf
		}
		buf.text += formatted
		snap.text = buf.text
		snap.messageID = buf.messageID
		snap.needNewMsg = buf.messageID == ""
	}()

	if strings.TrimSpace(snap.text) == "" {
		return nil
	}

	// Phase 2: 动作阶段（无锁）—— 创建消息或更新内容
	if snap.needNewMsg {
		receiveIDType := f.receiveIDType(actualChatID)
		replyMsgID := ""
		if msgID, ok := msg.Metadata["message_id"].(string); ok {
			replyMsgID = msgID
		}

		newMsgID, err := f.createStreamingMessage(ctx, receiveIDType, actualChatID, replyMsgID, f.shouldUseReplyInThread(msg.Metadata))
		if err != nil {
			return err
		}

		f.mu.Lock()
		buf := f.streamBufs[streamKey]
		if buf != nil {
			buf.messageID = newMsgID
			buf.lastEdit = float64(time.Now().UnixNano()) / 1e9
		}
		f.mu.Unlock()

		postContent := f.buildPostContent(snap.text)
		return f.updateMessage(ctx, newMsgID, postContent)
	}

	postContent := f.buildPostContent(snap.text)
	return f.updateMessage(ctx, snap.messageID, postContent)
}

func (f *FeishuChannel) sendMessageWithFormat(ctx context.Context, msg bus.OutboundMessage) error {
	// 从会话ID中提取真实的接收者ID（处理 "feishu:ou_xxx" 格式）
	actualChatID := extractReceiveIDFromChatID(msg.ChatID)
	receiveIDType := f.receiveIDType(actualChatID)

	replyMessageID := ""
	hasThreadID := false
	if msg.Metadata != nil {
		if msgID, ok := msg.Metadata["message_id"].(string); ok {
			replyMessageID = msgID
		}
		if _, ok := msg.Metadata["thread_id"].(string); ok {
			hasThreadID = true
		}
	}

	if !f.cfg.ReplyToMessage && !hasThreadID {
		replyMessageID = ""
	}

	firstSend := true

	doSend := func(mType string, content string) error {
		if replyMessageID != "" {
			if hasThreadID {
				ok, _ := f.replyMessage(ctx, replyMessageID, mType, content, f.shouldUseReplyInThread(msg.Metadata))
				if ok {
					return nil
				}
			} else if firstSend {
				firstSend = false
				ok, _ := f.replyMessage(ctx, replyMessageID, mType, content, f.shouldUseReplyInThread(msg.Metadata))
				if ok {
					return nil
				}
			}
		}
		return f.createMessage(ctx, receiveIDType, actualChatID, mType, content)
	}

	for _, filePath := range msg.Media {
		info, err := os.Stat(filePath)
		if err != nil || info.IsDir() {
			logger.Warn("Media file not found", logger.String("path", filePath))
			continue
		}

		ext := strings.ToLower(filepath.Ext(filePath))
		if imageExts[ext] {
			key, err := f.uploadImage(ctx, filePath)
			if err == nil && key != "" {
				content, _ := json.Marshal(map[string]string{"image_key": key})
				_ = doSend("image", string(content))
			}
		} else {
			key, err := f.uploadFile(ctx, filePath)
			if err == nil && key != "" {
				mediaType := "file"
				if audioExts[ext] {
					mediaType = "audio"
				} else if videoExts[ext] {
					mediaType = "media"
				}
				content, _ := json.Marshal(map[string]string{"file_key": key})
				_ = doSend(mediaType, string(content))
			}
		}
	}

	content := strings.TrimSpace(msg.Content)
	if content == "" {
		return nil
	}

	// 超长回复按段落切分后分多条发送：飞书消息体有大小限制，
	// 整段超限会直接发送失败，用户就收不到这条回复了。
	parts := splitMessageContent(content, f.maxMessageChars())

	for _, part := range parts {
		if err := f.sendOneMessagePart(part, doSend); err != nil {
			return err
		}
	}
	return nil
}

// sendOneMessagePart 发送单段内容（已保证长度不超限）
func (f *FeishuChannel) sendOneMessagePart(
	content string,
	doSend func(mType string, content string) error,
) error {
	fmt := f.detectMsgFormat(content)
	switch fmt {
	case "text":
		textBody, _ := json.Marshal(map[string]string{"text": content})
		return doSend("text", string(textBody))
	case "post":
		postBody := f.markdownToPost(content)
		return doSend("post", postBody)
	default:
		elements := f.buildCardElements(content)
		chunks := f.splitElementsByTableLimit(elements)
		for _, chunk := range chunks {
			card := map[string]interface{}{
				"config":   map[string]interface{}{"wide_screen_mode": true},
				"elements": chunk,
			}
			cardContent, _ := json.Marshal(card)
			if err := doSend("interactive", string(cardContent)); err != nil {
				return err
			}
		}
	}
	return nil
}

// maxMessageChars 返回单条消息的字符上限（配置优先）
func (f *FeishuChannel) maxMessageChars() int {
	if f.cfg != nil && f.cfg.MaxMessageChars > 0 {
		return f.cfg.MaxMessageChars
	}
	return defaultMaxMessageChars
}

func (f *FeishuChannel) sendStreamDelta(ctx context.Context, chatID string, delta string, metadata map[string]interface{}) error {
	// 从会话ID中提取真实的接收者ID（处理 "feishu:ou_xxx" 格式）
	actualChatID := extractReceiveIDFromChatID(chatID)
	streamKey := f.streamKey(actualChatID, metadata)

	// Phase 1: 快照阶段（持有锁）—— 读取/创建 buffer，追加 delta
	type snapshot struct {
		text       string
		messageID  string
		needNewMsg bool
		needUpdate bool
	}

	var snap snapshot
	func() {
		f.mu.Lock()
		defer f.mu.Unlock()

		buf := f.streamBufs[streamKey]
		if buf == nil {
			buf = &streamBuf{createdAt: time.Now()}
			f.streamBufs[streamKey] = buf
		}
		buf.text += delta
		text := buf.text

		if strings.TrimSpace(text) == "" {
			return
		}

		snap.text = text
		snap.messageID = buf.messageID

		now := float64(time.Now().UnixNano()) / 1e9
		if buf.messageID == "" {
			snap.needNewMsg = true
		} else if now-buf.lastEdit >= 0.5 {
			buf.lastEdit = now
			snap.needUpdate = true
		}
	}()

	// Phase 2: 动作阶段（无锁）—— 创建消息或更新内容
	if snap.text == "" {
		return nil
	}

	if snap.needNewMsg {
		receiveIDType := f.receiveIDType(actualChatID)
		replyMsgID := ""
		if metadata != nil {
			if msgID, ok := metadata["message_id"].(string); ok {
				replyMsgID = msgID
			}
		}

		newMsgID, err := f.createStreamingMessage(ctx, receiveIDType, actualChatID, replyMsgID, f.shouldUseReplyInThread(metadata))
		if err != nil {
			return err
		}

		f.mu.Lock()
		buf := f.streamBufs[streamKey]
		if buf != nil {
			buf.messageID = newMsgID
			buf.lastEdit = float64(time.Now().UnixNano()) / 1e9
		}
		f.mu.Unlock()

		// 更新消息内容
		postContent := f.buildPostContent(snap.text)
		_ = f.updateMessage(ctx, newMsgID, postContent)
	} else if snap.needUpdate {
		postContent := f.buildPostContent(snap.text)
		_ = f.updateMessage(ctx, snap.messageID, postContent)
	}

	return nil
}

func (f *FeishuChannel) finalizeStream(ctx context.Context, chatID string, metadata map[string]interface{}) error {
	// 从会话ID中提取真实的接收者ID（处理 "feishu:ou_xxx" 格式）
	actualChatID := extractReceiveIDFromChatID(chatID)
	streamKey := f.streamKey(actualChatID, metadata)

	f.mu.Lock()
	buf := f.streamBufs[streamKey]
	delete(f.streamBufs, streamKey)
	f.mu.Unlock()

	if metadata != nil {
		if msgID, ok := metadata["message_id"].(string); ok {
			if resuming, _ := metadata["_resuming"].(bool); !resuming {
				f.mu.Lock()
				reactionID := f.reactionIDs[msgID]
				delete(f.reactionIDs, msgID)
				f.mu.Unlock()

				if reactionID != "" {
					_ = f.removeReaction(msgID, reactionID)
				}
				if f.cfg.DoneEmoji != "" {
					_, _ = f.addReaction(msgID, f.cfg.DoneEmoji)
				}
			}
		}
	}

	if buf == nil || buf.text == "" {
		return nil
	}

	// 如果有流式消息，更新最终内容
	if buf.messageID != "" {
		postContent := f.buildPostContent(buf.text)
		if err := f.updateMessage(ctx, buf.messageID, postContent); err == nil {
			return nil
		} else {
			logger.Warn("Streaming message final update failed, falling back to regular message", logger.String("message_id", buf.messageID), logger.ErrorField(err))
		}
	}

	receiveIDType := f.receiveIDType(actualChatID)

	elements := f.buildCardElements(buf.text)
	chunks := f.splitElementsByTableLimit(elements)
	var sendErrors []string
	for _, chunk := range chunks {
		card := map[string]interface{}{
			"config":   map[string]interface{}{"wide_screen_mode": true},
			"elements": chunk,
		}
		cardContent, _ := json.Marshal(card)

		replyMsgID := f.threadReplyTarget(metadata)
		if replyMsgID != "" {
			if _, err := f.replyMessage(ctx, replyMsgID, "interactive", string(cardContent), f.shouldUseReplyInThread(metadata)); err != nil {
				sendErrors = append(sendErrors, fmt.Sprintf("reply failed: %v", err))
			}
		} else {
			if err := f.createMessage(ctx, receiveIDType, actualChatID, "interactive", string(cardContent)); err != nil {
				sendErrors = append(sendErrors, fmt.Sprintf("create failed: %v", err))
			}
		}
	}

	if len(sendErrors) > 0 {
		return fmt.Errorf("feishu send errors: %s", strings.Join(sendErrors, "; "))
	}
	return nil
}

// tokenNeedsRefresh 依据「总有效期」判断是否该提前刷新。
// 旧实现用剩余有效期算 margin，而 margin 恒小于剩余有效期，
// 判定 `now+margin > expiry` 只有在剩不到 1 分钟时才成立——
// 配合 30 分钟的 ticker，token 过期后可能半小时都不刷新。
func (f *FeishuChannel) tokenNeedsRefresh() bool {
	f.mu.RLock()
	token := f.tenantToken
	expiry := f.tokenExpiry
	ttl := f.tokenTTL
	f.mu.RUnlock()

	if token == "" || expiry.IsZero() {
		return true
	}
	margin := tokenRefreshMarginForTTL(ttl)
	if margin <= 0 {
		margin = time.Minute
	}
	return time.Now().Add(margin).After(expiry)
}

// mediaRetention 入站媒体文件的保留时长
const mediaRetention = 7 * 24 * time.Hour

// cleanupOldMedia 删除目录中过期的媒体文件（按修改时间）。
// 只清理本目录内的普通文件，失败只告警，不影响消息处理。
func cleanupOldMedia(dir string, retention time.Duration) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return
	}
	cutoff := time.Now().Add(-retention)
	removed := 0
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		info, err := e.Info()
		if err != nil || info.ModTime().After(cutoff) {
			continue
		}
		if err := os.Remove(filepath.Join(dir, e.Name())); err == nil {
			removed++
		}
	}
	if removed > 0 {
		logger.Debug("Cleaned up expired media files",
			logger.String("dir", dir), logger.Int("removed", removed))
	}
}

// readLimitedBody 读取响应体并检测是否被截断。
// 直接 io.ReadAll(io.LimitReader(...)) 会在超过上限时静默返回半截内容：
// 图片/文件被截断后照样落盘，后续解析失败却看不出原因。
func readLimitedBody(r io.Reader, limit int64) ([]byte, error) {
	data, err := io.ReadAll(io.LimitReader(r, limit+1))
	if err != nil {
		return nil, err
	}
	if int64(len(data)) > limit {
		return nil, fmt.Errorf("response body exceeds %d bytes, refusing to truncate", limit)
	}
	return data, nil
}

// sendFailureNotice 发送"处理失败"提示。
// 必须用独立 ctx：走到这里时原 ctx 往往已经超时/取消，
// 复用它会让提示本身也发不出去（用户既收不到回复也收不到提示），
// 且旧代码用 `_ =` 丢弃了发送错误，失败完全不可见。
func (f *FeishuChannel) sendFailureNotice(messageID, userID, chatType string) {
	replyTo := ""
	if f.cfg.ReplyToMessage {
		replyTo = messageID
	}

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	if err := f.sendTextMessage(ctx, userID, "抱歉，处理失败，请稍后重试。", replyTo, chatType == "group"); err != nil {
		logger.Warn("Failed to send failure notice to user",
			logger.String("user", userID),
			logger.ErrorField(err))
	}
}

// ensureToken 获取有效的 tenant token，必要时刷新
func (f *FeishuChannel) ensureToken() (string, error) {
	if f.tokenNeedsRefresh() {
		if err := f.refreshToken(); err != nil {
			// 刷新失败时，如果手上还有未过期的 token，继续用它
			f.mu.RLock()
			token := f.tenantToken
			expiry := f.tokenExpiry
			f.mu.RUnlock()
			if token != "" && time.Now().Before(expiry) {
				logger.Warn("Feishu token refresh failed, using existing token",
					logger.ErrorField(err))
				return token, nil
			}
			return "", fmt.Errorf("no feishu token available: %w", err)
		}
	}

	f.mu.RLock()
	token := f.tenantToken
	f.mu.RUnlock()
	return token, nil
}

func (f *FeishuChannel) addReaction(messageID, emojiType string) (string, error) {
	token, err := f.ensureToken()
	if err != nil {
		return "", err
	}

	reqBody := map[string]interface{}{
		"reaction_type": map[string]string{
			"emoji_type": emojiType,
		},
	}

	body, _ := json.Marshal(reqBody)

	url := f.getDomain() + fmt.Sprintf("/open-apis/im/v1/messages/%s/reactions", messageID)
	req, err := http.NewRequest("POST", url, bytes.NewReader(body))
	if err != nil {
		return "", err
	}

	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")

	resp, err := f.httpClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(io.LimitReader(resp.Body, maxFeishuBodySize))

	var result struct {
		Code int    `json:"code"`
		Msg  string `json:"msg"`
		Data struct {
			ReactionID string `json:"reaction_id"`
		} `json:"data"`
	}

	if err := json.Unmarshal(respBody, &result); err != nil {
		return "", err
	}

	if result.Code != 0 {
		return "", fmt.Errorf("add reaction error: %s (code: %d)", result.Msg, result.Code)
	}

	logger.Debug("Added reaction", logger.String("message_id", messageID), logger.String("emoji", emojiType))
	return result.Data.ReactionID, nil
}

func (f *FeishuChannel) removeReaction(messageID, reactionID string) error {
	token, err := f.ensureToken()
	if err != nil {
		return err
	}

	url := f.getDomain() + fmt.Sprintf("/open-apis/im/v1/messages/%s/reactions/%s", messageID, reactionID)
	req, err := http.NewRequest("DELETE", url, nil)
	if err != nil {
		return err
	}

	req.Header.Set("Authorization", "Bearer "+token)

	resp, err := f.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(io.LimitReader(resp.Body, maxFeishuBodySize))

	var result struct {
		Code int    `json:"code"`
		Msg  string `json:"msg"`
	}

	if err := json.Unmarshal(respBody, &result); err != nil {
		return err
	}

	if result.Code != 0 {
		return fmt.Errorf("remove reaction error: %s (code: %d)", result.Msg, result.Code)
	}

	logger.Debug("Removed reaction", logger.String("message_id", messageID))
	return nil
}

func (f *FeishuChannel) startSDKWebSocket(ctx context.Context) {
	eventHandler := dispatcher.NewEventDispatcher("", "").
		OnP2MessageReceiveV1(func(ctx context.Context, event *larkim.P2MessageReceiveV1) error {
			// 异步处理消息，避免阻塞 WebSocket 事件循环
			go func() {
				// 事件内容来自外部，解析/处理中任何 panic 都不应带走整个进程
				defer func() {
					if r := recover(); r != nil {
						logger.Error("Feishu event handler panicked",
							logger.String("panic", fmt.Sprintf("%v", r)))
					}
				}()
				if event != nil && event.EventReq != nil && len(event.EventReq.Body) > 0 {
					f.handleMessageEvent(event.EventReq.Body)
					return
				}

				body, err := json.Marshal(event)
				if err != nil {
					logger.Error("Failed to marshal Feishu message event", logger.ErrorField(err))
					return
				}
				f.handleMessageEvent(body)
			}()
			return nil
		}).
		OnP2MessageReadV1(func(ctx context.Context, event *larkim.P2MessageReadV1) error {
			// 忽略消息已读回执事件，仅记录调试日志
			logger.Debug("Message read event received (ignored)")
			return nil
		})

	client := larkws.NewClient(
		f.cfg.AppID,
		f.cfg.AppSecret,
		larkws.WithEventHandler(eventHandler),
		larkws.WithDomain(f.getDomain()),
		larkws.WithOnReady(func() {
			logger.Info("Feishu WebSocket connected")
		}),
		larkws.WithOnError(func(err error) {
			logger.Error("Feishu WebSocket error", logger.ErrorField(err))
		}),
		larkws.WithOnReconnecting(func() {
			logger.Warn("Feishu WebSocket reconnecting")
		}),
		larkws.WithOnReconnected(func() {
			logger.Info("Feishu WebSocket reconnected")
		}),
		larkws.WithOnDisconnected(func() {
			logger.Warn("Feishu WebSocket disconnected")
		}),
	)

	f.mu.Lock()
	f.wsClient = client
	f.mu.Unlock()

	go func() {
		// SDK 的 Start 在正常关闭时也会返回错误，因此用 ctx 状态区分
		// 「主动停止」与「异常断开」（原先的 err != nil 判断恒为真）
		err := client.Start(ctx)
		if ctx.Err() != nil {
			return
		}
		logger.Error("Feishu WebSocket client stopped unexpectedly", logger.ErrorField(err))
	}()
}

type FeishuEvent struct {
	UUID   string `json:"uuid"`
	Token  string `json:"token"`
	TS     string `json:"ts"`
	Type   string `json:"type"`
	Header struct {
		EventID    string `json:"event_id"`
		EventType  string `json:"event_type"`
		CreateTime string `json:"create_time"`
		Token      string `json:"token"`
		AppID      string `json:"app_id"`
		TenantKey  string `json:"tenant_key"`
	} `json:"header"`
}

type FeishuMention struct {
	Key  string `json:"key"`
	Name string `json:"name"`
	ID   struct {
		UnionID string `json:"union_id"`
		UserID  string `json:"user_id"`
		OpenID  string `json:"open_id"`
	} `json:"id"`
	TenantKey string `json:"tenant_key"`
}

type FeishuMessageEvent struct {
	FeishuEvent
	Event struct {
		Sender struct {
			SenderID struct {
				UnionID    string `json:"union_id"`
				UserID     string `json:"user_id"`
				OpenID     string `json:"open_id"`
				SenderType string `json:"sender_type"`
			} `json:"sender_id"`
			SenderType string `json:"sender_type"`
		} `json:"sender"`
		Message struct {
			MessageID   string          `json:"message_id"`
			MessageType string          `json:"message_type"`
			Content     string          `json:"content"`
			ChatID      string          `json:"chat_id"`
			ChatType    string          `json:"chat_type"`
			ParentID    string          `json:"parent_id"`
			RootID      string          `json:"root_id"`
			ThreadID    string          `json:"thread_id"`
			Mentions    []FeishuMention `json:"mentions"`
		} `json:"message"`
	} `json:"event"`
}

func (f *FeishuChannel) isBotMentioned(messageContent string, mentions []FeishuMention) bool {
	if strings.Contains(messageContent, "@_all") {
		return true
	}

	f.mu.RLock()
	botOpenID := f.botOpenID
	f.mu.RUnlock()

	for _, mention := range mentions {
		if mention.ID.OpenID == botOpenID {
			return true
		}
		if botOpenID == "" && mention.ID.UserID == "" && strings.HasPrefix(mention.ID.OpenID, "ou_") {
			return true
		}
	}

	return false
}

func (f *FeishuChannel) isGroupMessageForBot(messageContent string, mentions []FeishuMention) bool {
	if f.cfg.GroupPolicy == "open" {
		return true
	}
	return f.isBotMentioned(messageContent, mentions)
}

func (f *FeishuChannel) resolveMentions(text string, mentions []FeishuMention) string {
	if len(mentions) == 0 {
		return text
	}

	result := text
	for _, mention := range mentions {
		key := mention.Key
		if key == "" {
			continue
		}
		name := mention.Name
		if name == "" {
			name = key
		}
		openID := mention.ID.OpenID
		userID := mention.ID.UserID
		var replacement string
		if openID != "" && userID != "" {
			replacement = fmt.Sprintf("@%s (%s, user id: %s)", name, openID, userID)
		} else if openID != "" {
			replacement = fmt.Sprintf("@%s (%s)", name, openID)
		} else {
			replacement = fmt.Sprintf("@%s", name)
		}
		result = strings.ReplaceAll(result, key, replacement)
	}
	return result
}

func (f *FeishuChannel) handleMessageEvent(body []byte) {
	var msgEvent FeishuMessageEvent
	if err := json.Unmarshal(body, &msgEvent); err != nil {
		logger.Error("Failed to parse message event", logger.ErrorField(err))
		return
	}

	messageID := msgEvent.Event.Message.MessageID
	senderType := msgEvent.Event.Sender.SenderType
	chatType := msgEvent.Event.Message.ChatType

	if senderType == "bot" {
		return
	}

	// 原子去重：同一条消息只处理一次（检查与登记在同一把锁内完成）
	if !f.processedMessageIDs.addIfNew(messageID) {
		logger.Debug("Skipping duplicate message", logger.String("message_id", messageID))
		return
	}

	userID := msgEvent.Event.Sender.SenderID.OpenID
	chatID := msgEvent.Event.Message.ChatID
	messageType := msgEvent.Event.Message.MessageType

	if chatType == "group" && !f.isGroupMessageForBot(msgEvent.Event.Message.Content, msgEvent.Event.Message.Mentions) {
		logger.Debug("Skipping group message (not mentioned)", logger.String("message_id", messageID))
		return
	}

	contentParts := []string{}
	mediaPaths := []string{}

	switch messageType {
	case "text":
		var content struct {
			Text string `json:"text"`
		}
		if err := json.Unmarshal([]byte(msgEvent.Event.Message.Content), &content); err == nil {
			text := f.resolveMentions(content.Text, msgEvent.Event.Message.Mentions)
			if text != "" {
				contentParts = append(contentParts, text)
			}
		}

	case "post":
		text, imageKeys, _ := f.extractPostContent(msgEvent.Event.Message.Content)
		if text != "" {
			contentParts = append(contentParts, text)
		}
		for _, imgKey := range imageKeys {
			filePath, contentText, _ := f.downloadAndSaveMedia("image", map[string]interface{}{"image_key": imgKey}, messageID)
			if filePath != "" {
				mediaPaths = append(mediaPaths, filePath)
			}
			contentParts = append(contentParts, contentText)
		}

	case "image", "audio", "file", "media":
		var contentJSON map[string]interface{}
		_ = json.Unmarshal([]byte(msgEvent.Event.Message.Content), &contentJSON)
		filePath, contentText, _ := f.downloadAndSaveMedia(messageType, contentJSON, messageID)
		if filePath != "" {
			mediaPaths = append(mediaPaths, filePath)
		}
		contentParts = append(contentParts, contentText)

	case "share_chat", "share_user", "share_calendar_event", "interactive", "system", "merge_forward":
		var contentJSON map[string]interface{}
		_ = json.Unmarshal([]byte(msgEvent.Event.Message.Content), &contentJSON)
		text := f.extractShareCardContent(contentJSON, messageType)
		if text != "" {
			contentParts = append(contentParts, text)
		}

	default:
		contentParts = append(contentParts, fmt.Sprintf("[%s message]", messageType))
	}

	if msgEvent.Event.Message.ParentID != "" {
		parentText := f.getMessageContent(msgEvent.Event.Message.ParentID)
		if parentText != "" {
			if len(parentText) > 200 {
				parentText = parentText[:200] + "..."
			}
			contentParts = append([]string{fmt.Sprintf("[Reply to: %s]", parentText)}, contentParts...)
		}
	}

	content := strings.Join(contentParts, "\n")
	if content == "" && len(mediaPaths) == 0 {
		return
	}

	logger.Info("Received Feishu message",
		logger.String("user_id", userID),
		logger.String("message_id", messageID),
		logger.String("chat_type", chatType),
		logger.Int("content_length", len(content)))
	logger.Debug("Feishu message content",
		logger.String("user_id", userID),
		logger.String("content", content))

	go func() {
		if f.cfg.ReactEmoji != "" {
			reactionID, err := f.addReaction(messageID, f.cfg.ReactEmoji)
			if err == nil && reactionID != "" {
				f.mu.Lock()
				f.reactionIDs[messageID] = reactionID
				// 清理过期条目，保留最近 400 个
				const maxReactionIDs = 500
				const targetReactionIDs = 400
				if len(f.reactionIDs) > maxReactionIDs {
					removed := 0
					toRemove := len(f.reactionIDs) - targetReactionIDs
					for k := range f.reactionIDs {
						delete(f.reactionIDs, k)
						removed++
						if removed >= toRemove {
							break
						}
					}
				}
				f.mu.Unlock()
			}
		}
	}()

	var sessID string
	if chatType == "group" {
		if f.cfg.TopicIsolation {
			rootID := msgEvent.Event.Message.RootID
			if rootID == "" {
				rootID = messageID
			}
			sessID = fmt.Sprintf("feishu:%s:%s", chatID, rootID)
		} else {
			sessID = fmt.Sprintf("feishu:%s", chatID)
		}
	} else {
		sessID = fmt.Sprintf("feishu:%s", userID)
	}

	sess, err := f.sessionMgr.GetSession(sessID)
	if err != nil {
		sess = session.NewSession("feishu", userID)
		sess.ID = sessID
		sess.Title = "Feishu Chat"
		if saveErr := f.sessionMgr.SaveSession(sess); saveErr != nil {
			logger.Warn("Failed to save new Feishu session", logger.ErrorField(saveErr))
		}
	}

	// 整轮处理超时：默认 10 分钟，可由 agent.message_timeout 覆盖
	chatTimeout := 10 * time.Minute
	if f.appConfig != nil && f.appConfig.Agent.MessageTimeout > 0 {
		chatTimeout = f.appConfig.Agent.MessageTimeout
	}
	ctx, cancel := context.WithTimeout(context.Background(), chatTimeout)
	defer cancel()

	result, err := f.agent.Chat(ctx, sess, content)
	if err != nil {
		logger.Error("Agent chat failed", logger.ErrorField(err))
		f.sendFailureNotice(messageID, userID, chatType)
		return
	}

	// 检查 Agent 返回的错误（processMessage 失败时 err 为 nil 但 result.Error 不为 nil）
	if result != nil && result.Error != nil {
		logger.Error("Agent processing failed", logger.ErrorField(result.Error))
		f.sendFailureNotice(messageID, userID, chatType)
		return
	}

	go func() {
		f.mu.RLock()
		reactionID, ok := f.reactionIDs[messageID]
		f.mu.RUnlock()
		if ok && reactionID != "" {
			_ = f.removeReaction(messageID, reactionID)
			f.mu.Lock()
			delete(f.reactionIDs, messageID)
			f.mu.Unlock()
		}
		if f.cfg.DoneEmoji != "" {
			_, _ = f.addReaction(messageID, f.cfg.DoneEmoji)
		}
	}()

	replyChatID := chatID

	metadata := map[string]interface{}{
		"message_id": messageID,
		"chat_type":  chatType,
		"thread_id":  msgEvent.Event.Message.ThreadID,
		"parent_id":  msgEvent.Event.Message.ParentID,
		"root_id":    msgEvent.Event.Message.RootID,
	}

	replyContent := ""
	if result != nil && result.Message != nil {
		replyContent = result.Message.Content
	}

	// 如果回复内容为空，发送默认消息
	if strings.TrimSpace(replyContent) == "" {
		replyContent = "抱歉，我无法处理您的消息，请稍后重试。"
		logger.Warn("Empty reply content, sending default message",
			logger.String("message_id", messageID),
			logger.String("chat_id", replyChatID))
	}

	if f.cfg.Streaming {
		if err := f.SendDelta(ctx, replyChatID, replyContent, map[string]interface{}{
			"_stream_end": true,
			"message_id":  messageID,
			"chat_type":   chatType,
			"thread_id":   msgEvent.Event.Message.ThreadID,
		}); err != nil {
			logger.Error("Failed to send Feishu streaming reply, falling back to regular reply", logger.String("message_id", messageID), logger.String("chat_id", replyChatID), logger.ErrorField(err))
			outMsg := bus.OutboundMessage{
				ChatID:   replyChatID,
				Content:  replyContent,
				Media:    []string{},
				Metadata: metadata,
			}
			if fallbackErr := f.Send(ctx, outMsg); fallbackErr != nil {
				logger.Error("Failed to send Feishu fallback reply", logger.String("message_id", messageID), logger.String("chat_id", replyChatID), logger.ErrorField(fallbackErr))
			}
		}
	} else {
		outMsg := bus.OutboundMessage{
			ChatID:   replyChatID,
			Content:  replyContent,
			Media:    []string{},
			Metadata: metadata,
		}
		if err := f.Send(ctx, outMsg); err != nil {
			logger.Error("Failed to send Feishu reply", logger.String("message_id", messageID), logger.String("chat_id", replyChatID), logger.ErrorField(err))
		}
	}
}

func (f *FeishuChannel) getMessageContent(messageID string) string {
	token, err := f.ensureToken()
	if err != nil {
		return ""
	}

	url := f.getDomain() + fmt.Sprintf("/open-apis/im/v1/messages/%s", messageID)
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return ""
	}

	req.Header.Set("Authorization", "Bearer "+token)

	resp, err := f.httpClient.Do(req)
	if err != nil {
		return ""
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(io.LimitReader(resp.Body, maxFeishuBodySize))

	var result struct {
		Code int    `json:"code"`
		Msg  string `json:"msg"`
		Data struct {
			Items []struct {
				Body struct {
					Content string `json:"content"`
				} `json:"body"`
				MsgType string `json:"msg_type"`
			} `json:"items"`
		} `json:"data"`
	}

	if err := json.Unmarshal(respBody, &result); err != nil {
		return ""
	}

	if result.Code != 0 || len(result.Data.Items) == 0 {
		return ""
	}

	item := result.Data.Items[0]
	switch item.MsgType {
	case "text":
		var content struct {
			Text string `json:"text"`
		}
		if err := json.Unmarshal([]byte(item.Body.Content), &content); err == nil {
			return content.Text
		}
	case "post":
		text, _, _ := f.extractPostContent(item.Body.Content)
		return text
	}

	return ""
}

func (f *FeishuChannel) extractPostContent(content string) (string, []string, error) {
	var contentJSON map[string]interface{}
	if err := json.Unmarshal([]byte(content), &contentJSON); err != nil {
		return "", nil, err
	}

	if post, ok := contentJSON["post"].(map[string]interface{}); ok {
		contentJSON = post
	}

	var root map[string]interface{}
	for _, key := range []string{"zh_cn", "en_us", "ja_jp"} {
		if loc, ok := contentJSON[key].(map[string]interface{}); ok {
			root = loc
			break
		}
	}
	if root == nil {
		root = contentJSON
	}

	texts := []string{}
	imageKeys := []string{}

	if contentArr, ok := root["content"].([]interface{}); ok {
		for _, row := range contentArr {
			if elements, ok := row.([]interface{}); ok {
				for _, elem := range elements {
					if elemMap, ok := elem.(map[string]interface{}); ok {
						tag, _ := elemMap["tag"].(string)
						switch tag {
						case "text":
							if t, ok := elemMap["text"].(string); ok {
								texts = append(texts, t)
							}
						case "a":
							if t, ok := elemMap["text"].(string); ok {
								texts = append(texts, t)
							}
							if href, ok := elemMap["href"].(string); ok {
								texts = append(texts, "("+href+")")
							}
						case "at":
							if name, ok := elemMap["user_name"].(string); ok {
								texts = append(texts, "@"+name)
							}
						case "img":
							if key, ok := elemMap["image_key"].(string); ok {
								imageKeys = append(imageKeys, key)
							}
						}
					}
				}
			}
		}
	}

	if title, ok := root["title"].(string); ok && title != "" {
		texts = append([]string{title}, texts...)
	}

	return strings.Join(texts, ""), imageKeys, nil
}

func (f *FeishuChannel) extractShareCardContent(content map[string]interface{}, msgType string) string {
	switch msgType {
	case "share_chat":
		if chatID, ok := content["chat_id"].(string); ok {
			return fmt.Sprintf("[shared chat: %s]", chatID)
		}
	case "share_user":
		if userID, ok := content["user_id"].(string); ok {
			return fmt.Sprintf("[shared user: %s]", userID)
		}
	case "interactive":
		return f.extractInteractiveContent(content)
	case "share_calendar_event":
		if eventKey, ok := content["event_key"].(string); ok {
			return fmt.Sprintf("[shared calendar event: %s]", eventKey)
		}
	case "system":
		return "[system message]"
	case "merge_forward":
		return "[merged forward messages]"
	}
	return fmt.Sprintf("[%s]", msgType)
}

func (f *FeishuChannel) extractInteractiveContent(content map[string]interface{}) string {
	parts := []string{}

	if header, ok := content["header"].(map[string]interface{}); ok {
		if headerTitle, ok := header["title"].(map[string]interface{}); ok {
			if text, ok := headerTitle["content"].(string); ok && text != "" {
				parts = append(parts, "title: "+text)
			} else if text, ok := headerTitle["text"].(string); ok && text != "" {
				parts = append(parts, "title: "+text)
			}
		}
	}

	if card, ok := content["card"].(map[string]interface{}); ok {
		parts = append(parts, f.extractInteractiveContent(card))
	}

	if elements, ok := content["elements"].([]interface{}); ok {
		for _, elem := range elements {
			if elemMap, ok := elem.(map[string]interface{}); ok {
				parts = append(parts, f.extractElementContent(elemMap))
			}
		}
	}

	return strings.Join(parts, "\n")
}

func (f *FeishuChannel) extractElementContent(elem map[string]interface{}) string {
	tag, _ := elem["tag"].(string)

	switch tag {
	case "markdown", "lark_md":
		if content, ok := elem["content"].(string); ok {
			return content
		}
	case "div":
		if textObj, ok := elem["text"].(map[string]interface{}); ok {
			if text, ok := textObj["content"].(string); ok {
				return text
			} else if text, ok := textObj["text"].(string); ok {
				return text
			}
		}
	case "a":
		if href, ok := elem["href"].(string); ok {
			if text, ok := elem["text"].(string); ok {
				return fmt.Sprintf("%s (link: %s)", text, href)
			}
			return fmt.Sprintf("link: %s", href)
		}
	case "button":
		if textObj, ok := elem["text"].(map[string]interface{}); ok {
			if text, ok := textObj["content"].(string); ok {
				return text
			}
		}
		if url, ok := elem["url"].(string); ok {
			return fmt.Sprintf("link: %s", url)
		}
	case "img":
		if alt, ok := elem["alt"].(map[string]interface{}); ok {
			if text, ok := alt["content"].(string); ok {
				return fmt.Sprintf("[image: %s]", text)
			}
		}
		return "[image]"
	case "note":
		subParts := []string{}
		if elements, ok := elem["elements"].([]interface{}); ok {
			for _, subElem := range elements {
				if subElemMap, ok := subElem.(map[string]interface{}); ok {
					subParts = append(subParts, f.extractElementContent(subElemMap))
				}
			}
		}
		return strings.Join(subParts, " ")
	case "column_set":
		subParts := []string{}
		if columns, ok := elem["columns"].([]interface{}); ok {
			for _, col := range columns {
				if colMap, ok := col.(map[string]interface{}); ok {
					if elements, ok := colMap["elements"].([]interface{}); ok {
						for _, subElem := range elements {
							if subElemMap, ok := subElem.(map[string]interface{}); ok {
								subParts = append(subParts, f.extractElementContent(subElemMap))
							}
						}
					}
				}
			}
		}
		return strings.Join(subParts, " ")
	case "plain_text":
		if content, ok := elem["content"].(string); ok {
			return content
		}
	}

	if elements, ok := elem["elements"].([]interface{}); ok {
		subParts := []string{}
		for _, subElem := range elements {
			if subElemMap, ok := subElem.(map[string]interface{}); ok {
				subParts = append(subParts, f.extractElementContent(subElemMap))
			}
		}
		return strings.Join(subParts, "\n")
	}

	return ""
}

func (f *FeishuChannel) downloadAndSaveMedia(msgType string, contentJSON map[string]interface{}, messageID string) (string, string, error) {
	var fileKey string
	switch msgType {
	case "image":
		if key, ok := contentJSON["image_key"].(string); ok {
			fileKey = key
		}
	default:
		if key, ok := contentJSON["file_key"].(string); ok {
			fileKey = key
		}
	}

	if fileKey == "" {
		return "", fmt.Sprintf("[%s: missing file key]", msgType), fmt.Errorf("missing file key")
	}

	mediaDir := f.appConfig.ResolvePath("media/feishu")
	// 入站媒体此前只写不删：图片/语音/文件永久堆积。
	// 每次落盘顺带清理过期文件（默认 7 天，按 mtime）。
	cleanupOldMedia(mediaDir, mediaRetention)
	if err := os.MkdirAll(mediaDir, 0755); err != nil {
		return "", fmt.Sprintf("[%s: download failed]", msgType), err
	}

	var fileData []byte
	var fileName string
	var err error

	if msgType == "image" {
		fileData, fileName, err = f.downloadImage(messageID, fileKey)
	} else {
		fileData, fileName, err = f.downloadFile(messageID, fileKey, msgType)
	}

	if err != nil || fileData == nil {
		return "", fmt.Sprintf("[%s: download failed]", msgType), err
	}

	if fileName == "" {
		fileName = fileKey
	}

	if msgType == "audio" && !strings.HasSuffix(fileName, ".opus") && !strings.HasSuffix(fileName, ".ogg") {
		fileName += ".ogg"
	}

	safeName := f.safeMediaFilename(fileName, fileKey)
	filePath := filepath.Join(mediaDir, safeName)
	if err := os.WriteFile(filePath, fileData, 0600); err != nil {
		return "", fmt.Sprintf("[%s: save failed]", msgType), err
	}

	logger.Debug("Downloaded media", logger.String("type", msgType), logger.String("path", filePath))
	return filePath, fmt.Sprintf("[%s: %s]", msgType, filePath), nil
}

func (f *FeishuChannel) downloadImage(messageID, imageKey string) ([]byte, string, error) {
	token, err := f.ensureToken()
	if err != nil {
		return nil, "", err
	}

	url := f.getDomain() + fmt.Sprintf("/open-apis/im/v1/messages/%s/resources?file_key=%s&type=image", messageID, imageKey)
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, "", err
	}

	req.Header.Set("Authorization", "Bearer "+token)

	resp, err := f.httpClient.Do(req)
	if err != nil {
		return nil, "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, "", fmt.Errorf("download failed: %d", resp.StatusCode)
	}

	data, err := readLimitedBody(resp.Body, maxFeishuBodySize)
	if err != nil {
		return nil, "", fmt.Errorf("download image: %w", err)
	}

	fileName := resp.Header.Get("X-Filename")
	if fileName == "" {
		fileName = imageKey + ".jpg"
	}

	return data, fileName, nil
}

func (f *FeishuChannel) downloadFile(messageID, fileKey, resourceType string) ([]byte, string, error) {
	token, err := f.ensureToken()
	if err != nil {
		return nil, "", err
	}

	downloadType := resourceType
	if downloadType == "audio" || downloadType == "media" {
		downloadType = "file"
	}

	url := f.getDomain() + fmt.Sprintf("/open-apis/im/v1/messages/%s/resources?file_key=%s&type=%s", messageID, fileKey, downloadType)
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, "", err
	}

	req.Header.Set("Authorization", "Bearer "+token)

	resp, err := f.httpClient.Do(req)
	if err != nil {
		return nil, "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, "", fmt.Errorf("download failed: %d", resp.StatusCode)
	}

	data, err := readLimitedBody(resp.Body, maxFeishuBodySize)
	if err != nil {
		return nil, "", fmt.Errorf("download image: %w", err)
	}

	fileName := resp.Header.Get("X-Filename")
	if fileName == "" {
		fileName = fileKey
	}

	return data, fileName, nil
}

func (f *FeishuChannel) uploadImage(ctx context.Context, filePath string) (string, error) {
	token, err := f.ensureToken()
	if err != nil {
		return "", err
	}

	fileData, err := os.ReadFile(filePath)
	if err != nil {
		return "", err
	}

	body := &bytes.Buffer{}
	body.WriteString("--boundary\r\n")
	body.WriteString("Content-Disposition: form-data; name=\"image_type\"\r\n\r\n")
	body.WriteString("message\r\n")
	body.WriteString("--boundary\r\n")
	body.WriteString(fmt.Sprintf("Content-Disposition: form-data; name=\"image\"; filename=\"%s\"\r\n", filepath.Base(filePath)))
	body.WriteString("Content-Type: application/octet-stream\r\n\r\n")
	body.Write(fileData)
	body.WriteString("\r\n--boundary--\r\n")

	req, err := http.NewRequestWithContext(ctx, "POST", f.getDomain()+"/open-apis/im/v1/images", body)
	if err != nil {
		return "", err
	}

	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "multipart/form-data; boundary=boundary")

	resp, err := f.httpClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(io.LimitReader(resp.Body, maxFeishuBodySize))

	var result struct {
		Code int    `json:"code"`
		Msg  string `json:"msg"`
		Data struct {
			ImageKey string `json:"image_key"`
		} `json:"data"`
	}

	if err := json.Unmarshal(respBody, &result); err != nil {
		return "", err
	}

	if result.Code != 0 {
		return "", fmt.Errorf("upload image error: %s (code: %d)", result.Msg, result.Code)
	}

	logger.Debug("Uploaded image", logger.String("path", filePath), logger.String("image_key", result.Data.ImageKey))
	return result.Data.ImageKey, nil
}

func (f *FeishuChannel) uploadFile(ctx context.Context, filePath string) (string, error) {
	token, err := f.ensureToken()
	if err != nil {
		return "", err
	}

	ext := strings.ToLower(filepath.Ext(filePath))
	fileType := fileTypeMap[ext]
	if fileType == "" {
		fileType = "stream"
	}

	fileData, err := os.ReadFile(filePath)
	if err != nil {
		return "", err
	}

	body := &bytes.Buffer{}
	body.WriteString("--boundary\r\n")
	body.WriteString("Content-Disposition: form-data; name=\"file_type\"\r\n\r\n")
	body.WriteString(fileType + "\r\n")
	body.WriteString("--boundary\r\n")
	body.WriteString("Content-Disposition: form-data; name=\"file_name\"\r\n\r\n")
	body.WriteString(filepath.Base(filePath) + "\r\n")
	body.WriteString("--boundary\r\n")
	body.WriteString(fmt.Sprintf("Content-Disposition: form-data; name=\"file\"; filename=\"%s\"\r\n", filepath.Base(filePath)))
	body.WriteString("Content-Type: application/octet-stream\r\n\r\n")
	body.Write(fileData)
	body.WriteString("\r\n--boundary--\r\n")

	req, err := http.NewRequestWithContext(ctx, "POST", f.getDomain()+"/open-apis/im/v1/files", body)
	if err != nil {
		return "", err
	}

	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "multipart/form-data; boundary=boundary")

	resp, err := f.httpClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(io.LimitReader(resp.Body, maxFeishuBodySize))

	var result struct {
		Code int    `json:"code"`
		Msg  string `json:"msg"`
		Data struct {
			FileKey string `json:"file_key"`
		} `json:"data"`
	}

	if err := json.Unmarshal(respBody, &result); err != nil {
		return "", err
	}

	if result.Code != 0 {
		return "", fmt.Errorf("upload file error: %s (code: %d)", result.Msg, result.Code)
	}

	logger.Debug("Uploaded file", logger.String("path", filePath), logger.String("file_key", result.Data.FileKey))
	return result.Data.FileKey, nil
}

func (f *FeishuChannel) createMessage(ctx context.Context, receiveIDType, receiveID, msgType, content string) error {
	req := larkim.NewCreateMessageReqBuilder().
		ReceiveIdType(receiveIDType).
		Body(larkim.NewCreateMessageReqBodyBuilder().
			ReceiveId(receiveID).
			MsgType(msgType).
			Content(content).
			Build()).
		Build()

	resp, err := f.larkClient.Im.V1.Message.Create(ctx, req)
	if err != nil {
		return fmt.Errorf("create message: %w", err)
	}
	if !resp.Success() {
		return fmt.Errorf("create message failed: code=%d, msg=%s", resp.Code, resp.Msg)
	}

	logger.Debug("Created Feishu message", logger.String("receive_id_type", receiveIDType), logger.String("receive_id", receiveID), logger.String("msg_type", msgType))
	return nil
}

func (f *FeishuChannel) sendInteractiveMessage(ctx context.Context, receiveID, content string, metadata map[string]interface{}) error {
	// 从会话ID中提取真实的接收者ID（处理 "feishu:ou_xxx" 格式）
	actualReceiveID := extractReceiveIDFromChatID(receiveID)
	receiveIDType := f.receiveIDType(actualReceiveID)

	replyMsgID := f.threadReplyTarget(metadata)
	if replyMsgID != "" {
		_, err := f.replyMessage(ctx, replyMsgID, "interactive", content, f.shouldUseReplyInThread(metadata))
		return err
	}

	return f.createMessage(ctx, receiveIDType, actualReceiveID, "interactive", content)
}

func (f *FeishuChannel) replyMessage(ctx context.Context, messageID, msgType, content string, replyInThread bool) (bool, error) {
	// replyInThread 此前是未使用参数：话题隔离模式下回复落到主楼，
	// 话题外成员会看到串台消息。这里真正传给飞书。
	req := larkim.NewReplyMessageReqBuilder().
		MessageId(messageID).
		Body(larkim.NewReplyMessageReqBodyBuilder().
			MsgType(msgType).
			Content(content).
			ReplyInThread(replyInThread).
			Build()).
		Build()

	resp, err := f.larkClient.Im.V1.Message.Reply(ctx, req)
	if err != nil {
		return false, fmt.Errorf("reply message: %w", err)
	}
	if !resp.Success() {
		return false, fmt.Errorf("reply message failed: code=%d, msg=%s", resp.Code, resp.Msg)
	}

	logger.Debug("Reply sent", logger.String("message_id", messageID), logger.String("msg_type", msgType))
	return true, nil
}

// createStreamingMessage 创建流式消息（发送初始消息并返回消息ID）
func (f *FeishuChannel) createStreamingMessage(ctx context.Context, receiveIDType, chatID, replyMessageID string, replyInThread bool) (string, error) {
	// 发送初始空消息
	initialContent := "..."
	postBody := f.buildPostContent(initialContent)

	if replyMessageID != "" {
		req := larkim.NewReplyMessageReqBuilder().
			MessageId(replyMessageID).
			Body(larkim.NewReplyMessageReqBodyBuilder().
				MsgType("post").
				Content(postBody).
				Build()).
			Build()

		resp, err := f.larkClient.Im.V1.Message.Reply(ctx, req)
		if err != nil {
			return "", fmt.Errorf("create streaming message: %w", err)
		}
		if !resp.Success() {
			return "", fmt.Errorf("create streaming message failed: code=%d, msg=%s", resp.Code, resp.Msg)
		}
		return *resp.Data.MessageId, nil
	}

	req := larkim.NewCreateMessageReqBuilder().
		ReceiveIdType(receiveIDType).
		Body(larkim.NewCreateMessageReqBodyBuilder().
			ReceiveId(chatID).
			MsgType("post").
			Content(postBody).
			Build()).
		Build()

	resp, err := f.larkClient.Im.V1.Message.Create(ctx, req)
	if err != nil {
		return "", fmt.Errorf("create streaming message: %w", err)
	}
	if !resp.Success() {
		return "", fmt.Errorf("create streaming message failed: code=%d, msg=%s", resp.Code, resp.Msg)
	}
	return *resp.Data.MessageId, nil
}

// updateMessage 更新消息内容（使用 Message.Update API）
func (f *FeishuChannel) updateMessage(ctx context.Context, messageID, content string) error {
	req := larkim.NewUpdateMessageReqBuilder().
		MessageId(messageID).
		Body(larkim.NewUpdateMessageReqBodyBuilder().
			MsgType("post").
			Content(content).
			Build()).
		Build()

	resp, err := f.larkClient.Im.V1.Message.Update(ctx, req)
	if err != nil {
		return fmt.Errorf("update message: %w", err)
	}
	if !resp.Success() {
		return fmt.Errorf("update message failed: code=%d, msg=%s", resp.Code, resp.Msg)
	}
	return nil
}

func (f *FeishuChannel) detectMsgFormat(content string) string {
	stripped := strings.TrimSpace(content)

	if tableRegex.MatchString(stripped) || headingRegex.MatchString(stripped) || strings.Contains(stripped, "```") {
		return "interactive"
	}

	if len(stripped) > 2000 {
		return "interactive"
	}

	if mdItalicRegex.MatchString(stripped) || mdStrikeRegex.MatchString(stripped) || mdBoldRegex.MatchString(stripped) || mdBoldUnderscoreRegex.MatchString(stripped) {
		return "interactive"
	}

	if listRegex.MatchString(stripped) || orderedListRegex.MatchString(stripped) {
		return "interactive"
	}

	if mdLinkRegex.MatchString(stripped) {
		return "post"
	}

	if len(stripped) <= 200 {
		return "text"
	}

	return "post"
}

func (f *FeishuChannel) buildCardElements(content string) []map[string]interface{} {
	elements := []map[string]interface{}{}
	matches := tableRegex.FindAllStringSubmatchIndex(content, -1)
	lastEnd := 0

	for _, match := range matches {
		before := content[lastEnd:match[0]]
		if strings.TrimSpace(before) != "" {
			elements = append(elements, f.splitHeadings(before)...)
		}
		table := f.parseMarkdownTable(content[match[0]:match[1]])
		if table != nil {
			elements = append(elements, table)
		} else {
			elements = append(elements, map[string]interface{}{"tag": "markdown", "content": content[match[0]:match[1]]})
		}
		lastEnd = match[1]
	}

	if lastEnd < len(content) {
		remaining := content[lastEnd:]
		if strings.TrimSpace(remaining) != "" {
			elements = append(elements, f.splitHeadings(remaining)...)
		}
	}

	if len(elements) == 0 {
		return []map[string]interface{}{{"tag": "markdown", "content": content}}
	}

	return elements
}

func (f *FeishuChannel) splitElementsByTableLimit(elements []map[string]interface{}) [][]map[string]interface{} {
	if len(elements) == 0 {
		return [][]map[string]interface{}{{}}
	}

	chunks := [][]map[string]interface{}{}
	current := []map[string]interface{}{}
	tableCount := 0

	for _, elem := range elements {
		if tag, ok := elem["tag"].(string); ok && tag == "table" {
			if tableCount >= 1 {
				chunks = append(chunks, current)
				current = []map[string]interface{}{}
				tableCount = 0
			}
			current = append(current, elem)
			tableCount++
		} else {
			current = append(current, elem)
		}
	}

	if len(current) > 0 {
		chunks = append(chunks, current)
	}

	return chunks
}

func (f *FeishuChannel) splitHeadings(content string) []map[string]interface{} {
	protected := content
	codeBlocks := []string{}
	for _, match := range codeBlockRegex.FindAllStringSubmatch(content, -1) {
		if len(match) > 0 {
			codeBlocks = append(codeBlocks, match[0])
			protected = strings.Replace(protected, match[0], fmt.Sprintf("\x00CODE%d\x00", len(codeBlocks)-1), 1)
		}
	}

	elements := []map[string]interface{}{}
	matches := headingRegex.FindAllStringSubmatchIndex(protected, -1)
	lastEnd := 0

	for _, match := range matches {
		before := strings.TrimSpace(protected[lastEnd:match[0]])
		if before != "" {
			beforeContent := before
			for i, cb := range codeBlocks {
				beforeContent = strings.ReplaceAll(beforeContent, fmt.Sprintf("\x00CODE%d\x00", i), cb)
			}
			elements = append(elements, map[string]interface{}{"tag": "markdown", "content": beforeContent})
		}

		headingText := strings.TrimSpace(protected[match[4]:match[5]])
		headingText = f.stripMarkdownFormatting(headingText)
		displayText := fmt.Sprintf("**%s**", headingText)

		for i, cb := range codeBlocks {
			displayText = strings.ReplaceAll(displayText, fmt.Sprintf("\x00CODE%d\x00", i), cb)
		}

		elements = append(elements, map[string]interface{}{
			"tag": "div",
			"text": map[string]interface{}{
				"tag":     "lark_md",
				"content": displayText,
			},
		})
		lastEnd = match[1]
	}

	if lastEnd < len(protected) {
		remaining := strings.TrimSpace(protected[lastEnd:])
		if remaining != "" {
			for i, cb := range codeBlocks {
				remaining = strings.ReplaceAll(remaining, fmt.Sprintf("\x00CODE%d\x00", i), cb)
			}
			elements = append(elements, map[string]interface{}{"tag": "markdown", "content": remaining})
		}
	}

	if len(elements) == 0 {
		return []map[string]interface{}{{"tag": "markdown", "content": content}}
	}

	return elements
}

func (f *FeishuChannel) parseMarkdownTable(tableText string) map[string]interface{} {
	lines := strings.Split(strings.TrimSpace(tableText), "\n")
	filteredLines := []string{}
	for _, line := range lines {
		if strings.TrimSpace(line) != "" {
			filteredLines = append(filteredLines, line)
		}
	}
	if len(filteredLines) < 3 {
		return nil
	}

	splitLine := func(line string) []string {
		parts := strings.Split(strings.Trim(line, " |"), "|")
		result := []string{}
		for _, p := range parts {
			result = append(result, strings.TrimSpace(p))
		}
		return result
	}

	headers := splitLine(filteredLines[0])
	for i := range headers {
		headers[i] = f.stripMarkdownFormatting(headers[i])
	}

	rows := []map[string]string{}
	for _, line := range filteredLines[2:] {
		parts := splitLine(line)
		row := map[string]string{}
		for i := range headers {
			if i < len(parts) {
				row[fmt.Sprintf("c%d", i)] = f.stripMarkdownFormatting(parts[i])
			} else {
				row[fmt.Sprintf("c%d", i)] = ""
			}
		}
		rows = append(rows, row)
	}

	columns := []map[string]interface{}{}
	for i := range headers {
		columns = append(columns, map[string]interface{}{
			"tag":          "column",
			"name":         fmt.Sprintf("c%d", i),
			"display_name": headers[i],
			"width":        "auto",
		})
	}

	return map[string]interface{}{
		"tag":       "table",
		"page_size": len(rows) + 1,
		"columns":   columns,
		"rows":      rows,
	}
}

func (f *FeishuChannel) stripMarkdownFormatting(text string) string {
	text = mdBoldRegex.ReplaceAllString(text, "$1")
	text = mdBoldUnderscoreRegex.ReplaceAllString(text, "$1")
	text = mdItalicRegex.ReplaceAllString(text, "$1$2$3")
	text = mdStrikeRegex.ReplaceAllString(text, "$1")
	return text
}

// buildPostContent 构建 post 格式的消息内容
func (f *FeishuChannel) buildPostContent(content string) string {
	return f.markdownToPost(content)
}

func (f *FeishuChannel) markdownToPost(content string) string {
	lines := strings.Split(strings.TrimSpace(content), "\n")
	paragraphs := [][]map[string]interface{}{}

	for _, line := range lines {
		elements := []map[string]interface{}{}
		lastEnd := 0

		for _, match := range mdLinkRegex.FindAllStringSubmatchIndex(line, -1) {
			before := line[lastEnd:match[0]]
			if before != "" {
				elements = append(elements, map[string]interface{}{"tag": "text", "text": before})
			}
			elements = append(elements, map[string]interface{}{
				"tag":  "a",
				"text": line[match[2]:match[3]],
				"href": line[match[4]:match[5]],
			})
			lastEnd = match[1]
		}

		remaining := line[lastEnd:]
		if remaining != "" {
			elements = append(elements, map[string]interface{}{"tag": "text", "text": remaining})
		}

		if len(elements) == 0 {
			elements = append(elements, map[string]interface{}{"tag": "text", "text": ""})
		}

		paragraphs = append(paragraphs, elements)
	}

	postBody := map[string]interface{}{
		"zh_cn": map[string]interface{}{
			"content": paragraphs,
		},
	}
	data, _ := json.Marshal(postBody)
	return string(data)
}

func (f *FeishuChannel) receiveIDType(receiveID string) string {
	if strings.HasPrefix(receiveID, "ou_") || strings.HasPrefix(receiveID, "on_") || strings.HasPrefix(receiveID, "un_") {
		return "open_id"
	}
	return "chat_id"
}

func (f *FeishuChannel) streamKey(chatID string, metadata map[string]interface{}) string {
	if metadata != nil {
		if msgID, ok := metadata["message_id"].(string); ok {
			return msgID
		}
	}
	return chatID
}

func (f *FeishuChannel) shouldUseReplyInThread(metadata map[string]interface{}) bool {
	if metadata == nil {
		return false
	}
	chatType, _ := metadata["chat_type"].(string)
	return chatType == "group" && f.cfg.ReplyToMessage
}

func (f *FeishuChannel) threadReplyTarget(metadata map[string]interface{}) string {
	if metadata == nil {
		return ""
	}
	chatType, _ := metadata["chat_type"].(string)
	if chatType != "group" {
		return ""
	}
	messageID, _ := metadata["message_id"].(string)
	if messageID == "" {
		return ""
	}
	if _, ok := metadata["thread_id"]; ok || f.cfg.ReplyToMessage {
		return messageID
	}
	return ""
}

func (f *FeishuChannel) safeMediaFilename(filename, fallback string) string {
	filename = strings.ReplaceAll(filename, "\\", "/")
	filename = filepath.Base(filename)

	// 白名单方式：只保留安全字符（字母、数字、下划线、连字符、点），其余替换为下划线
	filename = strings.Map(func(r rune) rune {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '_' || r == '-' || r == '.' {
			return r
		}
		return '_'
	}, filename)

	// 去除首尾的点和下划线（Windows 会静默去除尾部的点和空格）
	filename = strings.Trim(filename, "._")

	if filename == "" || filename == "." || filename == ".." {
		return fallback
	}

	// 检查 Windows 保留设备名
	upper := strings.ToUpper(filename)
	baseName := strings.SplitN(upper, ".", 2)[0]
	reservedNames := map[string]bool{
		"CON": true, "PRN": true, "AUX": true, "NUL": true,
		"COM1": true, "COM2": true, "COM3": true, "COM4": true,
		"COM5": true, "COM6": true, "COM7": true, "COM8": true, "COM9": true,
		"LPT1": true, "LPT2": true, "LPT3": true, "LPT4": true,
		"LPT5": true, "LPT6": true, "LPT7": true, "LPT8": true, "LPT9": true,
	}
	if reservedNames[baseName] {
		return "_" + filename
	}

	return filename
}

func (f *FeishuChannel) formatToolHintDelta(hint string) string {
	lines := f.formatToolHintLines(hint)
	result := []string{}
	for _, line := range strings.Split(lines, "\n") {
		if strings.TrimSpace(line) != "" {
			result = append(result, f.cfg.ToolHintPrefix+" "+line)
		}
	}
	return strings.Join(result, "\n")
}

func (f *FeishuChannel) formatToolHintLines(hint string) string {
	parts := []string{}
	buf := []rune{}
	depth := 0
	inString := false
	quoteChar := rune(0)
	escaped := false

	runes := []rune(hint)
	for i, r := range runes {
		buf = append(buf, r)

		if inString {
			if escaped {
				escaped = false
			} else if r == '\\' {
				escaped = true
			} else if r == quoteChar {
				inString = false
			}
			continue
		}

		if r == '"' || r == '\'' {
			inString = true
			quoteChar = r
			continue
		}

		if r == '(' {
			depth++
			continue
		}

		if r == ')' && depth > 0 {
			depth--
			continue
		}

		if r == ',' && depth == 0 {
			nextChar := rune(0)
			if i+1 < len(runes) {
				nextChar = runes[i+1]
			}
			if nextChar == ' ' {
				parts = append(parts, strings.TrimRight(string(buf), " \t\n\r"))
				buf = []rune{}
			}
		}
	}

	if len(buf) > 0 {
		parts = append(parts, strings.TrimSpace(string(buf)))
	}

	return strings.Join(parts, "\n")
}

func (f *FeishuChannel) refreshToken() error {
	reqBody := map[string]string{
		"app_id":     f.cfg.AppID,
		"app_secret": f.cfg.AppSecret,
	}

	body, _ := json.Marshal(reqBody)

	resp, err := f.httpClient.Post(
		f.getDomain()+"/open-apis/auth/v3/tenant_access_token/internal",
		"application/json",
		bytes.NewReader(body))
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	var result struct {
		Code              int    `json:"code"`
		Msg               string `json:"msg"`
		TenantAccessToken string `json:"tenant_access_token"`
		Expire            int    `json:"expire"`
	}

	if err := json.NewDecoder(io.LimitReader(resp.Body, 64*1024)).Decode(&result); err != nil {
		return err
	}

	if result.Code != 0 {
		return fmt.Errorf("feishu token error: %s (code: %d)", result.Msg, result.Code)
	}

	f.mu.Lock()
	f.tenantToken = result.TenantAccessToken
	f.tokenTTL = time.Duration(result.Expire) * time.Second
	f.tokenExpiry = time.Now().Add(f.tokenTTL)
	f.mu.Unlock()

	logger.Debug("Feishu token refreshed", logger.String("expiry", f.tokenExpiry.String()))

	return nil
}

// streamBufCleanupLoop 定期清理超时的 streamBuf，防止异常退出时内存泄漏
func (f *FeishuChannel) streamBufCleanupLoop() {
	const (
		cleanupInterval = 5 * time.Minute
		maxBufAge       = 30 * time.Minute
	)
	ticker := time.NewTicker(cleanupInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			f.mu.Lock()
			now := time.Now()
			for key, buf := range f.streamBufs {
				if !buf.createdAt.IsZero() && now.Sub(buf.createdAt) > maxBufAge {
					delete(f.streamBufs, key)
					logger.Debug("Cleaned up stale streamBuf", logger.String("key", key))
				}
			}
			f.mu.Unlock()
		case <-f.stopChan:
			return
		}
	}
}

func (f *FeishuChannel) tokenRefreshLoop() {
	// 由 30 分钟改为 1 分钟：配合「总有效期 1/10 提前刷新」，
	// 过期最迟在 1 分钟内被发现
	ticker := time.NewTicker(time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			if f.tokenNeedsRefresh() {
				if err := f.refreshToken(); err != nil {
					logger.Warn("Failed to refresh feishu token", logger.ErrorField(err))
				}
			}

		case <-f.stopChan:
			return
		}
	}
}

// feishuAPIResponse 飞书 API 通用响应结构
type feishuAPIResponse struct {
	Code int             `json:"code"`
	Msg  string          `json:"msg"`
	Data json.RawMessage `json:"data"`
}

// invalidTenantTokenCodes 表示 tenant_access_token 无效/过期，
// 这类错误刷新 token 后重试一次即可恢复，不必让调用方失败。
var invalidTenantTokenCodes = map[int]bool{
	99991661: true, // tenant_access_token invalid
	99991663: true, // tenant_access_token expired / not exist
	99991668: true, // token invalid（部分接口返回）
}

// isInvalidTenantToken 判断错误码是否属于「token 失效，可刷新后重试」
func isInvalidTenantToken(code int) bool {
	return invalidTenantTokenCodes[code]
}

// doAPIRequest 执行飞书 API 请求（带 JSON body），解析通用响应结构
// method: HTTP 方法（POST, PATCH, DELETE 等）
// url: 完整 API URL
// body: 请求体（会被 JSON 序列化），传 nil 表示无 body
// result: 响应 Data 字段的反序列化目标，传 nil 表示忽略 data
func (f *FeishuChannel) doAPIRequest(ctx context.Context, method, url string, body interface{}, result interface{}) error {
	// 最多尝试两次：命中 token 失效码时强制刷新并重试一次。
	// ensureToken 只在 token 为空时刷新，一旦服务端判定其失效，
	// 后续调用会连续失败到下一个刷新窗口（表现为「机器人忽然不回消息」）。
	for attempt := 0; attempt < 2; attempt++ {
		err := f.doAPIRequestOnce(ctx, method, url, body, result)
		if err == nil {
			return nil
		}
		if apiErr, ok := err.(*feishuAPIError); ok && isInvalidTenantToken(apiErr.Code) && attempt == 0 {
			logger.Warn("Feishu token invalid, refreshing and retrying",
				logger.Int("code", apiErr.Code),
				logger.String("url", url))
			if refreshErr := f.refreshToken(); refreshErr != nil {
				logger.Error("Failed to refresh Feishu token", logger.ErrorField(refreshErr))
				return err
			}
			continue
		}
		return err
	}
	return fmt.Errorf("feishu api request failed after retry: %s", url)
}

// feishuAPIError 携带飞书返回的 code，供上层判断是否可重试
type feishuAPIError struct {
	Msg    string
	Code   int
	Status int
	Body   string
}

func (e *feishuAPIError) Error() string {
	return fmt.Sprintf("feishu api error: %s (code: %d, status: %d, body: %s)",
		e.Msg, e.Code, e.Status, e.Body)
}

func (f *FeishuChannel) doAPIRequestOnce(ctx context.Context, method, url string, body interface{}, result interface{}) error {
	token, err := f.ensureToken()
	if err != nil {
		return err
	}

	var reqBody io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			return fmt.Errorf("marshal request body: %w", err)
		}
		reqBody = bytes.NewReader(data)
	}

	req, err := http.NewRequestWithContext(ctx, method, url, reqBody)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := f.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(io.LimitReader(resp.Body, maxFeishuBodySize))

	var apiResp feishuAPIResponse
	if err := json.Unmarshal(respBody, &apiResp); err != nil {
		return fmt.Errorf("unmarshal feishu api response: %w (status: %d, body: %s)",
			err, resp.StatusCode, truncateString(string(respBody), 500))
	}

	if resp.StatusCode != http.StatusOK || apiResp.Code != 0 {
		return &feishuAPIError{
			Msg:    apiResp.Msg,
			Code:   apiResp.Code,
			Status: resp.StatusCode,
			Body:   truncateString(string(respBody), 500),
		}
	}

	if result != nil && len(apiResp.Data) > 0 {
		if err := json.Unmarshal(apiResp.Data, result); err != nil {
			return fmt.Errorf("unmarshal response data: %w", err)
		}
	}

	return nil
}

func truncateString(s string, max int) string {
	if max <= 0 || len(s) <= max {
		return s
	}
	return s[:max] + "..."
}

func escapeJSON(s string) string {
	var buf bytes.Buffer
	for _, r := range s {
		switch r {
		case '"':
			buf.WriteString(`\"`)
		case '\\':
			buf.WriteString(`\\`)
		case '\n':
			buf.WriteString(`\n`)
		case '\r':
			buf.WriteString(`\r`)
		case '\t':
			buf.WriteString(`\t`)
		default:
			buf.WriteRune(r)
		}
	}
	return buf.String()
}
