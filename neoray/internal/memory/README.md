# Memory Package - 长期记忆系统

这个包实现了完整的长期记忆系统，包括：

## 组件

### 1. MemoryStore - 工作区文件存储
- 管理 `MEMORY.md`、`SOUL.md`、`USER.md`
- 历史记录存储 `history.jsonl`
- 支持从旧格式迁移
- 原子文件写入

```go
ms := memory.NewMemoryStore("./workspace")
ms.InitializeDefaultFiles()  // 初始化默认文件
content := ms.ReadMemory()    // 读取长期记忆
ms.WriteMemory(newContent)    // 更新记忆
ms.AppendHistory("important event")  // 添加历史记录
```

### 2. GitStore - Git 版本控制
- 自动提交记忆文件变更
- 行级历史追溯（用于标记陈旧信息）
- 回滚支持

```go
git := ms.Git()
if !git.IsInitialized() {
    git.Init()
}
sha := git.AutoCommit("updated memory")  // 自动提交变更
```

### 3. Dream - 两阶段记忆压缩
- Phase 1: 分析历史记录，提取关键信息
- Phase 2: 使用工具编辑记忆文件
- 定期运行来保持记忆更新

```go
dream := memory.NewDream(ms, llmProvider, "claude-3-sonnet")
changed, err := dream.Run(ctx)
```

### 4. Consolidator - 轻量级压缩
- Token 预算触发的会话压缩
- 将旧对话摘要归档到历史记录
- 保持会话轻量

```go
consolidator := memory.NewConsolidator(ms, provider, model, sessionMgr, 128000)
err := consolidator.MaybeConsolidateByTokens(ctx, session, key, 50)
```

### 5. AutoCompact - 过期会话自动归档
- 定期检查空闲会话
- TTL 配置
- 自动加载摘要到上下文

```go
autoCompact := memory.NewAutoCompact(sessionMgr, consolidator,
    memory.WithSessionTTLMinutes(60 * 24))
autoCompact.CheckExpired(ctx, activeSessions)
session, summary, err := autoCompact.PrepareSession(sessionKey)
```

### 6. ContextBuilder - 上下文注入
- 构建系统提示词
- 注入记忆、技能、摘要
- 构建完整消息列表

```go
ctxBuilder := memory.NewContextBuilder(workspace, ms)
systemPrompt := ctxBuilder.BuildSystemPrompt(skills, channel, sessionSummary)
messages := ctxBuilder.BuildMessages(history, input, skills, ...)
```

## 文件结构

```
workspace/
├── SOUL.md           // AI 身份和原则
├── USER.md           // 用户配置和偏好
├── AGENTS.md         // 代理配置（可选）
└── memory/
    ├── MEMORY.md     // 长期记忆
    ├── history.jsonl // 历史记录
    ├── .cursor       // 当前游标
    └── .dream_cursor // Dream 处理游标
```

## 完整流程

1. 初始化所有组件
2. 消息处理时，使用 ContextBuilder 注入记忆
3. 会话过程中，Consolidator 按需压缩
4. 定期运行 Dream 处理历史记录
5. 后台运行 AutoCompact 管理空闲会话
6. Git 自动提交所有变更

## 配置选项

所有组件都提供 Functional Options 模式进行配置：

```go
WithMaxHistoryEntries(2000)
WithDreamMaxBatchSize(30)
WithSessionTTLMinutes(60 * 24 * 7)
WithTimezone("Asia/Shanghai")
```
