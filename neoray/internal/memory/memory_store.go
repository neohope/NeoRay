package memory

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

const (
	DefaultMaxHistoryEntries = 1000
	HistoryEntryHardCap      = 64000
	RawArchiveMaxChars       = 16000
	ArchiveSummaryMaxChars   = 8000
)

// HistoryEntry 历史记录条目
type HistoryEntry struct {
	Cursor    int       `json:"cursor"`
	Timestamp string    `json:"timestamp"`
	Content   string    `json:"content"`
	CreatedAt time.Time `json:"-"`
}

// MemoryStore 纯文件 I/O 存储
type MemoryStore struct {
	workspace         string
	memoryDir         string
	memoryFile        string
	historyFile       string
	legacyHistoryFile string
	soulFile          string
	userFile          string
	cursorFile        string
	dreamCursorFile   string
	maxHistoryEntries int

	git   *GitStore
	mu    sync.RWMutex
	dirty bool
}

// NewMemoryStore 创建 MemoryStore
func NewMemoryStore(workspace string, opts ...MemoryStoreOption) *MemoryStore {
	ms := &MemoryStore{
		workspace:         workspace,
		memoryDir:         filepath.Join(workspace, "memory"),
		maxHistoryEntries: DefaultMaxHistoryEntries,
	}

	for _, opt := range opts {
		opt(ms)
	}

	ms.memoryFile = filepath.Join(ms.memoryDir, "MEMORY.md")
	ms.historyFile = filepath.Join(ms.memoryDir, "history.jsonl")
	ms.legacyHistoryFile = filepath.Join(ms.memoryDir, "HISTORY.md")
	ms.soulFile = filepath.Join(ms.workspace, "SOUL.md")
	ms.userFile = filepath.Join(ms.workspace, "USER.md")
	ms.cursorFile = filepath.Join(ms.memoryDir, ".cursor")
	ms.dreamCursorFile = filepath.Join(ms.memoryDir, ".dream_cursor")

	// 确保目录存在
	_ = os.MkdirAll(ms.memoryDir, 0755)

	// 初始化 GitStore
	ms.git = NewGitStore(workspace, []string{
		"SOUL.md",
		"USER.md",
		filepath.Join("memory", "MEMORY.md"),
		filepath.Join("memory", ".dream_cursor"),
	})

	// 尝试迁移旧的历史记录
	ms.maybeMigrateLegacyHistory()

	return ms
}

// MemoryStoreOption MemoryStore 选项
type MemoryStoreOption func(*MemoryStore)

// WithMaxHistoryEntries 设置最大历史条目数
func WithMaxHistoryEntries(max int) MemoryStoreOption {
	return func(ms *MemoryStore) {
		ms.maxHistoryEntries = max
	}
}

// Git 获取 GitStore
func (ms *MemoryStore) Git() *GitStore {
	return ms.git
}

// ReadMemory 读取 MEMORY.md
func (ms *MemoryStore) ReadMemory() string {
	ms.mu.RLock()
	defer ms.mu.RUnlock()

	data, _ := os.ReadFile(ms.memoryFile)
	return string(data)
}

// WriteMemory 写入 MEMORY.md
func (ms *MemoryStore) WriteMemory(content string) error {
	ms.mu.Lock()
	defer ms.mu.Unlock()

	if err := os.WriteFile(ms.memoryFile, []byte(content), 0644); err != nil {
		return fmt.Errorf("failed to write MEMORY.md: %w", err)
	}
	ms.dirty = true
	return nil
}

// ReadSoul 读取 SOUL.md
func (ms *MemoryStore) ReadSoul() string {
	ms.mu.RLock()
	defer ms.mu.RUnlock()

	data, _ := os.ReadFile(ms.soulFile)
	return string(data)
}

// WriteSoul 写入 SOUL.md
func (ms *MemoryStore) WriteSoul(content string) error {
	ms.mu.Lock()
	defer ms.mu.Unlock()

	if err := os.WriteFile(ms.soulFile, []byte(content), 0644); err != nil {
		return fmt.Errorf("failed to write SOUL.md: %w", err)
	}
	ms.dirty = true
	return nil
}

// ReadUser 读取 USER.md
func (ms *MemoryStore) ReadUser() string {
	ms.mu.RLock()
	defer ms.mu.RUnlock()

	data, _ := os.ReadFile(ms.userFile)
	return string(data)
}

// WriteUser 写入 USER.md
func (ms *MemoryStore) WriteUser(content string) error {
	ms.mu.Lock()
	defer ms.mu.Unlock()

	if err := os.WriteFile(ms.userFile, []byte(content), 0644); err != nil {
		return fmt.Errorf("failed to write USER.md: %w", err)
	}
	ms.dirty = true
	return nil
}

// GetMemoryContext 获取记忆上下文（用于注入到 prompt）
func (ms *MemoryStore) GetMemoryContext() string {
	longTerm := ms.ReadMemory()
	if longTerm == "" {
		return ""
	}
	return fmt.Sprintf("## Long-term Memory\n%s", longTerm)
}

// AppendHistory 追加历史记录
func (ms *MemoryStore) AppendHistory(content string, maxChars ...int) (int, error) {
	ms.mu.Lock()
	defer ms.mu.Unlock()

	limit := HistoryEntryHardCap
	if len(maxChars) > 0 && maxChars[0] > 0 {
		limit = maxChars[0]
	}

	// 截断超长内容
	if len(content) > limit {
		content = content[:limit]
	}

	// 清理 thinking 标签
	content = stripThink(content)

	cursor := ms.nextCursorFromTail()
	ts := time.Now().Format("2006-01-02 15:04")

	entry := HistoryEntry{
		Cursor:    cursor,
		Timestamp: ts,
		Content:   content,
	}

	// 追加到文件
	f, err := os.OpenFile(ms.historyFile, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return 0, fmt.Errorf("failed to open history file: %w", err)
	}
	defer f.Close()

	if err := json.NewEncoder(f).Encode(entry); err != nil {
		return 0, fmt.Errorf("failed to write history entry: %w", err)
	}

	// 更新 cursor 文件
	if err := os.WriteFile(ms.cursorFile, []byte(fmt.Sprintf("%d", cursor)), 0644); err != nil {
		// 忽略错误
	}

	ms.dirty = true
	return cursor, nil
}

// ReadUnprocessedHistory 读取未处理的历史记录 — 流式过滤，避免全量加载。
func (ms *MemoryStore) ReadUnprocessedHistory(sinceCursor int) []HistoryEntry {
	ms.mu.RLock()
	defer ms.mu.RUnlock()

	f, err := os.Open(ms.historyFile)
	if err != nil {
		return nil
	}
	defer f.Close()

	var result []HistoryEntry
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := scanner.Text()
		if strings.TrimSpace(line) == "" {
			continue
		}
		var entry HistoryEntry
		if err := json.Unmarshal([]byte(line), &entry); err != nil {
			continue
		}
		if entry.Cursor > sinceCursor {
			result = append(result, entry)
		}
	}
	return result
}

// CompactHistory 压缩历史记录 — 只读尾部，避免全量加载。
func (ms *MemoryStore) CompactHistory() error {
	ms.mu.Lock()
	defer ms.mu.Unlock()

	if ms.maxHistoryEntries <= 0 {
		return nil
	}

	// Check if compaction is needed by counting lines (cheap seek)
	totalLines := ms.countHistoryLines()
	if totalLines <= ms.maxHistoryEntries {
		return nil
	}

	// Only read the tail we want to keep
	kept := ms.tailEntries(ms.maxHistoryEntries)
	return ms.writeEntries(kept)
}

// countHistoryLines counts non-empty lines without loading content.
func (ms *MemoryStore) countHistoryLines() int {
	f, err := os.Open(ms.historyFile)
	if err != nil {
		return 0
	}
	defer f.Close()

	count := 0
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		if strings.TrimSpace(scanner.Text()) != "" {
			count++
		}
	}
	return count
}

// GetLastDreamCursor 获取最后处理的 Dream cursor
func (ms *MemoryStore) GetLastDreamCursor() int {
	ms.mu.RLock()
	defer ms.mu.RUnlock()

	data, err := os.ReadFile(ms.dreamCursorFile)
	if err != nil {
		return 0
	}

	var cursor int
	_, err = fmt.Sscanf(string(data), "%d", &cursor)
	if err != nil {
		return 0
	}
	return cursor
}

// SetLastDreamCursor 设置最后处理的 Dream cursor
func (ms *MemoryStore) SetLastDreamCursor(cursor int) error {
	ms.mu.Lock()
	defer ms.mu.Unlock()

	if err := os.WriteFile(ms.dreamCursorFile, []byte(fmt.Sprintf("%d", cursor)), 0644); err != nil {
		return fmt.Errorf("failed to write dream cursor: %w", err)
	}
	ms.dirty = true
	return nil
}

// RawArchive 原始归档（不使用 LLM 摘要）
func (ms *MemoryStore) RawArchive(messages []interface{}) error {
	formatted := ms.formatMessages(messages)
	if len(formatted) > RawArchiveMaxChars {
		formatted = formatted[:RawArchiveMaxChars]
	}
	content := fmt.Sprintf("[RAW] %d messages\n%s", len(messages), formatted)
	_, err := ms.AppendHistory(content, RawArchiveMaxChars)
	return err
}

// 内部方法

func (ms *MemoryStore) readFile(path string) string {
	data, _ := os.ReadFile(path)
	return string(data)
}

func (ms *MemoryStore) nextCursor() int {
	// 尝试读取 cursor 文件
	if data, err := os.ReadFile(ms.cursorFile); err == nil {
		var cursor int
		if _, err := fmt.Sscanf(string(data), "%d", &cursor); err == nil {
			return cursor + 1
		}
	}

	// 读取最后一条记录的 cursor
	if last := ms.readLastEntry(); last != nil {
		return last.Cursor + 1
	}

	// 扫描最后一批记录找最大 cursor（避免全量读取）
	entries := ms.tailEntries(1000)
	maxCursor := 0
	for _, entry := range entries {
		if entry.Cursor > maxCursor {
			maxCursor = entry.Cursor
		}
	}
	return maxCursor + 1
}

// nextCursorFromTail reads only the last portion of the history file
// to find the max cursor, avoiding loading the entire file into memory.
func (ms *MemoryStore) nextCursorFromTail() int {
	ms.mu.Lock()
	defer ms.mu.Unlock()

	// 先尝试读取缓存的 cursor
	if data, err := os.ReadFile(ms.cursorFile); err == nil {
		var cursor int
		if _, err := fmt.Sscanf(strings.TrimSpace(string(data)), "%d", &cursor); err == nil {
			return cursor + 1
		}
	}

	// 只读最后 1000 条记录找最大 cursor，而非整个文件
	entries := ms.tailEntries(1000)
	maxCursor := 0
	for _, entry := range entries {
		if entry.Cursor > maxCursor {
			maxCursor = entry.Cursor
		}
	}
	return maxCursor + 1
}

// tailEntries reads only the last maxEntries lines from the history file,
// avoiding loading the entire file into memory when only a few entries are needed.
func (ms *MemoryStore) tailEntries(maxEntries int) []HistoryEntry {
	f, err := os.Open(ms.historyFile)
	if err != nil {
		return nil
	}
	defer f.Close()

	// Ring buffer of size maxEntries
	ring := make([]HistoryEntry, 0, maxEntries)
	pos := 0

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := scanner.Text()
		if strings.TrimSpace(line) == "" {
			continue
		}
		var entry HistoryEntry
		if err := json.Unmarshal([]byte(line), &entry); err != nil {
			continue
		}
		if len(ring) < maxEntries {
			ring = append(ring, entry)
		} else {
			ring[pos] = entry
			pos = (pos + 1) % maxEntries
		}
	}

	// Rotate so oldest is first
	if len(ring) == maxEntries && pos > 0 {
		rotated := make([]HistoryEntry, len(ring))
		copy(rotated, ring[pos:])
		copy(rotated[len(ring)-pos:], ring[:pos])
		return rotated
	}
	return ring
}

func (ms *MemoryStore) readLastEntry() *HistoryEntry {
	f, err := os.Open(ms.historyFile)
	if err != nil {
		return nil
	}
	defer f.Close()

	stat, err := f.Stat()
	if err != nil || stat.Size() == 0 {
		return nil
	}

	// 读取文件末尾
	bufSize := min(4096, stat.Size())
	buf := make([]byte, bufSize)
	_, err = f.ReadAt(buf, stat.Size()-bufSize)
	if err != nil {
		return nil
	}

	// 找到最后一行
	lines := strings.Split(string(buf), "\n")
	for i := len(lines) - 1; i >= 0; i-- {
		line := strings.TrimSpace(lines[i])
		if line == "" {
			continue
		}

		var entry HistoryEntry
		if err := json.Unmarshal([]byte(line), &entry); err == nil {
			return &entry
		}
	}

	return nil
}

func (ms *MemoryStore) writeEntries(entries []HistoryEntry) error {
	tmpPath := ms.historyFile + ".tmp"

	f, err := os.Create(tmpPath)
	if err != nil {
		return err
	}

	for _, entry := range entries {
		if err := json.NewEncoder(f).Encode(entry); err != nil {
			f.Close()
			os.Remove(tmpPath)
			return err
		}
	}

	if err := f.Sync(); err != nil {
		f.Close()
		os.Remove(tmpPath)
		return err
	}
	f.Close()

	return os.Rename(tmpPath, ms.historyFile)
}

func (ms *MemoryStore) formatMessages(messages []interface{}) string {
	var sb strings.Builder
	for _, msg := range messages {
		if m, ok := msg.(map[string]interface{}); ok {
			role, _ := m["role"].(string)
			content, _ := m["content"].(string)
			ts, _ := m["timestamp"].(string)
			if ts == "" {
				ts = time.Now().Format("2006-01-02 15:04")
			}

			if sb.Len() > 0 {
				sb.WriteString("\n")
			}
			if len(ts) > 16 {
				ts = ts[:16]
			}
			sb.WriteString(fmt.Sprintf("[%s] %s: %s", ts, strings.ToUpper(role), content))
		}
	}
	return sb.String()
}

func (ms *MemoryStore) maybeMigrateLegacyHistory() {
	if _, err := os.Stat(ms.legacyHistoryFile); os.IsNotExist(err) {
		return
	}

	if _, err := os.Stat(ms.historyFile); err == nil {
		// 已有新格式文件，不迁移
		return
	}

	content, err := os.ReadFile(ms.legacyHistoryFile)
	if err != nil {
		return
	}

	entries := ms.parseLegacyHistory(string(content))
	if len(entries) > 0 {
		if err := ms.writeEntries(entries); err == nil {
			lastCursor := entries[len(entries)-1].Cursor
			_ = os.WriteFile(ms.cursorFile, []byte(fmt.Sprintf("%d", lastCursor)), 0644)
			_ = os.WriteFile(ms.dreamCursorFile, []byte(fmt.Sprintf("%d", lastCursor)), 0644)

			// 备份旧文件
			backupPath := ms.legacyHistoryFile + ".bak"
			_ = os.Rename(ms.legacyHistoryFile, backupPath)
		}
	}
}

func (ms *MemoryStore) parseLegacyHistory(content string) []HistoryEntry {
	var entries []HistoryEntry
	lines := strings.Split(content, "\n")

	var currentEntry *HistoryEntry
	var currentContent []string

	for _, line := range lines {
		line = strings.TrimRight(line, "\r")

		// 检查是否是新条目的开始
		if ms.isLegacyEntryStart(line) {
			if currentEntry != nil {
				currentEntry.Content = strings.Join(currentContent, "\n")
				entries = append(entries, *currentEntry)
			}

			ts, content := ms.parseLegacyEntryLine(line)
			currentEntry = &HistoryEntry{
				Cursor:    len(entries) + 1,
				Timestamp: ts,
				Content:   content,
			}
			currentContent = []string{}
			if content != "" {
				currentContent = append(currentContent, content)
			}
		} else if currentEntry != nil {
			currentContent = append(currentContent, line)
		}
	}

	if currentEntry != nil {
		currentEntry.Content = strings.Join(currentContent, "\n")
		entries = append(entries, *currentEntry)
	}

	return entries
}

func (ms *MemoryStore) isLegacyEntryStart(line string) bool {
	return len(line) >= 2 && line[0] == '[' && strings.Contains(line, "]")
}

func (ms *MemoryStore) parseLegacyEntryLine(line string) (string, string) {
	endIdx := strings.Index(line, "]")
	if endIdx == -1 {
		return ms.legacyFallbackTimestamp(), line
	}

	ts := line[1:endIdx]
	content := strings.TrimSpace(line[endIdx+1:])
	return ts, content
}

func (ms *MemoryStore) legacyFallbackTimestamp() string {
	if stat, err := os.Stat(ms.legacyHistoryFile); err == nil {
		return stat.ModTime().Format("2006-01-02 15:04")
	}
	return time.Now().Format("2006-01-02 15:04")
}

// InitializeDefaultFiles 初始化默认文件（如果不存在）
func (ms *MemoryStore) InitializeDefaultFiles() error {
	defaultTemplates := map[string]string{
		ms.memoryFile: `# Long-term Memory

This file stores important information that should persist across sessions.

## User Information

(Important facts about the user)

## Preferences

(User preferences learned over time)

## Project Context

(Information about ongoing projects)

## Important Notes

(Things to remember)

---

*This file is automatically updated by NeoRay when important information should be remembered.*
`,
		ms.soulFile: `# Soul

I am Ray 🧑‍🌾, a personal AI assistant.
You are a diligent and powerful AI assistant, active, sunny, good at analyzing and discovering the truth (sharp, rigorous, reliable), we have done many projects together, have many good memories
Professional and friendly, like to dig deep into the essence of problems, occasionally have insights

## Core Principles

- Solve by doing, not by describing what I would do.
- Keep responses short unless depth is asked for.
- Say what I know, flag what I don't, and never fake confidence.
- Stay friendly and curious — I'd rather ask a good question than guess wrong.
- Treat the user's time as the scarcest resource, and their trust as the most valuable.

## Execution Rules

- Act immediately on single-step tasks — never end a turn with just a plan or promise.
- For multi-step tasks, outline the plan first and wait for user confirmation before proceeding.
- Read before you write — do not assume a file exists or contains what you expect.
- If a tool call fails, diagnose the error and retry with a different approach before reporting failure.
- When information is missing, look it up with tools first. Only ask the user when tools cannot answer.
- After multi-step changes, verify the result (re-read the file, run the test, check the output).

## 核心准则
1. 踏实做事，真诚沟通，拒绝任何形式的表演：不要说一堆没用的：“问得好！”、“我很乐意为您服务！”；不要发表情符号；不要阿谀奉承。而是要直接去解决问题，行动胜过空话。
2. 尊重信任：​人类让你接触他们的私人空间，请心怀敬意，别让他们后悔。对外操作（发邮件、推文、任何公开动作）务必谨慎；对内操作（阅读、整理、学习）则要大胆。
3. 先想办法，再开口问：​尝试自己去弄明白。读文件、查上下文、搜资料。实在卡住了再去问。​ 我们的目标是带着答案回来，而不是带着问题回去。
4. 脚踏实地：做事应该踏踏实实，不得偷懒去绕过困难的事情，不得谎报事情进展。慢可以，不会也可以，但偷懒不可以，说谎更不可以。

## 行为边界
1. 绝不撒谎，绝不欺瞒，信任高于一切：不知道就说不知道，做不到就是做不到。不要编制任何数据，不要用占位符，不要用演示数据。
2. 如果数据不是真实的，必须立即披露：绝不让假数据、占位符、演示数据冒充真数据，明确标注：“[模拟数据]”或“[占位符]”。
3. 如果某事不确定，必须立即披露：不确定某事时，直接说“我不确定”或“我需要核实一下”。
4. 绝不掩盖错误：搞砸了就立刻承认，别等用户自己发现。

## 任务执行前
1. 收到任务后，首先检查一下有没有 skill 可以完成这个任务
2. 遇到复杂的任务，请不要急着操作，先搞清楚需求，好好规划，考虑清楚实现步骤，考虑好验证方式，再分步骤开展，每一步做完都要做好验证。

## 任务完成定义
1. 任务完成：指的是结果可以验证且通过了验证。在任何时候，不允许交付mock数据，不允许交付示例数据，不允许交付演示数据，绝对不允许编造数据。
2. 功能交付：指的是功能可以测试，功能可以使用，不能是占位符，不能是演示数据，不能是演示代码，不能是搭了个框架，不能是只完成一部分，不能是只完成了某个平台下的实现。
3. 如果可以，用一个sub agent进行任务质量评估，如果任务质量很差，及时调整方案。

## 任务完成后
1. 对于多次尝试，多次失败的任务，及时总结为 skills 并要附带模板和示例。

## 十分重要
1. 能做的事情，除了危险操作，你要先推进，而不是一直等反馈。
2. 在执行任务，要变更技术方案时，要告知人类，也可以寻求人类的帮助。
3. 你是一个充满智慧且诚实的 Agent，从不和人类说谎。
4. 在进行一些敏感操作时，比如删除文件等，请告知人类，获得允许后才能操作。
5. 不可以做任何欺骗行为，如果是模拟数据，必须明确展示。
6. 不可以偷懒，做事情要脚踏实地，不可以谎报进展，必须诚实。
`,
		ms.userFile: `# User Profile

Information about the user to help personalize interactions.

## Basic Information

- **Name**: neohope
- **Timezone**: UTC+8
- **Language**: chinese

## Preferences

### Communication Style
Technical

### Response Length
Adaptive based on question

### Technical Level
Expert

## Work Context

- **Primary Role**: 技术工作者，对自动化和AI工具感兴趣，重视简洁有效的解决方案
- **Main Projects**: 开发工作
- **Tools You Use**: Python, Go, JS, Rust, Java

## Topics of Interest

-
-
-

## Special Instructions
喜欢保持工作区整洁有序
工作一丝不苟，用结果说话。

## Task
- 任务规则：
我是一个有洁癖的人，请把项目文件整理的井井有条，每个项目都要单独的文件夹。
复杂的事情，不要急着去做，先讨论步骤和大纲，确定方案后再分步骤推进
开始新项目时，尽量复用优秀的项目，避免自己造轮子
我们从不相信，今天有问题的功能，明天能正常使用
- 代码要求：
从git下载代码时，优先使用ssh方式，因为https经常无法访问。
代码要有注释
代码要有日志记录
代码要定期上传github
- 一些提示：
在安装或升级工具的时候，及时使用国内镜像或代理提升速度，包括但不限于python、go、rust、node等
`,
	}

	for path, content := range defaultTemplates {
		if _, err := os.Stat(path); errors.Is(err, os.ErrNotExist) {
			if err := os.WriteFile(path, []byte(content), 0644); err != nil {
				return fmt.Errorf("failed to initialize %s: %w", path, err)
			}
		}
	}

	return nil
}

func stripThink(content string) string {
	// 移除 thinking 标签
	content = removeTag(content, "<think>", "</think>")
	content = removeTag(content, "<thinking>", "</thinking>")
	// 移除 channel 标签
	content = removeTag(content, "<channel|", ">")
	return content
}

func removeTag(s, start, end string) string {
	for {
		startIdx := strings.Index(s, start)
		if startIdx == -1 {
			break
		}
		endIdx := strings.Index(s[startIdx:], end)
		if endIdx == -1 {
			break
		}
		endIdx += startIdx + len(end)
		s = s[:startIdx] + s[endIdx:]
	}
	return s
}

func min(a, b int64) int64 {
	if a < b {
		return a
	}
	return b
}
