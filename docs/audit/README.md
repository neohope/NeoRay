# NeoRay 代码审计索引

本目录记录 2026-10-22 进行的三轮代码审计：发现的每一个问题、修复它的提交、
以及仍然保留的决策项。**看这一页就够了，细节在各自报告里。**

- 范围：`neoray/` 后端（76 个非测试 Go 文件，约 3.1 万行）
- 方法：静态检索 + `go vet` + `go test -race` + `staticcheck` + 真实配置端到端验证
- 前三轮结果：**30 项问题 = 28 项代码修复 + 1 项按决策仅文档化（N4）+ 1 项按决策保留（F15）**
- 第四轮（全量通读）新增 **29 项（P0 3 / P1 12 / P2 14）+ 38 个依赖漏洞**，**已全部修复**：
  R1–R29 逐项落地并有回归测试；依赖升级到 Go 1.26.10 / x/net 0.60.0 / websocket 1.5.3，
  `govulncheck` 从 38 项降为 **No vulnerabilities found**，CI 也改为阻塞任务。
  详见 [第四轮报告](2026-10-23-audit-round4.md)
- 提交：14 个修复/优化提交 + 4 个报告提交；新增 20 个测试文件

| 报告 | 主题 | 对应提交 |
|---|---|---|
| [第一轮](2026-10-22-code-audit.md) | 崩溃 / 静默失败 / 硬编码 / 死代码 | `725da3d` … `a82ef7e` |
| [第二轮](2026-10-22-audit-round2.md) | 语义与配置一致性、并发模型 | `6b9a717` … `182075b` |
| [第三轮](2026-10-22-audit-round3.md) | 投递可靠性、重复计算、内容正确性 | `883c5b1` … `6f67385` |
| [第四轮](2026-10-23-audit-round4.md) | **全量通读**（76 文件 / 3.17 万行）+ 依赖漏洞 + 覆盖率 | `8b9f324` … `83492b7` |

---

## 一、已修复问题总表

### 第一轮：崩溃与静默失败

| 编号 | 问题 | 修复提交 |
|---|---|---|
| F1–F5 | 五处 goroutine 缺 panic 兜底（飞书事件、agent 会话、cron 任务、WS handler、`Stop()` 关通道竞态） | `725da3d` |
| F6–F10 | 五处静默失败（归档、压缩、建目录、`crypto/rand`、重投递）改为告警或返回错误 | `0fbf8e7` |
| F11 | 飞书 token 失效不重试 → 命中 99991661/3/8 时刷新并重试一次 | `77b40bc` |
| F12 | 去重表每条消息整表重写 → dirty 标记 + 1s 批量落盘 + Stop 刷盘 | `77b40bc` |
| F13 | 会话锁无超时 → 带超时获取，超时回「上一条还在处理」 | `1cadbe9` |
| F14 | 工具参数/结果原文入日志 → 按 JSON 字段 / Bearer / `KEY=value` / URL 凭证四类打码 | `1cadbe9` |
| F16 | 静态资源未校验软链 → `resolveDiskPath` 解析软链并校验根目录 | `299cb05` |
| F17 | 仓库无 CI → gofmt / vet / test -race / staticcheck / govulncheck | `299cb05` |
| — | 清空 staticcheck 报告的 21 处死代码（U1000 归零） | `a82ef7e` |
| — | 修 staticcheck 发现的真实缺陷：RE2 不支持 `\h\1` 反向引用会 panic、子代理结果路由未用回退 session key、飞书 `err != nil` 恒真 | `299cb05` |

### 第二轮：语义与并发

| 编号 | 问题 | 修复提交 |
|---|---|---|
| N1 | 工具结果完全不截断（配置只用于展示）→ `TruncateToolResult`，头 2/3 + 尾 1/3 | `6b9a717` |
| N2 | cron 到期任务串行执行 → 每任务独立 goroutine + 重入保护 + Stop 超时 | `7c91473` |
| — | （N2 测试中 `-race` 抓到）logger 全局懒初始化竞态 | `7c91473` |
| N3 | `Session.DeepCopy` 浅拷贝嵌套字段 → 逐条深拷贝 + 16 层递归上限 | `20124f1` |
| N5 | 飞书去重容量写死 1000 → `channels.feishu.dedup_capacity`（默认 5000） | `ed736c7` |
| N6 | cron 任务失败仍按原周期推进 → 1/2/4/8 周期退避，成功复位 | `ed736c7` |
| N7 | 会话详情一次返回全部消息 → `?limit=&before=` 索引游标分页 | `182075b` |
| — | （N7 连带）会话 ID 正则拒绝 `feishu:ou_xxx`、身份校验对空值会话误判 404、历史消息无 ID 导致游标失效 | `182075b` |
| N4 | 停机期间错过的周期任务 | **不实现补跑**（决策），只写文档：`neoray.toml.example` 已注明 —— 即「问题已定性、行为已声明」，不是待办 |

### 第三轮：投递与开销

| 编号 | 问题 | 修复提交 |
|---|---|---|
| M1 | 出站总线「满了就丢」（缓冲仅 50、发送无重试）→ 缓冲 512 + 2s 宽限 + 三轮重试 + 告警聚合 | `883c5b1` |
| M3 | `update-setup` 的 YAML frontmatter 未加引号导致解析失败 | `883c5b1` |
| M2 | 技能清单无缓存（758µs/次）→ 目录签名缓存 + 1s TTL，降到 **5µs** | `df5b555` |
| M4 | 飞书长回复无长度校验 → 按段落切分 + `max_message_chars`（默认 8000） | `6f67385` |
| M5 | `message` 附件无上限 → 5 个 / 20MB | `6f67385` |
| M6 | `dream` 提示词无总量上限 → 60KB 预算，保留最新条目 | `6f67385` |

---

## 二、仍然保留的一项

**F15：API 请求方自称身份**（`channel_id` / `user_id` 来自 query 或 header）

- 现状：持 API 密钥者可读取任意会话（知道 ID 即可）
- 为什么不修：当前是单用户、单密钥部署，改动需要同时修改 Web UI 的调用约定，
  收益低于成本；多用户改造时应与鉴权体系一并处理
- 若未来要多用户：身份应从认证上下文取，而不是请求参数

---

## 三、如何验证这些修复

```bash
cd neoray
go vet ./...
staticcheck -checks "inherit,-ST1018" ./...
go test -race ./internal/... ./test/... -count=1
```

CI 会自动跑同样四步（`.github/workflows/ci.yml`）。
唯一保留的 lint 豁免是 `ST1018`：默认 SOUL 文案里 emoji 的零宽连接符属有意为之。

关键回归测试位置：

| 主题 | 测试文件 |
|---|---|
| 工具结果截断 | `internal/tools/truncate_test.go` |
| 出站投递与背压 | `internal/bus/outbound_test.go` |
| 频道发送重试 | `internal/channel/send_retry_test.go` |
| 长消息切分 | `internal/channel/split_test.go` |
| 技能缓存与失效 | `internal/skills/cache_test.go` |
| 会话深拷贝 | `internal/session/deepcopy_test.go` |
| 分页 | `internal/session/pagination_test.go` |
| cron 并发与退避 | `internal/cron/concurrent_test.go`、`internal/cron/backoff_test.go` |
| 工具路径策略 | `internal/security/command_paths_test.go`、`internal/tools/tool_path_allow_test.go` |

---

## 四、审计本身的局限

1. 未覆盖 Flutter 客户端（`neoray_app/`）与 Web UI 前端（`webui/`）
2. 未做真实压测：M1 的丢弃概率、N2 的阻塞时长是推算
3. 未核对飞书官方消息体上限精确值（M4 只确认「没有校验」，默认值取保守的 8000）
4. `govulncheck` 在 CI 中为信息性任务，未阻塞
5. 未验证飞书弱网重连时的重连退避与事件乱序
