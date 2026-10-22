package channel

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// TestOrderedMapBatchFlush 批量落盘：标记 dirty 后应在后台刷盘，stop 时刷最后一批
func TestOrderedMapBatchFlush(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "ids.json")

	o := newOrderedMap(10)
	o.flushPeriod = 50 * time.Millisecond
	o.loadFromFile(path)
	t.Cleanup(o.stop)

	for i, id := range []string{"m1", "m2", "m3"} {
		if !o.addIfNew(id) {
			t.Fatalf("第 %d 条应判为新消息", i)
		}
	}

	// 等后台刷盘
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		data, err := os.ReadFile(path)
		if err == nil {
			var ids []string
			if json.Unmarshal(data, &ids) == nil && len(ids) == 3 {
				return
			}
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatalf("3 条记录未在预期时间内落盘: %v", readIDs(t, path))
}

// TestOrderedMapStopFlushes 停止时必须把未落盘的改动刷掉（重启后靠它去重）
func TestOrderedMapStopFlushes(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "ids.json")

	o := newOrderedMap(10)
	o.flushPeriod = time.Hour // 确保只有 stop 才会刷盘
	o.loadFromFile(path)

	o.addIfNew("only-on-stop")
	o.stop()

	ids := readIDs(t, path)
	if len(ids) != 1 || ids[0] != "only-on-stop" {
		t.Fatalf("stop 未刷盘，实际: %v", ids)
	}
}

// TestOrderedMapDedup 同一消息第二次应判重
func TestOrderedMapDedup(t *testing.T) {
	o := newOrderedMap(4)
	o.loadFromFile(filepath.Join(t.TempDir(), "x.json"))
	t.Cleanup(o.stop)

	if !o.addIfNew("a") {
		t.Fatal("首次应为新")
	}
	if o.addIfNew("a") {
		t.Fatal("第二次应判重")
	}
}

func readIDs(t *testing.T, path string) []string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	var ids []string
	_ = json.Unmarshal(data, &ids)
	return ids
}
