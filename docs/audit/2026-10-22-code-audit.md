# NeoRay 代码审计报告

- 审计日期：2026-10-22
- 范围：`neoray/` 后端（76 个非测试 Go 文件，约 30,700 行）+ 配置/部署脚本
- 基线提交：`725da3d`
- 方法：静态检索（错误处理 / 并发 / 资源 / 输入校验 / 认证 / 持久化）+ `go vet` + `go test -race` + 用真实配置与真实工具做端到端验证

---

## 一、总体结论

代码整体结构清晰，但存在两类系统性风险：

1. **后台 goroutine 缺少 panic 兜底**——任何一处 panic 都会带走整个进程（飞书断线、定时任务停摆、Web UI 全部失联）。
2. **持久化与外部调用的错误被静默吞掉**——出问题时没有日志，只能靠猜（记忆归档、去重写盘、令牌失效重试）。

本轮已修掉 4 个 P0 与 1 个同类遗漏；其余按优先级列在第四节。

---

## 二、已在本轮修复（含验证）

| ID | 问题 | 位置 | 后果 | 状态 |
|---|---|---|---|---|
| F1 | 飞书事件处理 goroutine 无 `recover` | `internal/channel/feishu.go:1009` | 外部事件触发 panic → 进程退出 | ✅ 已修 |
| F2 | `Server.Stop()` 仍 `close(client.Send)` | `internal/api/server.go:498` | handler 晚到写入 → `send on closed channel` panic | ✅ 已修 |
| F3 | agent 会话 goroutine 无 `recover` | `internal/agent/loop.go:1482` | 单条消息 panic → 整服务退出，前端无结束信号 | ✅ 已修 |
| F4 | cron `onJob` 无 `recover` | `internal/cron/scheduler.go:302` | 定时任务 panic → 调度循环与进程一起挂 | ✅ 已修 |
| F5 | readPump 派生的 handler 无 `recover` | `internal/api/server.go:720` | 同上（Web UI 侧） | ✅ 已修 |

验证：`go vet` 通过；`go test -race ./internal/api ./internal/cron ./internal/channel` 无竞态；`internal/...` 11 个包全通过；新增 `TestStopDoesNotCloseSend`。

---

## 三、待修复问题

### P1 — 会造成数据丢失或静默失败

> 状态更新（2026-10-22 第 2 批已修复）：F6–F10 全部改为「失败即告警/返回错误」，
> 并顺带修掉 dream 无法创建新技能目录的两个缺陷。
>
> 状态更新（2026-10-22 第 3 批已修复）：F11 token 失效自动刷新重试、
> F12 去重表批量落盘 + Stop 刷盘。
>
> 状态更新（2026-10-22 第 4 批已修复）：F13 会话锁带超时（并用「上一条还在处理」
> 提示替代无限排队）、F14 日志脱敏。
>
> 状态更新（2026-10-22 第 5 批已修复）：F16 静态资源软链校验、F17 CI（gofmt /
> vet / test -race / build / staticcheck / govulncheck）。
> F15（身份从请求参数改为认证上下文）按计划**暂不做**：改动面涉及前端约定，
> 而单用户单密钥部署的实际收益有限，留作多用户改造时一并处理。

| ID | 问题 | 位置 | 证据 | 影响 |
|---|---|---|---|---|
| F6 | 记忆归档失败被忽略 | `internal/memory/consolidator.go:188` | `_ = c.store.RawArchive(messages)` | 磁盘满/权限错时历史直接丢，无日志 | ✅ 已修 |
| F7 | 历史压缩失败被忽略 | `internal/memory/dream.go:175` | `_ = d.store.CompactHistory()` | dream 反复做无用功，历史无限增长 | ✅ 已修 |
| F8 | 目录创建失败被忽略 | `internal/memory/dream.go:416` | `_ = os.MkdirAll(...)` | 后续写入全部失败且无提示 | ✅ 已修 |
| F9 | 随机数错误被忽略 | `internal/bus/message.go:172` | `_, _ = crypto_rand.Read(b)` | 熵源异常时消息 ID 可能重复/全零 | ✅ 已修 |
| F10 | 重投递消息忽略错误 | `internal/agent/loop.go:524` | `_ = al.msgBus.PublishInbound(leftoverMsg)` | 排队消息静默丢失 | ✅ 已修 |
| F11 | 飞书 token 失效无重试 | `internal/channel/feishu.go:899` | `ensureToken` 仅在 token 为空时刷新 | 令牌被判失效（99991663）后，最长一个刷新窗口内所有回复失败 ✅ 已修（doAPIRequest / doAPIGetRaw 命中 99991661/3/8 时刷新并重试一次） | ✅ 已修 |
| F12 | 去重表每条消息全量重写 | `internal/channel/feishu.go:223,261` | 每次 `addIfNew` 都 `go o.saveToFile()` | O(n) 写盘/每消息；高并发下 goroutine 排队堆积 ✅ 已修（dirty + 1s 批量落盘，Stop 时强制刷盘） | ✅ 已修 |

### P2 — 稳健性与可维护性

| ID | 问题 | 位置 | 说明 |
|---|---|---|---|
| F13 | 会话锁是无超时的 `sync.Mutex` | `internal/agent/loop.go:452,483` | 一个卡住的回合会让该会话后续消息全部排队，goroutine 只增不减 | ✅ 已修 |
| F14 | 工具参数原文入日志 | `internal/agent/loop.go:1055,2048`、`internal/provider/openai.go:327` | 命令里若含 token/密钥会落到磁盘日志 | ✅ 已修 |
| F15 | 请求方自称身份 | `internal/api/handlers.go:449` | `channel_id`/`user_id` 来自 query/header，持密钥者可按 ID 读任意会话（单用户部署可接受，多用户即 IDOR） ⏸ 暂不做：需与前端一起改约定 |
| F16 | 静态资源仅靠 `filepath.Clean` | `internal/api/static.go:110` | 未校验软链；`static_dir` 内有软链时可越界 | ✅ 已修 |
| F17 | **仓库没有任何 CI** | 仓库根目录无 `.github/` | 无自动 `go test` / `go vet` / lint，回归全靠人肉 | ✅ 已修 |

---

## 四、修复顺序建议

排序原则：**先止血（进程不挂）→ 再保数据（不静默丢）→ 再做性能 → 最后补工程化**。

### 第 1 批（已完成）：panic 兜底

F1–F5。理由：这类问题的代价是「服务整体不可用」，而修复成本极低（每个 goroutine 三行 defer）。

### 第 2 批：静默失败改为可见（建议 1 次提交）

- F6/F7/F8：把 `_ =` 换成 `logger.Warn(...)`，必要时向上返回错误
- F9：`crypto_rand.Read` 失败直接返回错误（ID 生成不该降级）
- F10：重投递失败记 warn

验收：故意让目录不可写，日志里能看到明确告警。

### 第 3 批：飞书可靠性（建议单独一次提交）

- F11：捕获 99991663 后强制刷新并**重试一次**发送
- F12：去重写盘改为「标记 dirty + 定时批量落盘」（如每 2 秒或每 100 条）

理由：F11 直接决定「机器人不回消息」这类最高频投诉；F12 在群聊繁忙时才暴露，可以稍后。

### 第 4 批：并发与日志卫生

- F13：会话锁改为带超时的获取（如 2×`agent.message_timeout` 后返回「前一条还在处理」）
- F14：对 `arguments` 做脱敏（按 key 名匹配 `key/token/secret/password` 打码，或只在 debug 级别打印）

### 第 5 批：安全与工程化

- F15：把身份从请求参数改为认证上下文（Web UI cookie / API token → user_id）
- F16：`static_dir` 读取前用 `IsPathAllowedForTool` 校验
- F17：加 CI（`go vet` + `go test -race` + `staticcheck`），并给飞书 JSON 解析加 fuzz 测试

理由：这几项收益确定但不紧急，且 F17 能把前面所有修复「锁住」，防止回退。

---

## 五、本轮顺带确认「没问题」的部分

避免重复排查，以下项目已核实为正常：

| 项目 | 结论 |
|---|---|
| 去重表持久化 | 有 `saveMu` 串行化 + 临时文件 rename，不会写坏 |
| 会话文件裁剪 | 本轮之前已修（`FileStore` 支持 `max_messages_per_session`） |
| 令牌比较 | 全部使用 `subtle.ConstantTimeCompare` |
| `/api/config` 密钥 | 使用 `maskAPIKey` 掩码（`***last4`） |
| HTTP Body 泄漏 | 20 处请求均有 `defer resp.Body.Close()` |
| 子代理错误传播 | `announceError` → 模板投递回原会话，`resultChan` 只关闭一次 |
| 路径策略一致性 | 六个工具已统一走 `IsPathAllowedForTool` / `CheckCommandPaths` |

---

## 六、本次审计的局限

1. **未做形式化验证**：结论来自静态阅读 + 针对性测试，不代表没有未发现的逻辑错误。
2. **未覆盖 Flutter 客户端**（`neoray_app/`）与 Web UI 前端（`webui/`）。
3. **未做压力测试**：F12/F13 的影响是推算得出，未在真实高并发下量化。
4. **未审计第三方依赖**：未跑 `govulncheck`。
5. **飞书长连接边界**：未验证重连退避、事件乱序、重复推送的最坏情况。

建议下一步补 `govulncheck` 与一条基于飞书事件回放的 fuzz 用例。
