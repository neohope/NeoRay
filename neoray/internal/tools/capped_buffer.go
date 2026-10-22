package tools

// cappedBuffer 是带总量上限的 io.Writer：
// 超过上限后停止增长并记录被丢弃的字节数。
//
// 一次性 shell 执行此前直接用 bytes.Buffer 接 stdout/stderr：
// 一条 `cat 大文件` 或 `yes` 就能把进程内存吃掉（工具侧没有超时之外的兜底）。
type cappedBuffer struct {
	buf     []byte
	max     int
	dropped int
}

func newCappedBuffer(max int) *cappedBuffer {
	if max <= 0 {
		max = DefaultMaxOutput
	}
	return &cappedBuffer{max: max}
}

func (c *cappedBuffer) Write(p []byte) (int, error) {
	remaining := c.max - len(c.buf)
	if remaining <= 0 {
		c.dropped += len(p)
		return len(p), nil
	}
	if len(p) > remaining {
		c.buf = append(c.buf, p[:remaining]...)
		c.dropped += len(p) - remaining
		return len(p), nil
	}
	c.buf = append(c.buf, p...)
	return len(p), nil
}

func (c *cappedBuffer) String() string { return string(c.buf) }

// Dropped 返回被丢弃的字节数（0 表示未截断）
func (c *cappedBuffer) Dropped() int { return c.dropped }
