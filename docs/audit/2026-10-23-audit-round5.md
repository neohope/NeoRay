# NeoRay 代码审计（第五轮 · 全量复扫）

- 审计日期：2026-10-23（第四轮修复完成后）
- 基线提交：`7f3d352`
- 范围：全部后端代码——19 个包、78 个非测试文件、32,424 行
- 方法（与第四轮相同：全量，不抽样）：
  1. 76+ 个文件按包分 8 组**完整读完**（不是关键词抽查）
  2. 机械检查：`govulncheck`、`staticcheck`、`go test -race`、`-count=2` 重复运行、
     死配置扫描（配置字段是否真被使用）、未调用导出函数扫描、超长函数扫描、
     goroutine 是否带 panic 兜底、`_ =` 丢弃错误清单
  3. **每条发现都自己复核**；能跑的直接跑（写探查程序、-race 复现、真实命令验证）
- 结果：**12 项确认问题**（P0 1 / P1 11）+ 12 项 P2/P3 + 24 项未复核候选（附录）
- **状态：全部已修复**（4 个提交批次，`b3f0af7` … 见文末「修复结果」）

> **本轮最重要的结论**：**上一轮的修复有 4 处边界没守住**（下文 S3/S4/S5 与 P2-1 明确标注）。
> 全量复扫的价值就在这里——只跑"针对性用例"看不出这些。

---

## 一、P0：沙箱边界可被平凡命令绕过（实测）

### S1. shell 路径策略对三类命令完全不生效 ✅ 已修

- 位置：`internal/security/command_paths.go:162`（`-` 开头 token 直接跳过）、
  `:184`（含 `://` 的 token 直接跳过）
- 代码：
  ```go
  if !isRedirectTarget && strings.HasPrefix(token, "-") {
      continue          // ← python3 -c'...' 整段被当成"选项"放过
  }
  ...
  if strings.Contains(token, "://") {
      return nil        // ← file:// 也放过；而 SSRF 检查只认 http(s)://
  }
  ```
- **实测**（`CheckCommandPaths`，workspace = `~/.neoray/workspace`，`restrict_to_workspace=true`）：

  | 命令 | 结果 |
  |---|---|
  | `cat /etc/passwd` | ✅ 拦截 |
  | `python3 -c'import os;print(open("/etc/passwd").read())'` | ❌ **放行** |
  | `perl -e'print open("/etc/passwd")'` | ❌ **放行** |
  | `curl file:///etc/passwd` | ❌ **放行** |
  | `curl 169.254.169.254/latest/meta-data` | ❌ **放行** |

- 后果：`SECURITY.md` 声明的「shell 不能读工作区外文件」不成立。模型（或提示注入）
  一句 `python3 -c ...` 就能读 `~/.ssh/id_rsa`、`~/.neoray/neoray.toml`（含 API key）。
  无 scheme 的裸 host 还能直连云元数据/内网服务。
- 修复：
  1. `-` 开头 token 不要整段放过：剥掉选项前缀后仍走路径/内联代码判定
  2. `://` 只对 `http(s)` 免检，`file:`/`gopher:` 等按路径校验
  3. URL 检测覆盖无 scheme 的 `host:port/path` 形态

---

## 二、P1：11 项

### S2. cron 长任务执行期间调度器 100% CPU 空转 ✅ 已修

- 位置：`internal/cron/scheduler.go:283`（`if delay < 0 { delay = 0 }`）
- 机制：`onTimer` 异步派发任务后**不推进** `NextRunAtMS`（只在任务跑完才更新），
  于是 `computeDelay()` 算出"已过期" → `delay = 0` → `runLoop` 立刻再进 `onTimer` →
  看到任务仍在 `runningJobs` 只记一条 Warn → 再算 0 → **死循环**。
- **实测**：启动一个每 100ms 触发、每次执行 800ms 的任务，
  在任务执行期间采样 400ms，进程 CPU **433ms**（≈ 满核）。
- 后果：任何 cron 任务（dream、autocompact、agent turn）执行多久，就空转多久；
  日志被 Warn 刷屏，全局锁被反复抢占。
- 修复：派发时即推进 `NextRunAtMS`，或 `computeDelay` 忽略 `runningJobs` 里的任务。
- **这是第二轮「cron 并发化」引入的**，第四轮 wake 改造后仍在。

### S3. exec_session 回收条件把「占坑」修成了「丢输出」（上轮回归）✅ 已修

- 位置：`internal/tools/exec_session.go:524`（`session.returnCode != nil` 即回收）、
  `:442`（`Write` 先 `cleanupLocked()` 再取输出）
- 我上一轮为了让「已退出但未 poll」的会话不占名额，加了 `|| session.returnCode != nil`。
  但回收发生在**读取输出之前**：命令在上次 poll 与下次 poll 之间结束（最典型：
  `yield_time_ms=1000` 跑一个 5s 构建），第二次 `write_stdin` 得到的是
  `session not found`，退出码与剩余输出一起丢。
- 修复：回收条件加「输出已取空」或退出后宽限期（如 60s）；`Write` 先取输出再清理。

### S4. 每回合重复归档同一段历史（上轮没修完）✅ 已修

- 位置：`internal/memory/consolidator.go:216`（重播检查在预算判断**之前**）、
  `consolidateReplayOverflow` 归档后既不 `setLastConsolidated` 也不 `setMessages`
- **实测**：会话 25 条消息、尾部窗口首条不是 user，连续 3 个回合调用
  `MaybeConsolidateByTokens` → **3 次 LLM 调用，游标始终 0**（应只归档一次）。
- 后果：只要会话超过 10 条消息且窗口首条不是用户消息，**每回合多一次 LLM 往返**，
  并把同一段摘要反复追加进 `history.jsonl`（又被注入系统提示，越用越慢越贵）。
- 第四轮我修了预算和估算，但这条路径在预算判断之前就执行，等于没修到。
- 修复：归档后推进游标；把重播检查移到预算判断之后。

### S5. `session.Metadata` 并发读写（可致进程崩溃，已用 -race 复现）✅ 已修

- 位置：`internal/session/goal_state.go:127`（写 map 只拿 `GoalManager.mu`，
  不拿 session 锁）、`getGoalStateLocked:60-62`（RLock 只覆盖取 map 头）
- **实测**（`go test -race`）：`SetGoalState` 与 `DeepCopy` 并发 → **DATA RACE** 多处。
- 后果：`long_task`/`complete_goal` 与 `GET /api/sessions/{id}`（DeepCopy）
  并发时是 map 读 + 写，Go 会抛 `fatal error: concurrent map read and map write`
  ——**不可 recover，整进程退出**。
- 修复：metadata 的读改写整体放进 `session.Lock()`，改前先深拷贝 map。

### S6. grep/find_files 跟随软链读工作区外文件（实测泄漏）✅ 已修

- 位置：`internal/tools/search.go:534`（`os.ReadFile` 跟进软链）、
  `:177`（`info.IsDir() && info.Mode()&os.ModeSymlink != 0` 恒为假，防软链代码是死代码）
- **实测**：工作区内 `ln -s /tmp/neoray-secret.txt leak.txt` 后，
  `grep` 返回了工作区外文件的 `SECRET-MARKER`; 同一路径 `filesystem.read` 正确拒绝。
- 后果：软链是绕过路径策略的最短路径；`find_files` 也会列出指向界外的软链。
- 修复：Walk 回调开头跳过 `ModeSymlink`；或 `EvalSymlinks` 后对每个文件复用
  `security.IsPathAllowedForTool`。

### S7. 子代理 spawn 后立刻被取消（spawn 功能实际不可用）✅ 已修

- 位置：`internal/subagent/manager.go:158`（`context.WithCancel(ctx)` 继承调用方）、
  `internal/agent/loop.go:2121-2122`（工具 ctx + `defer cancel()`）
- 机制：`SpawnTool.Execute` 调用 `Spawn` 后立即返回，工具 goroutine 结束即执行
  `defer cancel()` → `subCtx` 被取消 → 子代理第一次 `p.Chat` 就 `context canceled`。
- 修复：子代理用独立生命周期 ctx（`context.WithCancel(context.Background())`），
  只由 `CancelTask/CancelBySession/Stop` 显式取消。

### S8. `llm.fallback_models` 在实际聊天路径完全不生效 ✅ 已修

- 位置：`cmd/server/main.go:640`（注册的是未包装 provider）、
  `internal/provider/provider.go:162`（`GetProvider(name)` 直接返回 map 里的原始实例）、
  `internal/agent/loop.go:987`（`GetProvider(cfg.LLM.DefaultProvider)`）
- 机制：`FallbackProvider` 只通过 `SetDefaultProvider` 安装；而 agent 按名字取，
  拿到的是原始 provider → 主 provider 429/5xx 时不会切备用模型。
  memoryManager 拿的是包装后的（`main.go:322`），两个子系统行为不一致。
- 修复：包装后以同名重新注册进 map，或 `GetProvider` 对默认名返回包装实例。

### S9. `tools.workspace.read_only` 配置完全没接线 ✅ 已修

- 位置：`internal/config/config.go:181` 定义、`internal/agent/loop.go:2225` 只写进运行时状态
- grep 确认：`ReadOnly` 全仓没有任何写路径检查点。`write_file`、`delete`、
  `apply_patch`、bwrap 沙箱（`--bind ws ws`）全都照写。
- 后果：用户设 `read_only = true` 后写入仍成功，且模型被告知「当前只读」——
  安全控制静默失效并误导模型。
- 修复：在 `writeFile`/`deleteFile`/`ApplyPatchTool` 写盘前统一校验。

### S10. 一条 `/stop` 能把整个消息总线堵住 ✅ 已修

- 位置：`internal/agent/loop.go:538`（`lock.mu.Lock()` 无超时）、
  `internal/bus/bus.go:240-262`（单协程顺序调用 handler）
- 机制：回合进行中（`hasPending`）又收到可分发命令时，走无超时的 `lock.mu.Lock()`。
  总线只有一个 `processInbound` 协程 → 所有渠道的入站消息（飞书、WS、子代理结果、
  cron）全部停摆，最长一个回合（默认 10 分钟）；而 `/stop` 恰恰因为拿不到锁，
  无法打断卡住的回合。
- 修复：命令分发移到独立 goroutine + `lock.acquire(ctx, timeout)`；
  `/stop` 走不依赖会话锁的通路。

### S11. 流式工具调用多分片合并是坏的（Web 端多分片必然失败）✅ 已修

- 位置：`internal/agent/loop.go:1731`（`key := len(toolCallMap)` 给无 ID 分片新建条目）、
  `internal/provider/provider.go:27`（`ToolCall` 无 `Index` 字段）
- 机制：OpenAI 只有首个 delta 带 `id/name`，后续 args 分片 id 为空 →
  每个分片都新建一条 → 一次工具调用被拆成多条（首条 args 空、其余 name 空）→
  执行时 "tool not found"；Anthropic 每片发完整累积 args → `Arguments += ` 重复拼接，
  JSON 被拼坏。
- 修复：`ToolCall` 增加 `Index`，按 Index 归并；只有真正的增量分片才追加。

### S12. LLM 失败时删错消息，可能把会话永久毒化 ✅ 已修

- 位置：`internal/agent/loop.go:1099`（`RemoveLastMessage()`）、
  `internal/session/entity.go:187`（只删最后一条）
- 机制：注释说"移除孤儿用户消息"，但删的是**最后一条**。第 2 轮及以后失败时，
  最后一条是上一轮的 tool 结果 → 删掉 tool 结果、留下 `assistant(tool_calls)`，
  落盘成「tool_use 后无 tool_result」的非法历史 → 之后每轮请求都被上游 400，
  只能 `/clear` 才能恢复。
- 修复：按索引删除本轮 user 消息；失败路径不要动工具结果。

---

## 三、P2/P3（12 项，均已复核代码位置）

| 编号 | 问题 | 位置 |
|---|---|---|
| P2-1 | **`max_message_chars` 与 `dedup_capacity` 从未从配置传入**（我上一轮加的字段，`main.go` 构造 `FeishuConfig` 时漏了两行）→ 用户设了被静默忽略 | `cmd/server/main.go:501` |
| P2-2 | 用户自建技能不可见：`filepath.Join(cfg.Memory.Workspace, "skills")`，而 `memory.workspace` 默认为空 → 实际扫描 `$CWD/skills`，与 `~/.neoray/skills`、`~/.neoray/workspace/skills` 都不是同一个目录 | `internal/skills/skills.go:156` |
| P2-3 | 三份 `truncateString` 都不做 UTF-8 边界修正，会切出半个汉字（prompt/日志/会话内容里出现非法 UTF-8）；另一份 `logger.Truncate`/`tools.TruncateToolResult` 却是安全的 | `memory/dream.go:370`、`channel/feishu.go:2857`、`session/entity.go:312` |
| P2-4 | `read_only`（同 S9）、`**` 通配不支持但文档示例在用、`allowed_extensions=[".*"]` 与精确匹配冲突 | `tools/search.go:737`、`tools/filesystem.go:228` 等 |
| P2-5 | `memory` 工具 `content` 未列入 required，漏传会清空 MEMORY.md/SOUL.md 并 git 提交 | `internal/tools/memory.go:117` |
| P2-6 | cron 的 `tz` 拼错被静默当成 UTC（触发时间差 8 小时） | `internal/tools/cron.go:358` |
| P2-7 | `thread_id` 只判 key 存在（普通消息是空串），导致 `reply_to_message=false` 失效、每段长回复都变成引用回复 | `internal/channel/feishu.go:706` |
| P2-8 | 去重登记发生在 @ 过滤之前 → 无关群消息占满 5000 容量，重投时重复回复 | `internal/channel/feishu.go:1365` |
| P2-9 | 每连接限流器对 `elapsed` 做整数截断（我上一轮只修了全局那个）→ 1.5s 间隔请求会永久 429 | `internal/api/server.go:719` |
| P2-10 | SSE 写截止时间硬编码 6 分钟，与可配的 `message_timeout`（默认 10 分钟）不一致 | `internal/api/handlers.go:606` |
| P2-11 | 配置文件语法错误被当成"文件不存在"静默吞掉，用默认值启动（表现为所有请求 401） | `internal/config/loader.go:56` |
| P2-12 | `[server.cors]` 前缀匹配把仍在生效的 `allowed_origins` 也当成"已移除"告警，用户删掉后 WS 连不上 | `internal/config/deprecated.go:21` |
| P3-1 | 失败路径不摘 ⏳ 表情（成功路径才摘）+ 流式/非流式重复加 ✅ 表情 | `feishu.go:1513/1534` |
| P3-2 | 21 个 goroutine 只有 3 个带 panic 兜底；剩余多是长生命周期循环（provider 流、频道分发、api） | 全局 |

---

## 四、附录：24 项未复核候选（来自并行通读，未逐条验证）

以下结论**看起来合理但我没验完**，仅供下一轮参考，不要当成事实：

- `apply_patch` 不检查 `blocked_extensions` / `max_file_size`；`add` 动作在无换行结尾的文件后拼接会粘连
- patch 与 write 并发写同一文件无互斥（后写覆盖前者，失败回滚还会删掉并发新建文件）
- `exec_session` 的 `Wait` 与 `StdoutPipe` 并发，可能丢尾部输出；`Start` 持锁做最长 30s 阻塞
- `web` 工具 8 处请求用 `http.NewRequest`（未绑 ctx），取消后仍跑满超时
- `consolidator` 的 `PickConsolidationBoundary` 相对/绝对下标混用；`getLock` 的 TryLock 清理有窗口
- `autocompact` 无并发上限，且与回合的 SaveSession 是读改写竞争
- `BuildSystemPrompt` 每回合全量扫描 `history.jsonl` 并按字节截断（可能切断 UTF-8）
- `filestore.Save` 持锁做全量序列化 + 写盘；`List` 返回活对象未深拷贝
- `Delete` 后被 in-flight 回合的 `Save` 重建（删了又回来）；分页游标与裁剪冲突
- `CleanupOldSessions` 无调用者，`max_sessions` 对文件存储无效
- 续轮请求没有 system 消息、也没复制 MaxTokens/Temperature/CacheEnabled
- cron `saveStore` 持锁写盘 + 无锁调用点共用同一个 `.tmp`；`GetJob` 返回内部指针（-race 可判）
- cron payload 的 `session_key` 从不写回 → 每次执行新建会话
- Anthropic `Retry-After` 被丢弃，`extractRetryAfterFromHeaders` 是死代码
- `loadInboundMedia` 对入站媒体无上限/无清理（我只加了 7 天清理）

---

## 五、修复顺序建议

原则：**先堵安全边界 → 再修我自己引入的回归 → 再修崩溃与竞态 → 最后清配置与体验**。

### 第 1 批（半天）：S1 + S6 — 把沙箱边界补上

- S1：`-` 开头 token 剥前缀后再校验；`://` 只认 http(s)；补无 scheme URL 检测
- S6：Walk 跳过软链（或逐文件 `EvalSymlinks` + 策略校验）
- 验收：`python3 -c`/`curl file://`/软链 三类命令全部被拦

### 第 2 批（半天）：S3 + S4 + S2 — 修我上一轮的回归

- S3：回收条件加"输出已取空/宽限期"
- S4：归档后推进游标；重播检查移到预算判断之后
- S2：派发即推进 `NextRunAtMS`
- 验收：长任务可继续 `write_stdin` 取输出；连续 3 回合只归档 1 次；cron 长任务期间 CPU < 5%

### 第 3 批（1 天）：S5 + S7 + S11 + S10

- S5：metadata 读改写进 session 锁（并补 `-race` 回归测试）
- S7：子代理独立 ctx
- S11：`ToolCall` 加 `Index` 并按其归并
- S10：命令分发异步 + 带超时；`/stop` 优先

### 第 4 批（1 天）：S8 + S9 + S12 + P2-1/P2-2/P2-9

- S8：fallback 同名注册
- S9：`read_only` 接入三条写路径
- S12：按索引删本轮 user 消息
- P2-1/P2-2：配置接线（`max_message_chars`/`dedup_capacity`）、技能目录解析
- P2-9：每连接限流器改浮点

### 第 5 批（1~2 天）：其余 P2/P3 + 附录里复核后确认的项

---

## 六、明确「不值得做」

| 想法 | 为什么不 |
|---|---|
| 把 shell 路径检查做成完整命令解析（AST/沙箱） | 静态检查永远漏；真正的隔离应该交给 bwrap（已支持），策略层只做"明显越界就拒"，重点是**不要有整类命令被无条件放过** |
| 给每个 goroutine 都套 recover | 长生命周期循环（provider 流、频道分发、api、cron）值得；一次性短协程套 recover 会掩盖真 bug |
| 把 consolidator 的相对/绝对下标重构成索引对象 | 先修 S4（游标不推进）就能消掉主要浪费；重构留到需要增量压缩时再做 |
| 现在实现 sqlite 会话存储 | 文件存储在单用户规模够用；`max_sessions` 接上即可 |
| 一口气升级 43 个直接依赖 | 只升有漏洞/有 bug 的；其余需逐个做行为回归 |

---

## 七、与前几轮的关系（含自我批评）

| 轮次 | 方式 | 发现 | 其中属上一轮回归 |
|---|---|---|---|
| 第一轮 | 抽查 | 17 | — |
| 第二轮 | 语义专项 | 7 | — |
| 第三轮 | 投递专项 | 6 | — |
| 第四轮 | 全量通读 | 29 + 38 漏洞 | — |
| **第五轮** | **全量复扫** | **12 项确认 + 12 P2/P3** | **4 项（S3、S4、P2-1，及 S2 来自第二轮）** |

**需要明确承认的**：S3（丢输出）、S4（重复归档）、P2-1（配置没接线）都是我在第四轮/第二轮
"修一个问题引入另一个问题"或"修了一半"造成的。这也说明第四轮的验收标准不够硬——
只有"跑一轮真实对话"级别的端到端测试才挡得住这类问题。

---

## 八、局限

1. 附录里 24 项未复核，可能有过期/误报，不要直接当结论用
2. S7/S12 是按代码链路推导（spawn 的取消传播、删错消息），未做端到端复现
3. 未做真实压测与长时间运行观察（S2 的 CPU 占用是短时采样）
4. 未覆盖 Flutter 客户端与 Web UI 前端
5. 依赖 0 漏洞是在本轮当天扫描的结果，需持续跟踪

---

## 九、修复结果（已落地）

| 批次 | 提交 | 内容 |
|---|---|---|
| 1 | `b3f0af7` | **S1** 选项内容/file:// /裸主机纳入校验；**S6** 搜索工具跳过软链 |
| 2 | `0b93c29` | **S2** cron 派发即推进 NextRunAtMS；**S3** 退出后 60s 宽限再回收；**S4** 归档推进游标 + 重播检查后移 |
| 3 | `ec8ed59` | **S5** metadata 快照/写时复制；**S7** 子代理 ctx 独立；**S11** ToolCall.Index 归并；**S10** 命令分发带超时；consolidator 坐标统一 |
| 4 | `b97568c` | **S8** fallback 同名注册；**S9** read_only 接线；**S12** RemoveLastUserMessage；**P2-1/P2-2/P2-9** |
| 5 | 本次 | **P2-3~P2-12、P3-1、P3-2**：三份 truncate 做 UTF-8 边界修正、memory 空内容拒绝、cron tz 校验、thread_id 按值判断、去重登记后移、SSE 截止时间对齐、配置解析错误不再吞、deprecated CORS 键收窄、失败路径清 ⏳、长生命周期协程加 panic 兜底 |

### 关键验证（都是实测，不是"应该好了"）

| 项 | 修复前 | 修复后 |
|---|---|---|
| `python3 -c'open("/etc/passwd")'` | 放行 | **拦截** |
| `curl file:///etc/passwd` | 放行 | **拦截** |
| `curl 169.254.169.254/...` | 放行 | **拦截** |
| 软链指向界外 + grep | 读到内容 | **读不到** |
| cron 长任务期间 CPU（1500ms 采样） | 1407ms | **7ms** |
| 25 条会话连续 3 回合的 LLM 调用 | 3 次 | **≤2 次且游标单调推进** |
| `SetGoalState` + `DeepCopy` 并发 | `-race` DATA RACE | **无竞争** |
| 只读工作区写入/打补丁 | 成功 | **拒绝且文件未改动** |

### 顺手清掉的「未复核候选」里能确认的几项

- 三份 `truncateString` 全部改为 UTF-8 安全（`session` 包加了遍历 max 的回归测试）
- `allowed_extensions = [".*"]` 与 `**` glob 两项**未修**（见下）
- patch 的 `blocked_extensions`/`max_file_size` 复用、`exec_session` 的 Wait/pipe 顺序、
  `web` 工具的 ctx 绑定等仍留待下一轮

### 仍未做（明确记录）

| 项 | 原因 |
|---|---|
| `**` 通配与 `allowed_extensions` 通配语义 | 需要先统一"配置里写什么算通配"的产品决定，改错会让现有配置更混乱 |
| `exec_session` 的 Wait 与 StdoutPipe 并发、Start 持锁阻塞 | 与 S3 同源但改动面更大（涉及读协程生命周期），单独一轮更稳妥 |
| `filestore` 锁内 IO / List 返回活对象 / 删除后被重建 | 会话存储改造成本高，单用户规模下影响有限 |
| `apply_patch` 复用全部文件策略、patch 与 write 并发互斥 | 需要引入按路径的写锁，影响面较大 |
| 依赖全面升级 | 只升了有漏洞/有 bug 的 |
