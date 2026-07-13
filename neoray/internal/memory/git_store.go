package memory

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"
)

// CommitInfo 提交信息
type CommitInfo struct {
	SHA       string `json:"sha"`
	Message   string `json:"message"`
	Timestamp string `json:"timestamp"`
}

// LineAge 行年龄信息
type LineAge struct {
	AgeDays int `json:"age_days"`
}

// GitStore Git 版本控制
type GitStore struct {
	workspace   string
	trackedFiles []string
}

// NewGitStore 创建 GitStore
func NewGitStore(workspace string, trackedFiles []string) *GitStore {
	return &GitStore{
		workspace:   workspace,
		trackedFiles: trackedFiles,
	}
}

// IsInitialized 检查是否已初始化
func (gs *GitStore) IsInitialized() bool {
	gitDir := filepath.Join(gs.workspace, ".git")
	_, err := os.Stat(gitDir)
	return err == nil
}

// Init 初始化 Git 仓库
func (gs *GitStore) Init() bool {
	if gs.IsInitialized() {
		return false
	}

	// 检查是否已在 Git 仓库内
	if gs.isInsideGitRepo() {
		return false
	}

	// 初始化 Git 仓库
	if err := gs.runGitCommand("init"); err != nil {
		return false
	}

	// 创建 .gitignore
	if err := gs.writeGitignore(); err != nil {
		return false
	}

	// 确保追踪文件存在
	for _, f := range gs.trackedFiles {
		path := filepath.Join(gs.workspace, f)
		dir := filepath.Dir(path)
		_ = os.MkdirAll(dir, 0755)
		if _, err := os.Stat(path); os.IsNotExist(err) {
			_ = os.WriteFile(path, []byte{}, 0644)
		}
	}

	// 初始提交
	if err := gs.runGitCommand("add", "--", ".gitignore"); err != nil {
		return false
	}
	for _, f := range gs.trackedFiles {
		_ = gs.runGitCommand("add", "--", f)
	}

	if err := gs.runGitCommand("commit", "-m", "init: neoray memory store"); err != nil {
		return false
	}

	return true
}

// AutoCommit 自动提交变更
func (gs *GitStore) AutoCommit(message string) string {
	if !gs.IsInitialized() {
		return ""
	}

	// 检查是否有变更
	status, err := gs.getStatus()
	if err != nil || !gs.hasChanges(status) {
		return ""
	}

	// 添加追踪文件
	for _, f := range gs.trackedFiles {
		_ = gs.runGitCommand("add", "--", f)
	}

	// 提交
	if err := gs.runGitCommand("commit", "-m", message); err != nil {
		return ""
	}

	// 获取短 SHA
	sha, _ := gs.getShortSHA()
	return sha
}

// Log 获取提交历史
func (gs *GitStore) Log(maxEntries int) []CommitInfo {
	if !gs.IsInitialized() {
		return nil
	}

	args := []string{"log", "--pretty=format:%h|%s|%ad", "--date=format:%Y-%m-%d %H:%M"}
	if maxEntries > 0 {
		args = append(args, fmt.Sprintf("-%d", maxEntries))
	}

	output, err := gs.runGitCommandOutput(args...)
	if err != nil {
		return nil
	}

	var commits []CommitInfo
	scanner := bufio.NewScanner(strings.NewReader(output))
	for scanner.Scan() {
		line := scanner.Text()
		parts := strings.SplitN(line, "|", 3)
		if len(parts) == 3 {
			commits = append(commits, CommitInfo{
				SHA:       parts[0],
				Message:   parts[1],
				Timestamp: parts[2],
			})
		}
	}

	return commits
}

// LineAges 获取文件每行的修改时间
func (gs *GitStore) LineAges(filePath string) []LineAge {
	if !gs.IsInitialized() {
		return nil
	}

	fullPath := filepath.Join(gs.workspace, filePath)
	if _, err := os.Stat(fullPath); os.IsNotExist(err) {
		return nil
	}

	// 使用 git blame
	output, err := gs.runGitCommandOutput("blame", "--line-porcelain", "--", filePath)
	if err != nil {
		return nil
	}

	return gs.parseBlameOutput(output)
}

// DiffCommits 比较两个提交
func (gs *GitStore) DiffCommits(sha1, sha2 string) string {
	if !gs.IsInitialized() {
		return ""
	}

	output, _ := gs.runGitCommandOutput("diff", "--", sha1, sha2)
	return output
}

// FindCommit 查找提交
func (gs *GitStore) FindCommit(shortSHA string, maxEntries int) *CommitInfo {
	commits := gs.Log(maxEntries)
	for _, c := range commits {
		if strings.HasPrefix(c.SHA, shortSHA) {
			return &c
		}
	}
	return nil
}

// ShowCommitDiff 显示提交与父提交的差异
func (gs *GitStore) ShowCommitDiff(shortSHA string, maxEntries int) (*CommitInfo, string) {
	commits := gs.Log(maxEntries)
	for i, c := range commits {
		if strings.HasPrefix(c.SHA, shortSHA) {
			var diff string
			if i+1 < len(commits) {
				diff = gs.DiffCommits(commits[i+1].SHA, c.SHA)
			}
			return &c, diff
		}
	}
	return nil, ""
}

// Revert 还原提交
func (gs *GitStore) Revert(commitSHA string) string {
	if !gs.IsInitialized() {
		return ""
	}

	// 获取完整 SHA
	fullSHA := gs.resolveSHA(commitSHA)
	if fullSHA == "" {
		return ""
	}

	// 获取父提交
	parentSHA, err := gs.getParentSHA(fullSHA)
	if err != nil || parentSHA == "" {
		return ""
	}

	// 从父提交恢复文件
	for _, f := range gs.trackedFiles {
		if err := gs.restoreFileFromCommit(parentSHA, f); err != nil {
			continue
		}
	}

	// 提交还原
	return gs.AutoCommit(fmt.Sprintf("revert: undo %s", commitSHA))
}

// 内部方法

func (gs *GitStore) runGitCommand(args ...string) error {
	cmd := exec.Command("git", args...)
	cmd.Dir = gs.workspace
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func (gs *GitStore) runGitCommandOutput(args ...string) (string, error) {
	cmd := exec.Command("git", args...)
	cmd.Dir = gs.workspace
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("git %s: %w", strings.Join(args, " "), errors.New(stderr.String()))
	}
	return stdout.String(), nil
}

func (gs *GitStore) isInsideGitRepo() bool {
	// 向上查找 .git
	current := gs.workspace
	for {
		gitDir := filepath.Join(current, ".git")
		if _, err := os.Stat(gitDir); err == nil {
			return true
		}
		parent := filepath.Dir(current)
		if parent == current {
			break
		}
		current = parent
	}
	return false
}

func (gs *GitStore) writeGitignore() error {
	gitignorePath := filepath.Join(gs.workspace, ".gitignore")

	// 构建 .gitignore 内容
	var content string
	content += "/*\n"

	// 添加目录例外
	dirs := make(map[string]bool)
	for _, f := range gs.trackedFiles {
		dir := filepath.Dir(f)
		if dir != "." && dir != "/" {
			dirs[dir] = true
		}
	}

	var sortedDirs []string
	for d := range dirs {
		sortedDirs = append(sortedDirs, d)
	}
	sort.Strings(sortedDirs)

	for _, d := range sortedDirs {
		content += fmt.Sprintf("!%s/\n", d)
	}

	// 添加文件例外
	for _, f := range gs.trackedFiles {
		content += fmt.Sprintf("!%s\n", f)
	}

	content += "!.gitignore\n"

	// 读取现有内容（如果有）
	existing, _ := os.ReadFile(gitignorePath)
	if len(existing) > 0 {
		// 合并，避免重复
		existingLines := strings.Split(string(existing), "\n")
		existingSet := make(map[string]bool)
		for _, line := range existingLines {
			existingSet[strings.TrimSpace(line)] = true
		}

		var newLines []string
		for _, line := range strings.Split(content, "\n") {
			line = strings.TrimSpace(line)
			if line != "" && !existingSet[line] {
				newLines = append(newLines, line)
			}
		}

		if len(newLines) > 0 {
			content = string(existing) + "\n" + strings.Join(newLines, "\n")
		} else {
			content = string(existing)
		}
	}

	return os.WriteFile(gitignorePath, []byte(content), 0644)
}

func (gs *GitStore) getStatus() (string, error) {
	return gs.runGitCommandOutput("status", "--porcelain")
}

func (gs *GitStore) hasChanges(status string) bool {
	return strings.TrimSpace(status) != ""
}

func (gs *GitStore) getShortSHA() (string, error) {
	return gs.runGitCommandOutput("rev-parse", "--short", "HEAD")
}

func (gs *GitStore) resolveSHA(shortSHA string) string {
	output, err := gs.runGitCommandOutput("rev-parse", "--", shortSHA)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(output)
}

func (gs *GitStore) getParentSHA(sha string) (string, error) {
	output, err := gs.runGitCommandOutput("rev-parse", "--", sha+"^")
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(output), nil
}

func (gs *GitStore) restoreFileFromCommit(commitSHA, filePath string) error {
	content, err := gs.runGitCommandOutput("show", "--", commitSHA+":"+filePath)
	if err != nil {
		return err
	}

	fullPath := filepath.Join(gs.workspace, filePath)
	return os.WriteFile(fullPath, []byte(content), 0644)
}

func (gs *GitStore) parseBlameOutput(output string) []LineAge {
	var ages []LineAge
	scanner := bufio.NewScanner(strings.NewReader(output))

	var currentTime int64
	inHeader := true

	for scanner.Scan() {
		line := scanner.Text()
		if inHeader {
			if strings.HasPrefix(line, "author-time ") {
				tsStr := strings.TrimSpace(line[len("author-time "):])
				if ts, err := strconv.ParseInt(tsStr, 10, 64); err == nil {
					currentTime = ts
				}
			} else if strings.HasPrefix(line, "\t") {
				// 内容行，计算年龄
				ageDays := int(time.Since(time.Unix(currentTime, 0)).Hours() / 24)
				ages = append(ages, LineAge{AgeDays: ageDays})
				inHeader = false
			}
		} else {
			// 检查是否是新的头部（以 SHA 开头）
			if len(line) > 0 && len(strings.Fields(line)) > 0 {
				// 可能是新的头部，重置
				inHeader = true
			} else if strings.HasPrefix(line, "\t") {
				// 继续内容行
				ageDays := int(time.Since(time.Unix(currentTime, 0)).Hours() / 24)
				ages = append(ages, LineAge{AgeDays: ageDays})
			}
		}
	}

	return ages
}
