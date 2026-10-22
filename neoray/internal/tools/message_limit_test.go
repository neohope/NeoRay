package tools

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestAttachmentLimitRejectsTooMany 附件数量超上限应拒绝
func TestAttachmentLimitRejectsTooMany(t *testing.T) {
	tool := &MessageTool{}
	paths := make([]string, 0, maxAttachmentsPerMessage+1)
	for i := 0; i <= maxAttachmentsPerMessage; i++ {
		paths = append(paths, "/tmp/x")
	}
	_, err := tool.resolveMedia(paths)
	if err == nil || !strings.Contains(err.Error(), "too many attachments") {
		t.Fatalf("应因附件过多被拒，实际 %v", err)
	}
}

// TestAttachmentLimitRejectsOversize 单个附件超上限应拒绝
func TestAttachmentLimitRejectsOversize(t *testing.T) {
	dir := t.TempDir()
	big := filepath.Join(dir, "big.bin")
	f, err := os.Create(big)
	if err != nil {
		t.Fatal(err)
	}
	if err := f.Truncate(maxAttachmentBytes + 1); err != nil {
		f.Close()
		t.Skipf("无法创建大文件（环境限制）: %v", err)
	}
	f.Close()

	tool := &MessageTool{}
	_, err = tool.resolveMedia([]string{big})
	if err == nil || !strings.Contains(err.Error(), "too large") {
		t.Fatalf("应因附件过大被拒，实际 %v", err)
	}
}

// TestAttachmentLimitAllowsNormal 正常大小的附件不应被拒
func TestAttachmentLimitAllowsNormal(t *testing.T) {
	dir := t.TempDir()
	small := filepath.Join(dir, "small.txt")
	if err := os.WriteFile(small, []byte("hi"), 0o644); err != nil {
		t.Fatal(err)
	}

	tool := &MessageTool{}
	got, err := tool.resolveMedia([]string{small})
	if err != nil {
		t.Fatalf("正常附件不应被拒: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("应返回 1 个路径，实际 %d", len(got))
	}
}
