package session

import (
	"os"
	"testing"
	"time"
)

// TestDeletedSessionNotResurrected 删除后，in-flight 回合的旧副本不应把会话写回来。
func TestDeletedSessionNotResurrected(t *testing.T) {
	dir := t.TempDir()
	store, err := NewFileStore(dir)
	if err != nil {
		t.Fatal(err)
	}

	sess := NewSession("feishu", "ou_1")
	if err := store.Save(sess); err != nil {
		t.Fatal(err)
	}

	// 模拟：回合开始时拿到的副本
	stale := sess.DeepCopy()
	time.Sleep(10 * time.Millisecond)

	if err := store.Delete(sess.ID); err != nil {
		t.Fatal(err)
	}

	// 回合结束时的保存
	if err := store.Save(stale); err != nil {
		t.Fatalf("保存旧副本不应报错（应静默忽略）: %v", err)
	}

	if _, err := store.Get(sess.ID); err == nil {
		t.Fatal("被删除的会话又被写回来了")
	}
	if _, statErr := os.Stat(store.getSessionFilePath(sess.ID)); !os.IsNotExist(statErr) {
		t.Fatal("会话文件被重新创建")
	}
}

// TestRecreatedSessionWithSameIDPersists 同 ID 重建（创建时间更晚）应允许落盘
func TestRecreatedSessionWithSameIDPersists(t *testing.T) {
	dir := t.TempDir()
	store, err := NewFileStore(dir)
	if err != nil {
		t.Fatal(err)
	}

	old := NewSession("feishu", "ou_2")
	if err := store.Save(old); err != nil {
		t.Fatal(err)
	}
	if err := store.Delete(old.ID); err != nil {
		t.Fatal(err)
	}

	time.Sleep(10 * time.Millisecond)
	fresh := NewSession("feishu", "ou_2")
	fresh.ID = old.ID // 同一个确定性 ID
	if err := store.Save(fresh); err != nil {
		t.Fatal(err)
	}
	if _, err := store.Get(old.ID); err != nil {
		t.Fatalf("重建的会话应能保存: %v", err)
	}
}

// TestListReturnsCopies List 必须返回副本，不能把内部对象交出去
func TestListReturnsCopies(t *testing.T) {
	dir := t.TempDir()
	store, err := NewFileStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	sess := NewSession("feishu", "ou_3")
	if err := store.Save(sess); err != nil {
		t.Fatal(err)
	}

	list, err := store.List()
	if err != nil || len(list) != 1 {
		t.Fatalf("列表异常: %v len=%d", err, len(list))
	}
	list[0].ChannelID = "TAMPERED"

	reloaded, err := store.Get(sess.ID)
	if err != nil {
		t.Fatal(err)
	}
	if reloaded.ChannelID == "TAMPERED" {
		t.Fatal("List 返回了内部对象，调用方能直接改存储")
	}
}
