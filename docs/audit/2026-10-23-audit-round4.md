# NeoRay 代码审计（第四轮 · 全量扫描）

- 审计日期：2026-10-23
- 基线提交：`55bbc24`
- 范围：**全部后端代码**——19 个包、76 个非测试文件、31,667 行
- 方法（与前三轮的区别：这次是**全量通读**，不是抽查）：
  1. 按包把 76 个文件分成 7 组，逐组**完整读完全部文件**
  2. 机械检查：`govulncheck`（依赖漏洞，前三轮没做）、`staticcheck`（全检查类）、`go test -race`、`go test -cover`（覆盖率）
  3. **每一条发现都由我回到源码逐行复核**；能用命令验证的直接跑（git、JSON 编码、cron 调度实测）
- 结果：**29 项确认问题**（P0 3 / P1 12 / P2 14）+ 1 项依赖安全 + 1 项工程化

> 关于「组 B（agent/subagent/command）」：通读后未发现新问题。该包在第二、三轮已被集中修过（工具结果截断、会话锁超时、panic 兜底），本轮复核确认那些修复仍在生效。

---

## 一、P0：会让功能整体失效或丢会话

### R1. 运行期新增/修改 cron 任务 → 调度器永久停摆 ✅ 已修

- 位置：`internal/cron/scheduler.go:553`（`AddJob` 里 `cs.timer.Stop()`，同样问题在 `RemoveJob:683`、`EnableJob:719`、`UpdateJob:800`）
- 代码：
  ```go
  if cs.running {
      cs.store.Jobs = append(cs.store.Jobs, job)
      cs.saveStore()
      if cs.timer != nil { cs.timer.Stop() }   // 只 Stop，没有 Reset / 唤醒
  }
  ```
  `runLoop`（`scheduler.go:212-235`）只阻塞在 `<-cs.timer.C` 或 `ctx.Done()`，没有唤醒通道。被 Stop 的 Timer 不会再触发 → 循环永久挂起。
- 实测（我写了临时程序跑真调度器）：`Start()` 后 `AddJob` 一个 every-1s 的任务，等 2.5 秒 **执行 0 次**；再 `AddJob` 一次，之后 3 秒仍 **执行 0 次**。
- 后果：agent 用 cron 工具创建任何提醒后，**dream、autocompact、所有定时任务一起静默停摆**，无日志、无报错，直到进程重启。
- 修复：加 `wakeCh chan struct{}`（cap 1），`runLoop` 的 select 里监听它；或在 Stop 后 `Reset(computeDelay())`。
- **已实现**：新增 `wakeCh` + `wake()`（非阻塞投递），`runLoop` 的 select 监听唤醒并停掉旧 timer 重算；
  所有运行期改动（Add/Remove/Enable/Update/RegisterSystemJob）都改为 `cs.wake()`。
  实测：Start 后运行期 AddJob，任务继续按周期执行（新增后 1.5 秒内执行 5 次）。
  回归测试：`TestRuntimeAddJobKeepsSchedulerAlive`、`TestWakeIsNonBlocking`。

### R2. 会话被 consolidator 保存后，一半字段永久丢失 ✅ 已修

- 位置：`internal/memory/manager.go:544-635`（`sessionToMap` / `mapToSession`）
- 代码：
  ```go
  return map[string]interface{}{
      "id": sess.ID, "messages": messages,
      "metadata": sess.Metadata, "updated_at": sess.UpdatedAt,
  }   // 没有 channel_id / user_id / title / created_at
  ```
  消息只带 `role/content/created_at/tool_calls`，**没有 message.ID、ToolResults**。
  `sessionManagerAdapter.SaveSession`（495-501）把这份残缺结构还原成 Session 后调 `mgr.SaveSession` → `FileStore.Save` **直接覆盖写盘**。
- 后果：
  1. 会话从 `ListSessionsByChannelAndUser` 里消失（channel/user 变空）
  2. `GetSessionWithValidation` 因身份为空走放行分支，**越权校验被绕过**
  3. 标题、创建时间、工具结果、消息 ID 永久丢失
- 触发条件：任何一次会话压缩（见 R3——目前是**每回合**都会触发）。
- 修复：补齐字段，或者让 adapter 直接传 `*session.Session`，彻底去掉 map 往返。
- **已实现**：`sessionToMap`/`mapToSession` 对称补齐 channel_id / user_id / title /
  created_at / 消息 id、channel/user/session_id、tool_results、metadata；
  顺带修正 tool_calls 的 slice 类型（`[]map` vs `[]interface{}` 不匹配导致工具调用也丢）。
  回归测试：`TestSessionMapRoundTrip`、`TestSessionMapKeepsIsError`。

### R3. 每回合都在做「无意义的压缩」 ✅ 已修

- 位置：`internal/memory/consolidator.go:360`（预算）、`:513`（估算）、`internal/memory/manager.go:190`（构造参数）
- 代码与算术：
  ```go
  func (c *Consolidator) inputTokenBudget() int {
      return c.contextWindowTokens - c.maxCompletionTokens - SafetyBufferTokens
  }   // 4096 - 4096 - 1024 = -1024
  ```
  `estimateSessionPromptTokens` 依赖 `buildMessages`，而 `WithBuildMessages` **全仓库无调用** → 恒返回 0。
  于是 `MaybeConsolidateByTokens` 里 `estimated(0) <= budget(-1024)` 恒为 false → 每回合进入归档循环。
- 后果：每回合额外 LLM 调用 + 同一批消息被反复摘要 + 触发 R2 的丢字段写盘。
- 修复：接入真实上下文窗口与 max_tokens；`budget <= 0` 直接返回；接上 `buildMessages`。
- **已实现**：上下文窗口改取 `agent.context_window_tokens`（默认 128000，原来硬编码 4096）；
  `inputTokenBudget()` 下限夹到 0；估算新增两条回退——会话 map 形态（consolidator 实际收到的就是这个）
  与 `*session.Session` 形态，按「4 字符 ≈ 1 token」粗估；预算为 0 时直接返回。
  回归测试：`TestInputTokenBudgetNeverNegative`、`TestEstimateSessionPromptTokensWithoutBuilder`、
  `TestMaybeConsolidateSkipsWhenBudgetZero`、`TestEstimateFromSessionMap`。

---

## 二、P1：功能失效或成本放大

| 编号 | 问题 | 位置 | 验证方式 |
|---|---|---|---|
| R4 ✅ 已修 | `exec_session` 的长任务在工具返回瞬间被 SIGKILL：`sessMgr.Start(ctx, ...)` 传的是工具 ctx，`Execute` 一返回 `defer cancel()` 就生效，而进程是 `exec.CommandContext(ctx,...)` 起的 | `tools/shell.go:141`、`exec_session.go:599` | 代码链路复核 |
| R5 ✅ 已修 | 一次性 shell 执行把输出全读进 `bytes.Buffer`，`max_output_chars` 只对 session 分支生效 → `cat 大文件`/`yes` 可把进程撑爆 | `tools/shell.go:268` | 代码复核（无上限 writer） |
| R6 ✅ 已修 | `MessageTool.restrictToWorkspace` 全仓库只有声明和一处读取、**从不赋值** → 附件路径校验整段是死代码，模型可把 `/etc/passwd` 当附件发出 | `tools/message.go:29,319` | grep 全仓确认无赋值 |
| R7 ✅ 已修 | 飞书 token 刷新判定恒不成立：`now+margin > expiry` 等价于 `margin > ttl`，而 margin 是 `clamp(ttl/10,1m,10m)`，只在剩余 <1 分钟时才为真；ticker 又是 30 分钟一次 | `channel/feishu.go:2635` | 数学推导 + 代码复核 |
| R8 ✅ 已修 | `PUT /api/config` 只替换 `api.Server` 的 cfg 指针；agent/tools/provider 各自持有旧指针，且没有重载方法 → 换 key、关工具全都不生效（返回 200） | `api/handlers.go:954` | 代码复核（`al.cfg` 只赋一次） |
| R9 ✅ 已修 | OpenAI 请求体（完整对话 + 工具输出）在 **Info** 级明文入日志，未过 `Redact` | `provider/openai.go:260` | 代码复核 |
| R10 ✅ 已修 | Anthropic prompt caching 永不启用：`req.CacheEnabled` 全仓库没有赋值，`config.PromptCacheEnabled` 只被读来展示 | `provider/anthropic.go:261`、`agent/loop.go` 构造 ChatRequest 处 | grep 确认无赋值 |
| R11 ✅ 已修 | dream 永远不会修改记忆文件：phase2 提示词说能读写文件，但 `BuildDreamTools` 只被测试引用、`ChatRequest` 不带 tools；却仍推进 cursor 并压缩历史 → 这段对话既没进记忆也从上下文消失 | `memory/dream.go:284`、`manager.go:444` | 代码复核 |
| R12 ✅ 已修 | autocompact 用**回合级 ctx** 起后台归档，回合结束即取消 → 归档必失败；`if err != nil { return }` 无日志，且 Archive 失败后仍截断+保存会话 | `memory/autocompact.go:188`、`agent/loop.go:1243` | 代码链路复核 |
| R13 ✅ 已修 | `git show -- <rev>:<path>` 的 `--` 让 revision 变成 pathspec → **返回空内容且退出码 0**；`git rev-parse -- HEAD^` 输出 `--`/`HEAD^` 而非 SHA | `memory/git_store.go:379,363` | **实测**（临时仓库） |
| R14 ✅ 已修 | 历史文件读用 `bufio.Scanner`（默认 64KB 单行上限）且**不检查 `Err()`**；64000 字符的条目编码后可达 72KB → 读到一半静默停止，`CompactHistory` 用不完整结果重写会删掉后半段 | `memory/memory_store.go:380` | **实测**（JSON 编码 72,070 字节） |
| R15 ✅ 已修 | `find_files` 的 `head_limit` 默认值不可达：缺省字段就是 0，而 `== 0` 被解释成"不限制"，`defaultFileHeadLimit` 永远用不上 | `tools/search.go:154` | 代码复核 |

---

## 三、P2：可靠性隐患（均已复核代码）

| 编号 | 问题 | 位置 |
|---|---|---|
| R16 ✅ 已修 | WS `handleMessage(ctx,…)` 从不使用 ctx；各 handler 自建 Background ctx → `ws_message_timeout` 无法中断 agent，超时后同连接仍可并发处理 | `api/server.go:805`、`api/handlers.go:112,155` |
| R17 ✅ 已修 | 限流器整数除法丢掉亚令牌零头却推进 `lastSeen` → 每 900ms 请求一次会让该 IP 永久 429（rate=60/min 时） | `api/server.go:128` |
| R18 ✅ 已修 | fallback 全失败时返回 `nil` error + `FinishReason:"error"` → agent 把这句英文诊断当成正常回答落盘并展示 | `provider/fallback.go:404` |
| R19 | 飞书处理失败提示复用**已到期**的 ctx 且 `_ =` 丢弃错误 → 用户既收不到回复也收不到失败提示 | `channel/feishu.go:1425` |
| R20 | `replyMessage` 的 `replyInThread` 参数从未使用 → 话题隔离下回复落到主楼，话题外成员看到串台 | `channel/feishu.go:2065` |
| R21 | 附件下载 `io.LimitReader(10MB)` 截断无检测 → 半截文件当完整文件落盘（表现为"文件解析失败"） | `channel/feishu.go:1847` |
| R22 | 入站图片/语音/文件永久写在 `media/feishu`，全仓无清理逻辑 → 磁盘线性增长 | `channel/feishu.go:1815` |
| R23 | exec session 只按 idle 30 分钟回收，已退出但未 poll 的不回收 → 连续 8 次后"maximum exec sessions reached" | `tools/exec_session.go:516` |
| R24 | session 进程 kill 只杀 `bash` 本身，脚本拉起的子进程成孤儿继续占端口 | `tools/exec_session.go:253` |
| R25 | cron 工具 `if !input.Deliver { deliver = true }` → 恒为 true，模型无法关闭投递 | `tools/cron.go:222` |
| R26 | 定时任务投递错误被 `_ =` 丢弃 → 消息没发出去却记为 `RunStatusOK` | `cron/integration.go:144` |
| R27 | `logger.rotate_daily` 时 `max_size/max_backups/max_age` 全部失效（只在 lumberjack 分支生效）→ 日志无上限 | `logger/logger.go:145` |
| R28 | `/api/auth/*` 三个端点未过中间件 → 登录接口无限流，可被用于灌日志/猜密钥 | `api/server.go:277` |
| R29 | `allowed_paths` 相对项未按 HomeDir 解析 → 默认模板的 `["workspace"]` 会被解析成 `$CWD/workspace`，凭空多一个可写根 | `security/command_paths.go:275` |

---

## 四、依赖安全（前三轮漏掉的维度）✅ 已修

`govulncheck ./...` 结果：**38 项**，你的代码确实会调用到。

| 来源 | 数量 | 修复版本 |
|---|---|---|
| Go 标准库（net/http、crypto/x509、crypto/tls、net/url…） | 33 | Go **1.25.3+**（部分需 1.26.x） |
| `golang.org/x/net v0.57.0` | 5 | **v0.60.0** |

当前 `go.mod` 是 `go 1.25.0`，本机 toolchain 也是 1.25.0。**升级 Go + x/net 即可消除**。

- **已实现**：`go 1.26.10`（toolchain 自动下载，修复全部标准库漏洞）、
  `golang.org/x/net v0.60.0`、`github.com/gorilla/websocket v1.5.3`。
  `govulncheck ./...` 结果从 **38 项 → No vulnerabilities found**。
  CI 的 govulncheck 由「信息性」改为**阻塞任务**。

---

## 五、工程化：测试覆盖极低

`go test -cover` 实测：

| 包 | 覆盖率 |
|---|---|
| api | **5.7%** |
| memory | **7.5%** |
| agent | **8.2%** |
| channel | **8.3%** |
| tools | 14.4% |
| subagent | 7.2% |
| cron | 26.7% |
| session | 26.0% |
| security | 35.4% |
| config | 47.7% |
| bus | 52.1% |
| skills | 60.2% |

意味着：前三轮修的 20+ 处问题里，多数只有"针对性用例"，**没有覆盖到主流程**。R2/R3 这种"每回合触发"的问题能被漏到现在，就是因为没有一条"跑一轮对话"的测试。

---

## 六、修复顺序建议

排序原则：**先修会让功能整体失效的 → 再修丢数据的 → 再修白花钱的 → 最后补边界与工程化**。

### 第 1 批：R1 + R2 + R3（P0，1 天）

这三条互相纠缠，必须一起修：

1. R1 加唤醒通道，让 cron 调度器在运行期修改后仍continue工作
2. R2 补齐 `sessionToMap/mapToSession` 字段（或去掉 map 往返）
3. R3 接真实上下文窗口、`budget<=0` 直接返回、接上 `buildMessages`

验收：`Start()` → 运行期 `AddJob` → 任务能按周期执行；连续 10 轮对话后会话的 channel/user/title/消息 ID 全在；日志里不再每回合出现压缩调用。

### 第 2 批：R4 + R6 + R7（P1，半天）

- R4 exec session 用 `context.WithoutCancel(ctx)`，超时交给 session 自己管
- R6 message 工具由配置驱动 `restrictToWorkspace`，两条分支都校验路径
- R7 token 刷新改为「发送前检查 `now > expiry - margin`」+ ticker 降到 1~5 分钟

这三条分别对应：长任务跑不了、附件能读任意文件、机器人会静默失联半小时。

### 第 3 批：R8 + R9 + R10 + R18（P1，半天）

- R8 配置热更新要么真正生效，要么明确返回「需重启」
- R9 请求体日志降到 Debug + 脱敏 + 截断
- R10 `CacheEnabled` 接上配置（省钱）
- R18 fallback 全失败返回真 error

### 第 4 批：R11 + R12 + R13 + R14（P1，1 天，记忆系统）

dream 没真正写文件、autocompact 用错 ctx、git `--` 误用、历史文件 64KB 截断——这四条合起来说明**记忆系统目前是"看起来在工作"**。建议一并重做并补一条端到端测试（写入 → dream → 读回）。

### 第 5 批：R5 + R15 + R16 + R17（半天）

输出与输入的上限、WS ctx 透传、限流器余量。

### 第 6 批：R19–R29（P2，1~2 天）

按接触面从高到低：飞书发送链路（R19/R20/R21）→ exec session 生命周期（R23/R24）→ 日志与限流（R27/R28）→ 路径解析（R29）。

### 第 7 批：依赖升级 + 测试补齐（半天 + 持续）

- `go get golang.org/x/net@v0.60.0`，Go 升到 1.25.3+，CI 里把 govulncheck 改为阻塞
- 优先给 `agent`（8.2%）、`api`（5.7%）、`memory`（7.5%）补主流程测试

---

## 七、明确「不值得做」

| 想法 | 为什么不 |
|---|---|
| 把 sessionToMap 的 map 往返"修好字段" | 不如直接去掉 map 往返，接口换成结构化类型——修字段只是把坑填平，坑还在 |
| 给 dream 做完整文件编辑沙箱 | 先用现成的 `IsPathAllowedForTool` 就够，dream 只在 workspace 内改文件 |
| 引入分布式锁替代进程内 session 锁 | 单机单进程，收益为零 |
| 把 TUI 撤掉 | 它是 `--tui` 的可选模式，不影响服务端；但要明确它不是主路径 |
| 一次性把覆盖率提到 60% | 先给三个核心包补主流程，其余按改动驱动 |

---

## 八、本轮与前几轮的关系

| 轮次 | 方式 | 发现 |
|---|---|---|
| 第一轮 | 抽查 + 机械检查 | 17 项（崩溃/静默失败/硬编码） |
| 第二轮 | 语义专项 | 7 项（截断/cron 并发/DeepCopy） |
| 第三轮 | 投递专项 | 6 项 |
| **第四轮** | **全量通读 + 依赖扫描 + 覆盖率** | **29 项 + 依赖 38 漏洞 + 覆盖率数据** |

第四轮之所以能挖到 P0，是因为**第一次逐文件读完**，而不是按关键词抽查——R1（cron 死锁）和 R3（预算为负）都属于"读逻辑才看得见"的问题。

---

## 九、局限

1. 覆盖率为行覆盖率，不代表关键路径被覆盖
2. 未做真实压测与长时间运行观察（R22 磁盘增长、R16 并发处理均属推算）
3. 未验证飞书官方对消息体/附件大小的精确限制（R21 只确认"没有检测"）
4. `go list -m -u` 显示 43 个直接依赖中多数落后于最新版，本轮只处理安全相关的 x/net，未做全面升级
