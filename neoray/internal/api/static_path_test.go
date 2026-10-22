package api

import (
	"os"
	"path/filepath"
	"testing"
)

// TestResolveDiskPathNormal 普通资源应解析到 diskDir 内
func TestResolveDiskPathNormal(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "index.html"), []byte("ok"), 0o644); err != nil {
		t.Fatal(err)
	}
	h := &staticHandler{diskDir: dir}

	got, err := h.resolveDiskPath("/index.html")
	if err != nil {
		t.Fatalf("不应报错: %v", err)
	}
	if filepath.Base(got) != "index.html" {
		t.Fatalf("解析结果异常: %s", got)
	}
}

// TestResolveDiskPathTraversal 路径穿越必须拒绝
func TestResolveDiskPathTraversal(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "dist")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	// 在 root 放一个"外部"文件
	if err := os.WriteFile(filepath.Join(root, "secret.txt"), []byte("secret"), 0o644); err != nil {
		t.Fatal(err)
	}
	h := &staticHandler{diskDir: dir}

	got, err := h.resolveDiskPath("../secret.txt")
	if err == nil {
		// Clean 已把 .. 消掉，结果必须仍在 dist 内（读不到外部文件即可）
		rel, relErr := filepath.Rel(dir, got)
		if relErr != nil || rel == ".." || filepath.IsAbs(rel) {
			t.Fatalf("路径穿越逃出了根目录: %s", got)
		}
	}
}

// TestResolveDiskPathSymlinkEscape 软链指向外部时必须拒绝（本次修复的核心）
func TestResolveDiskPathSymlinkEscape(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "dist")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	outside := filepath.Join(root, "outside.txt")
	if err := os.WriteFile(outside, []byte("secret"), 0o644); err != nil {
		t.Fatal(err)
	}
	// dist/leak -> ../outside.txt
	if err := os.Symlink(outside, filepath.Join(dir, "leak.txt")); err != nil {
		t.Skipf("创建软链失败（环境限制）: %v", err)
	}

	h := &staticHandler{diskDir: dir}
	if _, err := h.resolveDiskPath("leak.txt"); err == nil {
		t.Fatal("软链指向 diskDir 之外时应报错")
	}
}

// TestResolveDiskPathMissingFile 不存在的资源不报路径错误（交给 404 处理）
func TestResolveDiskPathMissingFile(t *testing.T) {
	h := &staticHandler{diskDir: t.TempDir()}
	if _, err := h.resolveDiskPath("assets/missing.js"); err != nil {
		t.Fatalf("缺失文件不应被判为越界: %v", err)
	}
}
