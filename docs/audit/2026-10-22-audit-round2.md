# NeoRay 代码审计（第二round）

- 审计日期：2026-10-22（第二轮，第一轮修复完成后）
- 基线提交：`a82ef7e`
- 范围：`neoray/` 后端 76 个非测试文件（约 3.08 万行）
- 前置状态：第一轮 17 项已修 16 项；`gofmt` / `vet` / `-race` / `staticcheck` 全绿

> 第一轮的机器可检出问题（panic 兜底、静默错误、硬编码超时、死代码）已清零。
> 本轮转向**语义层**：配置是否真的作用到行为、并发模型是否成立、失败是否会重试。

---

## 一、本轮新发现（按影响排序）

### P1｜会让功能悄悄失效或放大成本

#### N1. 主循环的工具结果完全不做长度限制（配置是摆设） ✅ 已修（2026-10-22）

- 位置：`internal/agent/loop.go:2149`（`"content": string(result)`）
- 说法：`agent.max_tool_result_chars` 存在，但**只被塞进 `RuntimeState` 供 reflection 展示**
  （`loop.go:2178`），从未参与截断；`subagent` 的 `WithMaxToolResultChars` 同样是只赋值不使用。
- 证据：全仓搜索只剩两处长度判断，都是**日志截断 500 字符**：
  ```
  internal/agent/loop.go:2135  if len(resultStr) > 500   // 日志
  internal/tools/web.go:508    if len(content) > 500     // 日志
  ```
- 影响：一次 `cat 大文件` / `grep -rn` across 大仓库，会把完整输出塞进下一轮 LLM 请求。
  后果是 token 成本暴涨，或直接撞 context 上限返回 400，而用户只看到"AI 服务连接失败"。
- 修复要点：在把 result 写入 tool 响应前统一截断（保留头尾 + 中间省略提示），
  阈值取 `agent.max_tool_result_chars`（默认 16000，与 subagent 的既有默认一致）。
- **已实现**：`tools.TruncateToolResult`（头 2/3 + 尾部 1/3 + 截断提示，UTF-8 安全），
  主循环与子代理共用；配置默认值 16000，示例配置已补该项。
  实测 330,000 字符 → 16,000 字符（压缩 95.2%），头尾与提示均保留。

#### N2. cron 任务是串行执行：一个长任务阻塞所有定时任务

- 位置：`internal/cron/scheduler.go:278`（`for _, job := range dueJobs { cs.executeJob(job) }`）
- 说法：`executeJob` 直接调 `onJob`（agent 一整轮，可能数分钟）。同一循环里其它到期任务全部等待；
  而 `Start/Stop` 用 `cs.wg.Wait()`，`Stop()` 也会被长任务拖住。
- 影响：dream（每小时）+ 用户提醒 + autocompact 共用一条执行队列，dream 跑得久时提醒会迟到。
- 修复要点：每个 job 起独立 goroutine + 同一 job 的重入保护（running 标记）；
  `Stop()` 增加超时，避免无限等待。

#### N3. `Session.DeepCopy()` 是浅拷贝，嵌套引用被共享

- 位置：`internal/session/entity.go:209`（`copy(cp.Messages, s.Messages)` 在 224 行）
- 说法：`cp.Messages = make(...); copy(...)` 只复制了 slice 头，
  每个 `Message` 里的 `ToolCalls []ToolCall`、`ToolResults []ToolResult`、`Metadata map[string]any` 仍指向同一内存。
- 影响：`FileStore.Get` / `Save` 都依赖 DeepCopy 隔离并发访问；任何一方修改嵌套 map
  都可能触发 `concurrent map read and map write`（Go 会直接 panic，无法 recover 到一致状态）。
  目前未复现，属**潜在**竞态，但代价是进程崩溃。
- 修复要点：对 Messages 逐条复制嵌套字段（map/slice），或改为只存不可变快照。

### P2｜行为与直觉不符

#### N4. 停机期间错过的周期任务不补跑

- 位置：`internal/cron/scheduler.go:91`（`ScheduleKindEvery` → `next = nowMS + EveryMS`）
- 说法：进程停 3 小时后重启，`every 1h` 的任务从"现在"重新计时，停机期间的 3 次全部跳过。
- 影响：对提醒类任务可以接受；对 dream（记忆整理）意味着长期不运行时记忆会持续堆积。
- 建议：明确为设计取舍并写进文档；或对 `dream-process` 单独支持"启动后立即补跑一次"。

#### N5. 飞书去重表容量 1000，繁忙群聊会过早淘汰

- 位置：`internal/channel/feishu.go:307`（`newOrderedMap(1000)`）
- 说法：LRU 满 1000 条后淘汰最旧；飞书长连接在弱网重连时会重投历史事件。
- 影响：在消息密集的群里，1000 条可能只覆盖几分钟；重投落在淘汰之后 → **重复回复**。
- 建议：容量提到 5000 或改为"按时间窗口"（如保留最近 24h），并与批量落盘配合。

#### N6. cron 任务失败后仍然推进到下一次，不做退避

- 位置：`internal/cron/scheduler.go:360`（无论 `LastStatus` 是 ok 还是 error 都算 next）
- 说法：失败不重试、不降频。一次网络抖动就跳过一整个周期。
- 建议：连续失败 N 次后指数退避（例如 1、2、4 个周期），恢复成功后复位。

### P3｜值得知道但不急

| 编号 | 内容 | 说明 |
|---|---|---|
| N7 | `/api/sessions/{id}` 返回会话全部消息 | 长会话单次响应可能数百 KB；Web UI 首次加载变慢。可加 `?limit=` |
| N8 | 子代理结果以 system 消息注入主会话并触发一轮 LLM | 设计取舍：多一次调用换自然语言转述，成本可预期 |
| N9 | `tools/web.go` 抓取结果的 500 字符截断只作用于日志 | 与 N1 同源，修 N1 时一并统一 |

---

## 二、明确「不值得做」的项

避免把精力花在低收益改动上：

| 想法 | 为什么不做 |
|---|---|
| 把 `agent.max_tool_result_chars` 改成仅展示、删掉配置 | 不如直接实现截断（N1），配置语义本来就是对的 |
| 给 cron 加完整分布式调度（错过补跑 + 抢占 + 持久化事务） | 单机单进程场景，收益远低于复杂度；只做 goroutine 隔离即可 |
| 把 `Session.DeepCopy` 换成序列化深拷贝 | 每次读会话都 JSON 编解码，性能代价明显；逐条复制嵌套字段即可 |
| 给 `/api` 加多用户鉴权体系（F15） | 当前单用户单密钥部署，改动面涉及前端约定，收益低（第一轮已决定暂缓） |
| 引入消息队列替换 msgBus | 单进程内 channel 足够；引入外部依赖会显著增加运维成本 |

---

## 三、修复顺序建议

排序原则：**先防"悄悄变贵/悄悄崩"，再改行为，最后做体感优化。**

### 第 1 批：N1 工具结果截断（半天，收益最大）

- 在 `executeToolCalls*` 汇总处统一截断，保留头 2/3 + 尾 1/3，中间标注 `…（已截断 N 字符）`
- 阈值读 `agent.max_tool_result_chars`（默认 16000），并在示例配置里补上这一项
- 补测试：超长结果被截断、短结果不变、截断后仍是合法 JSON 字符串
- 验收：`cat` 一个 5MB 文件后，下一轮请求的 messages 体积明显下降

### 第 2 批：N2 cron 并发化（半天）

- 每个到期任务独立 goroutine，job 级 `running` 标记防重入
- `Stop()` 加 30s 超时，超时后记录仍在跑的任务
- 验收：一个 sleep 60s 的测试任务不阻塞其它到期任务

### 第 3 批：N3 DeepCopy 补深（半天）

- 逐条复制 `ToolCalls` / `ToolResults` / `Metadata`
- 补并发测试：`-race` 下同时 Get + 修改嵌套 map 不报错

### 第 4 批：N5 + N6 飞书与 cron 的可靠性细节（1 天）

- 去重容量提升或改时间窗口
- cron 连续失败退避

### 第 5 批：N4 + N7 文档与体验（按需）

- 文档写明「不补跑」的语义；`dream` 可单独配置启动补跑
- `/api/sessions/{id}` 支持 `limit` 参数

---

## 四、与第一轮的关系

| 类别 | 第一轮 | 第二轮 |
|---|---|---|
| 崩溃风险 | panic 兜底（F1–F5） | DeepCopy 浅拷贝（N3） |
| 静默失败 | 日志缺失（F6–F10） | 截断缺失（N1）、失败仍推进（N6） |
| 硬编码 | 超时/上限（F11–F14） | cron 串行（N2）、去重容量（N5） |
| 工程化 | CI（F17） | 无新增；建议把 N1/N2/N3 的测试纳入 CI |

---

## 五、局限

1. 未覆盖 Flutter 客户端与 Web UI 前端。
2. 未做压测：N2 的阻塞时长、N5 的淘汰速率是推算，未在真实高并发下量化。
3. 未审计第三方依赖缺陷（CI 里 govulncheck 为信息性任务）。
4. 未验证飞书长连接在弱网下的重连退避与事件乱序。
