# NeoRay 代码审计（第三轮）

- 审计日期：2026-10-22（第三轮，第二轮修复完成后）
- 基线提交：`182075b`
- 范围：`neoray/` 后端（76 个非测试文件，约 3.1 万行）+ 随包技能内容
- 前置状态：前两轮 23 项问题已修 22 项；`gofmt` / `vet` / `-race` / `staticcheck` 全绿

> 前两轮清掉了「崩溃 / 静默失败 / 硬编码 / 死代码 / 语义与配置不符」。
> 本轮聚焦 **消息链路的投递可靠性** 与 **每轮重复计算的开销**，另外检查了随包技能内容。

---

## 一、本轮发现

### P1｜消息会真的丢

#### M1. 出站总线「满了就丢」，且各订阅者只有 50 的缓冲

- 位置：`internal/bus/bus.go:251-256`（`default:` 分支丢弃）、
  `internal/api/server.go:396`、`internal/channel/channel.go:93`（容量均为 50）
- 证据：
  ```go
  select {
  case ch <- msg:
  case <-b.stopChan:
      return
  default:
      b.recordDrop(id)                    // 记指标 + warn，然后丢弃这条出站消息
      logger.Warn("Outbound subscriber queue full, dropping message", ...)
  }
  ```
- 影响链：
  1. 飞书 API 单条发送 200~800ms，频道侧是**单 goroutine 串行发送**（`channel.go:106`）
  2. 用户连发多条 / 一次 turn 产生多条回复时，50 的缓冲几秒就满
  3. 满了以后总线**直接丢弃**——用户看到的是「AI 没回」，日志里只有一条 warn
  4. 发送失败（非 token 类错误）也只记 `logger.Error`，没有重试
- 结论：这是前三轮里**唯一会让用户实际丢消息**的问题，优先级最高。

### P2｜每轮重复计算 + 内容错误

#### M2. 技能清单与系统提示没有缓存，每轮迭代都重扫磁盘

- 位置：`internal/memory/context_builder.go:116`（`buildSkillsSummary`）、
  `internal/skills/skills.go:142`（`ListSkills` 每次读目录 + 解析 SKILL.md）
- 调用链：每次 `BuildMessages` → `BuildSystemPrompt` → `Snapshot()` → `ListSkills()`
  → 逐个 `SKILL.md` 读文件 + YAML 解析；`GetSkillMetadata` 还会**再解析一遍**同一文件
- 实测（真实技能目录，10 个技能）：

  | 操作 | 耗时 |
  |---|---|
  | `Snapshot()`（含目录扫描 + YAML 解析） | **758 µs / 次** |
  | 一轮 10 次工具迭代的累计开销 | ≈ 7.6 ms |

  绝对量不大，但有两个副作用：
  1. 每次构建 prompt 都会**重复输出 YAML 解析告警**（见 M3），日志被刷屏
  2. 技能目录越大、技能越多，成本线性上升
- 建议：给 `ListSkills` / `BuildSkillsSummary` 加「目录 mtime + 配置」为 key 的缓存，
  技能目录未变更时直接复用。

#### M3. 随包技能 `update-setup` 的 frontmatter 解析失败（内容 bug）

- 位置：`neoray/skills/update-setup/SKILL.md:3`
- 证据：`description` 未加引号却含 `: `
  ```
  description: One-time setup wizard for the neoray upgrade skill. Triggers: setup update, ...
  ```
  YAML 解析报 `mapping values are not allowed in this context`，代码退化为默认值。
- 影响：该技能的描述与 `requires` 信息丢失（清单里只剩名字），
  且每次解析都会打一条 warn —— 与 M2 叠加后变成日志噪音。
- 修复：给 description 加引号（或把 `Triggers:` 改成 `Triggers -`）。

### P3｜边界缺失，风险随使用规模上升

| 编号 | 问题 | 位置 | 说明 |
|---|---|---|---|
| M4 | 飞书长回复没有长度校验/分片 | `internal/channel/feishu.go:2341` `markdownToPost` | 只对**表格**做了分段（`splitElementsByTableLimit`），整体内容没有上限；超限时发送失败只记日志，用户收不到 |
| M5 | `message` 工具附件无大小/数量上限 | `internal/tools/message.go` | 传入超大文件会整份读入内存再上传 |
| M6 | `dream` 的提示词没有总量上限 | `internal/memory/dream.go:195` | 每条历史截断到 4000 字符，但批次 20 条 → 最多约 80 KB prompt，长历史下会挤占模型上下文 |

---

## 二、明确「不值得做」的项

| 想法 | 为什么不做 |
|---|---|
| 把出站总线换成带持久化的消息队列 | 单进程部署，先解决「缓冲容量 + 重试」即可；引入 MQ 是数倍运维成本 |
| 给技能清单做增量文件监听（fsnotify） | 目录 mtime 缓存已足够；多一个 watcher 就多一类泄漏源 |
| 把所有 `markdownToPost` 输出都做智能分片 | 分片会打断 Markdown 结构；先做「超限提示 + 按段落切分」即可 |
| 给 `dream` 做分段流式摘要 | 当前批次上限 20 条已能控制规模，收益不抵复杂度 |

---

## 三、修复顺序建议

原则：**先止血（不丢消息）→ 再清噪音（内容错误）→ 再降开销 → 最后补边界**。

### 第 1 批：M1 消息投递可靠性（半天，最高优先）

- 订阅者缓冲 50 → 512（可配置）；丢弃时降级为「阻塞 1s 再丢」，避免瞬时抖动就丢
- 频道发送改为「每频道 N 并发 + 失败重试 1 次（指数退避）」
- 丢弃指标 `recordDrop` 接入日志汇总（每分钟一条，而不是每条一条）
- 验收：连发 100 条测试消息，用户侧 0 丢失

### 第 2 批：M3 技能 frontmatter（10 分钟）

- `update-setup/SKILL.md` 的 description 加引号
- 顺带在 `skills` 加载器里加一条：解析失败时用 `logger.Warn` 带**文件路径**（现在只有 YAML 错误，定位不到是哪个技能）
- 验收：启动日志里不再出现 YAML 解析告警

### 第 3 批：M2 技能/提示缓存（半天）

- 以「workspace skills 目录 mtime + builtin 目录 mtime + disabledSkills」为 key 缓存 `ListSkills` 结果
- `GetSkillMetadata` 复用同一次解析结果，不再二次读盘
- 加基准测试：缓存命中时 `Snapshot()` 应 < 50 µs
- 验收：一次 turn 的 10 次迭代里，只发生 1 次目录扫描

### 第 4 批：M4/M5/M6 边界（1 天，按需）

- M4：回复超过阈值（如 20 KB）时按段落切分为多条消息；无法切分时截断并提示
- M5：附件超过 `security.upload.max_size`（配置已存在但未接线）时拒绝并说明
- M6：`dream` 构建提示时按总字符预算（如 60 KB）截断，超出部分标注省略

---

## 四、与前两轮的关系

| 维度 | 第一轮 | 第二轮 | 第三轮 |
|---|---|---|---|
| 主线 | 崩溃/静默失败/硬编码 | 语义与配置一致性 | 消息投递可靠性、重复计算 |
| 代表问题 | panic 兜底、工具结果不截断 | DeepCopy、cron 并发 | 出站丢消息、技能无缓存 |
| 新引入的检查手段 | staticcheck | -race + 端到端验证 | 真实耗时测量 + 内容检查 |

---

## 五、局限

1. 未做真实压测：M1 的丢弃概率来自「串行发送 + 50 缓冲」的推算，建议用连发脚本实测。
2. 未核对飞书官方消息体上限（M4 只确认了「没有校验」，未给出精确字节阈值）。
3. 未覆盖 Flutter 客户端与 Web UI 前端。
4. 未审计第三方依赖（govulncheck 在 CI 中为信息性任务）。
