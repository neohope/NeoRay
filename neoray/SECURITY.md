# 安全模型：工具与路径访问策略

本文说明 NeoRay 在「agent 能不能碰这个文件」上的统一规则，以及新增工具时该怎么做。

## 一句话规则

> 所有工具的路径判定，都必须调用 `security.IsPathAllowedForTool`；
> 所有 shell 命令的路径判定，都必须调用 `security.CheckCommandPaths`。
> 不允许任何工具自己写一套 `filepath.Join(workspace, ...)` + 前缀比较。

原因很直接：这套判定曾经在 5 个工具里各写了一遍，结果 shell 认 `allowed_paths`、
`read_file` 不认、`grep` 越界时静默搜工作区、`apply_patch` 干脆拒绝绝对路径。
**横切策略一旦重复实现，就一定会漂移。**

## 允许范围

```toml
[security]
restrict_to_workspace = true      # 关闭时所有路径检查都不生效

[tools.workspace]
allowed_paths = ["~/.neoray", "/tmp/neoray"]
```

判定集合 = `workspace`（默认 `~/.neoray/workspace`）∪ `allowed_paths`。

| 路径 | 结果 |
|---|---|
| `~/.neoray/workspace/**` | ✅ |
| `~/.neoray/**` | ✅（来自 allowed_paths） |
| `/tmp/neoray/**` | ✅ |
| `/tmp/其他` | ⛔ 父目录不放行 |
| `/tmp/neoray-evil` | ⛔ 前缀相同但不同目录 |
| `/etc`、`~/Documents`、`~/.ssh` | ⛔ |

三条实现细节：

1. **软链解析**：已存在的路径先 `EvalSymlinks` 再比较，防止工作区内软链指向外部。
2. **不存在的路径**（新建文件、`go build ./...` 的 `...`）：退回纯词法比较（`filepath.Rel`）。
3. **大小写/卷**：交给 `filepath.Rel` 处理，不手工拼字符串前缀。

## 各工具的行为

| 工具 | 相对路径 | 绝对路径 | `~` | `..` | 越界时 |
|---|---|---|---|---|---|
| `shell` | ✅ 按 workspace | ✅ | ✅ 展开 | 由分段解析处理 | 拒绝执行 |
| `read_file` / `write_file` | ✅ | ✅ | ❌ 不展开 | 解析后校验 | 返回错误 |
| `grep` | ✅ | ✅ | ❌ | 解析后校验 | 返回错误（**不再静默搜工作区**） |
| `find_files` | ✅ | ✅ | ❌ | 解析后校验 | 返回错误 |
| `apply_patch` | ✅ | ✅ | ✅ 展开 | 相对路径禁止 `..` | 返回错误 |
| `message`（附件） | ✅ | ✅ | ❌ | 解析后校验 | 返回错误 |

统一函数：`security.IsPathAllowedForTool(path, workspace, allowedPaths) bool`

## shell 命令的额外规则

`CheckCommandPaths` 先把命令按 shell 操作符切分，再逐 token 校验路径：

- 分段符：`|`、`||`、`&&`、`;`、重定向 `>` `>>` `<`
- **尊重引号**：`"a|b"` 不会被切成两段；引号内的路径照样校验
- 重定向目标也算路径：`echo x > /etc/hosts` 会被拒
- 下面这些写法会整体拒绝（无法静态确定真实路径）：

| 写法 | 例子 | 原因 |
|---|---|---|
| `$` 变量展开 | `cat $HOME/.ssh/id_rsa` | 运行时才展开成真实路径 |
| `$( )` 命令替换 | `ls $(pwd)/../..` | 同上 |
| 反引号 | `` cat `id` `` | 同上 |
| 内联代码里的路径 | `python3 -c 'open("/etc/passwd")'` | 含括号又含 `/`，视为内联代码 |
| 进程替换 | `>(cmd)`、`<(cmd)` | 同上 |
| 换行 | 多行命令 | 逐段校验会漏 |

正则里的括号不受影响：`grep -E "a(b|c)" file.txt` 仍然可用（该 token 不含 `/`）；
只有「同时含括号与 `/`」的 token 才按内联代码拒绝。

## 新增工具时照抄这段

```go
func (t *MyTool) Execute(ctx context.Context, args json.RawMessage) (json.RawMessage, error) {
    full := params.Path
    if !filepath.IsAbs(full) {
        full = filepath.Join(t.workspace, full)   // 相对路径按 workspace 解析
    }
    if !security.IsPathAllowedForTool(full, t.workspace, t.allowedPaths) {
        return nil, fmt.Errorf("path outside workspace and allowed_paths: %s%s",
            params.Path, security.WorkspaceBoundaryNote)
    }
    // ... 真正的操作
}
```

构造函数里注入配置：

```go
func NewMyTool(cfgs ...*config.Config) *MyTool {
    t := &MyTool{workspace: config.GetWorkspace()}
    for _, c := range cfgs {
        if c != nil {
            t.workspace = resolveWorkspacePath(c)
            t.allowedPaths = c.Tools.Workspace.AllowedPaths
        }
    }
    return t
}
```

并补三类测试：**允许目录内可读写**、**界外被拒**、**边界不误伤**（`/tmp/neoray-evil` 必须拒）。
参考 `internal/tools/tool_path_allow_test.go`、`internal/security/command_paths_test.go`。

## 这套机制**不能**保证什么

必须说清楚，避免把护栏当保险箱：

1. **它不是操作系统级沙箱。** `tools.shell.sandbox` 目前只有 `bwrap` 后端（Linux），
   macOS 上没有任何后端，因此命令过滤是**唯一**的防线。
2. **命令过滤对「任意代码执行」不可靠。** 只要能跑解释器，就能绕开静态检查。
   我们拦住了 `python3 -c 'open(...)'` 这类常见写法，但拦截是启发式的，不是证明。
3. **web_search / web_fetch 的内容是外部不可信数据。** 提示注入可能诱导 agent
   尝试越界，真正的防线是上面的路径检查 + 不把密钥写进配置以外的位置。
4. **软链竞态**（检查与打开之间被替换）只在 `read_file` 里做了 open 后二次校验，
   其它工具没有。要彻底解决需要 `O_NOFOLLOW` 或 OS 级沙箱。

### 需要真正隔离时

按强度递增：

1. 收紧 `allowed_paths`（最有效、成本最低）
2. 用容器/虚拟机跑 NeoRay，只挂载必要目录
3. 为 macOS 补一个 `sandbox-exec`（seatbelt）后端，与 `bwrap` 并列注册
4. 生产环境上 Linux + `tools.shell.sandbox = "bwrap"`

## 相关代码

| 位置 | 作用 |
|---|---|
| `internal/security/command_paths.go` | `CheckCommandPaths` / `IsPathAllowedForTool` / 分段解析 |
| `internal/security/workspace_policy.go` | `IsPathWithin`（软链解析）、`ResolvePath` |
| `internal/tools/shell.go`、`exec_session.go` | 命令执行前调用 `CheckCommandPaths` |
| `internal/tools/filesystem.go`、`search.go`、`patch.go`、`message.go` | 调用 `IsPathAllowedForTool` |
| `internal/tools/sandbox.go` | OS 级沙箱后端注册（当前仅 bwrap） |
