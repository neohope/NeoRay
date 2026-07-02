package memory

// 这个文件展示如何集成记忆系统到你的应用中

/*
集成示例:

1. 初始化 MemoryStore:

   workspace := "./workspace"
   ms := memory.NewMemoryStore(workspace)

   // 初始化默认文件（可选）
   _ = ms.InitializeDefaultFiles()

   // 初始化 Git（可选）
   if !ms.Git().IsInitialized() {
       ms.Git().Init()
   }

2. 初始化 LLM Provider 适配器:

   // 实现 DreamProvider 和 ConsolidatorProvider 接口
   type MyProvider struct {
       // 你的 LLM 客户端
   }

   func (p *MyProvider) Chat(ctx context.Context, model, system string, messages []interface{}) (string, error) {
       // 调用你的 LLM API
   }

   provider := &MyProvider{}
   model := "claude-3-sonnet"

3. 初始化 Dream:

   dream := memory.NewDream(ms, provider, model)

   // 定期运行 Dream（例如每小时）
   go func() {
       ticker := time.NewTicker(1 * time.Hour)
       defer ticker.Stop()
       for range ticker.C {
           changed, _ := dream.Run(context.Background())
           if changed {
               log.Println("Dream updated memory")
           }
       }
   }()

4. 初始化 Consolidator 和 AutoCompact:

   // 首先实现会话管理器接口
   type MySessionManager struct { ... }

   sm := &MySessionManager{}

   consolidator := memory.NewConsolidator(
       ms,
       provider,
       model,
       sm,
       128000, // context window size
   )

   autoCompact := memory.NewAutoCompact(sm, consolidator,
       memory.WithSessionTTLMinutes(60 * 24), // 24小时 TTL
   )

   // 定期检查过期会话
   go func() {
       ticker := time.NewTicker(30 * time.Minute)
       defer ticker.Stop()
       for range ticker.C {
           autoCompact.CheckExpired(context.Background(), nil)
       }
   }()

5. 构建上下文:

   ctxBuilder := memory.NewContextBuilder(workspace, ms)

   // 当处理消息时:
   session, summary, _ := autoCompact.PrepareSession(sessionKey)
   systemPrompt := ctxBuilder.BuildSystemPrompt(nil, channel, summary)

   // 或者构建完整消息列表:
   messages := ctxBuilder.BuildMessages(
       historyMessages,
       userInput,
       nil, // skill names
       channel,
       chatID,
       senderID,
       summary,
       sessionMetadata,
       nil, // runtime lines
   )

6. 记录对话历史:

   ms.AppendHistory("User asked about X")
   ms.AppendHistory("I responded with Y")

完整示例见项目文档。
*/
