package session

// PaginateMessages 按「最近的 limit 条、且结束于 beforeIndex」返回消息分页。
//
// 设计说明：早期实现用消息 ID 做游标，但历史会话里存在 id 为空的消息
// （部分写入路径直接用 struct literal 构造），游标会失效。
// 因此改为基于绝对索引的分页，对旧数据同样有效。
//
//   - limit <= 0：返回 [0, beforeIndex) 的全部消息
//   - beforeIndex < 0 或 0：表示从末尾开始
//
// 返回：本页消息、是否还有更早的消息、下一页游标（本页首条的绝对索引）。
func PaginateMessages(messages []Message, limit int, beforeIndex int) (page []Message, hasMore bool, nextBefore int) {
	total := len(messages)
	if total == 0 {
		return nil, false, -1
	}

	end := total
	if beforeIndex > 0 && beforeIndex < total {
		end = beforeIndex
	}
	if end <= 0 {
		return nil, false, -1
	}
	if limit <= 0 || limit > end {
		limit = end
	}

	start := end - limit
	page = make([]Message, end-start)
	copy(page, messages[start:end])

	hasMore = start > 0
	if hasMore {
		nextBefore = start
	} else {
		nextBefore = -1
	}
	return page, hasMore, nextBefore
}
