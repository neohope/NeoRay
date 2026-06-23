package memory

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"neoray/internal/config"
	"neoray/internal/logger"
	"neoray/internal/provider"
	"neoray/internal/session"
	"neoray/internal/skills"
)

// MemoryManager 统一管理记忆系统的所有组件
type MemoryManager struct {
	cfg         *config.Config
	workspace   string
	initialized bool

	store         *MemoryStore
	dream         *Dream
	consolidator  *Consolidator
	autoCompact   *AutoCompact
	contextBuilder *ContextBuilder
	skillsLoader   *skills.SkillsLoader

	// 活跃会话跟踪
	activeSessions sync.Map // session ID -> struct{}

	// 会话管理器引用
	sessionMgr MemorySessionManager

	mu sync.RWMutex
}

// MemorySessionManager 会话管理器接口（供记忆系统使用）
type MemorySessionManager interface {
	GetSession(sessionID string) (*session.Session, error)
	SaveSession(sess *session.Session) error
	ListSessions() ([]*session.Session, error)
	DeleteSession(sessionID string) error
}

// MemoryManagerOption MemoryManager 选项
type MemoryManagerOption func(*MemoryManager)

// WithSessionManager 设置会话管理器
func WithSessionManager(sessionMgr MemorySessionManager) MemoryManagerOption {
	return func(m *MemoryManager) {
		m.sessionMgr = sessionMgr
	}
}

// NewMemoryManager 创建 MemoryManager
func NewMemoryManager(cfg *config.Config, p provider.Provider, model string, sessionMgr MemorySessionManager, opts ...MemoryManagerOption) *MemoryManager {
	m := &MemoryManager{
		cfg:        cfg,
		workspace:  cfg.Memory.Workspace,
		sessionMgr: sessionMgr,
	}

	if m.workspace == "" {
		m.workspace = config.GetWorkspace()
	}

	// 应用选项
	for _, opt := range opts {
		opt(m)
	}

	// 初始化
	m.Initialize(p, model)

	return m
}

// Initialize 初始化记忆系统
func (m *MemoryManager) Initialize(p provider.Provider, model string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.initialized {
		return
	}

	logger.Info("Initializing memory system", logger.String("workspace", m.workspace))

	// 创建 MemoryStore
	storeOpts := make([]MemoryStoreOption, 0)
	if m.cfg.Memory.MaxHistoryEntries > 0 {
		storeOpts = append(storeOpts, WithMaxHistoryEntries(m.cfg.Memory.MaxHistoryEntries))
	}
	m.store = NewMemoryStore(m.workspace, storeOpts...)

	// 初始化默认文件
	if err := m.store.InitializeDefaultFiles(); err != nil {
		logger.Warn("Failed to initialize default memory files", logger.ErrorField(err))
	}

	// 初始化 Git（如果启用）
	if m.cfg.Memory.GitEnabled {
		if m.store.Git() != nil {
			if !m.store.Git().IsInitialized() {
				if m.store.Git().Init() {
					logger.Info("Git store initialized")
				}
			}
		}
	}

	// 创建 SkillsLoader
	skillsOpts := make([]skills.SkillsLoaderOption, 0)
	if m.cfg.Skills.BuiltinSkillsDir != "" {
		skillsOpts = append(skillsOpts, skills.WithBuiltinSkillsDir(m.cfg.Skills.BuiltinSkillsDir))
	}
	if len(m.cfg.Skills.DisabledSkills) > 0 {
		skillsOpts = append(skillsOpts, skills.WithDisabledSkills(m.cfg.Skills.DisabledSkills))
	}
	m.skillsLoader = skills.NewSkillsLoader(m.cfg, skillsOpts...)
	logger.Info("Skills loader initialized", logger.Bool("enabled", m.cfg.Skills.Enabled))

	// 创建 ContextBuilder
	m.contextBuilder = NewContextBuilder(m.workspace, m.store, WithSkillsLoader(m.skillsLoader))

	// 创建适配器
	adapter := NewProviderAdapter(p, model)

	// 创建 Dream
	dreamOpts := make([]DreamOption, 0)
	if m.cfg.Memory.GitEnabled {
		dreamOpts = append(dreamOpts, WithDreamAnnotateLineAges(true))
	}
	m.dream = NewDream(m.store, adapter, model, dreamOpts...)
	logger.Info("Dream initialized")

	// 创建 Consolidator
	if m.sessionMgr != nil {
		consolidatorOpts := make([]ConsolidatorOption, 0)
		m.consolidator = NewConsolidator(
			m.store,
			adapter,
			model,
			&sessionManagerAdapter{mgr: m.sessionMgr},
			4096, // 默认上下文窗口
			consolidatorOpts...,
		)
		logger.Info("Consolidator initialized")
		m.initAutoCompact()
	}

	m.initialized = true
	logger.Info("Memory system initialized")
}

func (m *MemoryManager) initAutoCompact() {
	if m.consolidator == nil || m.sessionMgr == nil {
		return
	}

	autoCompactOpts := make([]AutoCompactOption, 0)
	if m.cfg.Memory.SessionTTLMinutes > 0 {
		autoCompactOpts = append(autoCompactOpts, WithSessionTTLMinutes(m.cfg.Memory.SessionTTLMinutes))
	}
	m.autoCompact = NewAutoCompact(
		&autoCompactSessionAdapter{mgr: m.sessionMgr},
		m.consolidator,
		autoCompactOpts...,
	)
	logger.Info("AutoCompact initialized")
}

// IsInitialized 是否已初始化
func (m *MemoryManager) IsInitialized() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.initialized
}

// Store 获取 MemoryStore
func (m *MemoryManager) Store() *MemoryStore {
	return m.store
}

// ContextBuilder 获取 ContextBuilder
func (m *MemoryManager) ContextBuilder() *ContextBuilder {
	return m.contextBuilder
}

// TrackSession 跟踪活跃会话
func (m *MemoryManager) TrackSession(sessionID string) {
	if sessionID != "" {
		m.activeSessions.Store(sessionID, struct{}{})
	}
}

// UntrackSession 取消跟踪会话
func (m *MemoryManager) UntrackSession(sessionID string) {
	if sessionID != "" {
		m.activeSessions.Delete(sessionID)
	}
}

// GetActiveSessionKeys 获取活跃会话键列表
func (m *MemoryManager) GetActiveSessionKeys() []string {
	var keys []string
	m.activeSessions.Range(func(key, value interface{}) bool {
		if k, ok := key.(string); ok {
			keys = append(keys, k)
		}
		return true
	})
	return keys
}

// AppendHistory 记录对话历史
func (m *MemoryManager) AppendHistory(userInput, assistantResponse string) (int, error) {
	if m.store == nil {
		return 0, fmt.Errorf("memory store not initialized")
	}

	content := fmt.Sprintf("User: %s\nAssistant: %s", userInput, assistantResponse)
	return m.store.AppendHistory(content)
}

// AppendHistoryFormatted 记录格式化的历史
func (m *MemoryManager) AppendHistoryFormatted(content string) (int, error) {
	if m.store == nil {
		return 0, fmt.Errorf("memory store not initialized")
	}
	return m.store.AppendHistory(content)
}

// SkillsLoader 获取技能加载器
func (m *MemoryManager) SkillsLoader() *skills.SkillsLoader {
	return m.skillsLoader
}

// BuildSkillsSummary 构建所有可用技能的摘要
func (m *MemoryManager) BuildSkillsSummary() string {
	if m.skillsLoader == nil {
		return ""
	}
	return m.skillsLoader.BuildSkillsSummary(nil)
}

// GetAlwaysSkills 获取标记为 always 的技能
func (m *MemoryManager) GetAlwaysSkills() []string {
	if m.skillsLoader != nil && m.cfg.Skills.AutoLoadAlways {
		return m.skillsLoader.GetAlwaysSkills()
	}
	return nil
}

// BuildSystemPrompt 构建系统提示词
func (m *MemoryManager) BuildSystemPrompt(skillNames []string, channel string, sessionSummary string) string {
	if m.contextBuilder == nil {
		return ""
	}

	// 如果没有指定技能，但配置了自动加载 always 技能，则添加它们
	if len(skillNames) == 0 && m.cfg.Skills.AutoLoadAlways && m.skillsLoader != nil {
		skillNames = m.skillsLoader.GetAlwaysSkills()
	}

	return m.contextBuilder.BuildSystemPrompt(skillNames, channel, sessionSummary)
}

// ReadMemoryFile 读取记忆文件
func (m *MemoryManager) ReadMemoryFile(fileType string) (string, error) {
	if m.store == nil {
		return "", fmt.Errorf("memory store not initialized")
	}

	switch strings.ToLower(fileType) {
	case "soul", "soul.md":
		return m.store.ReadSoul(), nil
	case "user", "user.md":
		return m.store.ReadUser(), nil
	case "memory", "memory.md":
		return m.store.ReadMemory(), nil
	default:
		return "", fmt.Errorf("unknown file type: %s (supported: soul, user, memory)", fileType)
	}
}

// WriteMemoryFile 写入记忆文件
func (m *MemoryManager) WriteMemoryFile(fileType string, content string) error {
	if m.store == nil {
		return fmt.Errorf("memory store not initialized")
	}

	var err error
	switch strings.ToLower(fileType) {
	case "soul", "soul.md":
		err = m.store.WriteSoul(content)
	case "user", "user.md":
		err = m.store.WriteUser(content)
	case "memory", "memory.md":
		err = m.store.WriteMemory(content)
	default:
		return fmt.Errorf("unknown file type: %s (supported: soul, user, memory)", fileType)
	}

	if err == nil && m.cfg.Memory.GitEnabled && m.store.Git() != nil && m.store.Git().IsInitialized() {
		// 自动提交变更
		m.store.Git().AutoCommit(fmt.Sprintf("Update %s file", fileType))
	}

	return err
}

// RunDream 立即运行 Dream 处理
func (m *MemoryManager) RunDream(ctx context.Context) (bool, error) {
	if m.dream == nil {
		return false, fmt.Errorf("dream not initialized (missing provider)")
	}

	logger.Info("Running dream processing...")
	changed, err := m.dream.Run(ctx)
	if err != nil {
		logger.Error("Dream processing failed", logger.ErrorField(err))
		return false, err
	}

	if changed {
		logger.Info("Dream processing completed with changes")
	} else {
		logger.Info("Dream processing completed (no changes)")
	}

	return changed, nil
}

// CheckExpiredSessions 检查并压缩过期会话
func (m *MemoryManager) CheckExpiredSessions(ctx context.Context) {
	if m.autoCompact == nil {
		return
	}
	m.autoCompact.CheckExpired(ctx, m.GetActiveSessionKeys())
}

// GetSessionSummary 获取会话摘要（如果有）
func (m *MemoryManager) GetSessionSummary(sessionID string) (string, error) {
	if m.autoCompact == nil || m.sessionMgr == nil {
		return "", nil
	}
	_, summary, err := m.autoCompact.PrepareSession(sessionID)
	return summary, err
}

// MaybeConsolidateSession 可能压缩会话（根据 token 使用）
func (m *MemoryManager) MaybeConsolidateSession(ctx context.Context, sess *session.Session) error {
	if m.consolidator == nil {
		return nil
	}

	// 将会话转换为 map 格式供 consolidator 使用
	sessionMap := sessionToMap(sess)
	return m.consolidator.MaybeConsolidateByTokens(ctx, sessionMap, sess.ID, 10)
}

// CompactIdleSession 压缩空闲会话
func (m *MemoryManager) CompactIdleSession(ctx context.Context, sessionID string) (string, error) {
	if m.consolidator == nil {
		return "", fmt.Errorf("consolidator not initialized")
	}
	return m.consolidator.CompactIdleSession(ctx, sessionID, DefaultRecentSuffixMessages)
}

// ========== 适配层 ==========

// ProviderAdapter 适配 provider.Provider 到 DreamProvider 和 ConsolidatorProvider 接口
type ProviderAdapter struct {
	provider provider.Provider
	model    string
}

// NewProviderAdapter 创建 Provider 适配器
func NewProviderAdapter(p provider.Provider, model string) *ProviderAdapter {
	return &ProviderAdapter{
		provider: p,
		model:    model,
	}
}

// Chat 实现 DreamProvider 接口
func (p *ProviderAdapter) Chat(ctx context.Context, model string, system string, messages []interface{}) (string, error) {
	// 转换消息格式
	reqMessages := make([]provider.Message, 0, len(messages)+1)

	// 添加 system 消息
	if system != "" {
		reqMessages = append(reqMessages, provider.Message{
			Role:    "system",
			Content: system,
		})
	}

	// 添加其他消息
	for _, msg := range messages {
		if m, ok := msg.(map[string]interface{}); ok {
			role, _ := m["role"].(string)
			content, _ := m["content"].(string)
			reqMessages = append(reqMessages, provider.Message{
				Role:    role,
				Content: content,
			})
		}
	}

	// 构建请求
	req := &provider.ChatRequest{
		Model:     model,
		Messages:  reqMessages,
		MaxTokens: 2048,
	}

	resp, err := p.provider.Chat(ctx, req)
	if err != nil {
		return "", err
	}
	return resp.Content, nil
}

// sessionManagerAdapter 适配 session.Manager 到 ConsolidatorSessionManager
type sessionManagerAdapter struct {
	mgr MemorySessionManager
}

func (a *sessionManagerAdapter) GetSession(key string) (interface{}, error) {
	sess, err := a.mgr.GetSession(key)
	if err != nil {
		return nil, err
	}
	return sessionToMap(sess), nil
}

func (a *sessionManagerAdapter) SaveSession(session interface{}) error {
	sess, err := mapToSession(session)
	if err != nil {
		return err
	}
	return a.mgr.SaveSession(sess)
}

// autoCompactSessionAdapter 适配到 AutoCompactSessionManager
type autoCompactSessionAdapter struct {
	mgr MemorySessionManager
}

func (a *autoCompactSessionAdapter) GetSession(key string) (interface{}, error) {
	sess, err := a.mgr.GetSession(key)
	if err != nil {
		return nil, err
	}
	return sessionToMap(sess), nil
}

func (a *autoCompactSessionAdapter) SaveSession(session interface{}) error {
	sess, err := mapToSession(session)
	if err != nil {
		return err
	}
	return a.mgr.SaveSession(sess)
}

func (a *autoCompactSessionAdapter) ListSessions() ([]SessionInfo, error) {
	sessions, err := a.mgr.ListSessions()
	if err != nil {
		return nil, err
	}
	infos := make([]SessionInfo, 0, len(sessions))
	for _, s := range sessions {
		infos = append(infos, SessionInfo{
			Key:       s.ID,
			UpdatedAt: s.UpdatedAt,
		})
	}
	return infos, nil
}

func (a *autoCompactSessionAdapter) InvalidateSession(key string) {
	// 不执行操作，让下一次 GetSession 重新加载
}

// sessionToMap 将会话转换为 map
func sessionToMap(sess *session.Session) map[string]interface{} {
	if sess == nil {
		return nil
	}

	messages := make([]interface{}, 0, len(sess.Messages))
	for _, msg := range sess.Messages {
		msgMap := map[string]interface{}{
			"role":       msg.Role,
			"content":    msg.Content,
			"created_at": msg.Timestamp,
		}
		if len(msg.ToolCalls) > 0 {
			toolCalls := make([]map[string]interface{}, 0, len(msg.ToolCalls))
			for _, tc := range msg.ToolCalls {
				toolCalls = append(toolCalls, map[string]interface{}{
					"id":       tc.ID,
					"name":     tc.Name,
					"arguments": tc.Arguments,
				})
			}
			msgMap["tool_calls"] = toolCalls
		}
		messages = append(messages, msgMap)
	}

	return map[string]interface{}{
		"id":        sess.ID,
		"messages":  messages,
		"metadata":  sess.Metadata,
		"updated_at": sess.UpdatedAt,
	}
}

// mapToSession 从 map 恢复会话
func mapToSession(data interface{}) (*session.Session, error) {
	m, ok := data.(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("invalid session data")
	}

	id, _ := m["id"].(string)
	sess := &session.Session{
		ID: id,
	}

	if metadata, ok := m["metadata"].(map[string]interface{}); ok {
		sess.Metadata = metadata
	}

	if updatedAt, ok := m["updated_at"].(time.Time); ok {
		sess.UpdatedAt = updatedAt
	}

	return sess, nil
}
