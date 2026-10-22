package session

import (
	"sync"
	"testing"
)

// TestGoalStateConcurrentAccess 回归：goal state 写 metadata 必须持 session 锁，
// 否则与 DeepCopy/列表接口并发时是 concurrent map read and map write
// （Go 会 fatal error，无法 recover）。
func TestGoalStateConcurrentAccess(t *testing.T) {
	sess := NewSession("test", "u1")
	sess.ID = "test:u1"
	gm := NewGoalManager(nil, nil)

	var wg sync.WaitGroup
	wg.Add(2)

	go func() {
		defer wg.Done()
		for i := 0; i < 300; i++ {
			_ = gm.SetGoalState(sess, &GoalState{Status: GoalStatusActive, Objective: "x"})
		}
	}()

	go func() {
		defer wg.Done()
		for i := 0; i < 300; i++ {
			_ = sess.DeepCopy()
			_, _ = gm.GetGoalState(sess)
		}
	}()

	wg.Wait()
}
