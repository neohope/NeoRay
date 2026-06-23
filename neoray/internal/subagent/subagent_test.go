package subagent

import (
	"testing"
)

// TestSubagentStatus tests the SubagentStatus struct
func TestSubagentStatus(t *testing.T) {
	status := NewSubagentStatus("test-123", "Test Task", "Do something")
	if status.TaskID != "test-123" {
		t.Errorf("Expected task ID 'test-123', got '%s'", status.TaskID)
	}
	if status.Label != "Test Task" {
		t.Errorf("Expected label 'Test Task', got '%s'", status.Label)
	}
	if status.Phase != PhaseInitializing {
		t.Errorf("Expected phase %v, got %v", PhaseInitializing, status.Phase)
	}

	status.SetPhase(PhaseAwaitingTools)
	if status.GetPhase() != PhaseAwaitingTools {
		t.Errorf("Expected phase %v after SetPhase, got %v", PhaseAwaitingTools, status.GetPhase())
	}

	status.SetIteration(5)
	if status.Iteration != 5 {
		t.Errorf("Expected iteration 5, got %d", status.Iteration)
	}

	status.AddToolEvent("read_file", "ok", "Read file.txt")
	events := status.GetToolEvents()
	if len(events) != 1 {
		t.Errorf("Expected 1 tool event, got %d", len(events))
	}
}

// TestSubagentStatusIsRunning tests the IsRunning method
func TestSubagentStatusIsRunning(t *testing.T) {
	status := NewSubagentStatus("test-123", "Test", "Do something")
	if !status.IsRunning() {
		t.Error("Expected status to be running initially")
	}

	status.SetPhase(PhaseDone)
	if status.IsRunning() {
		t.Error("Expected status to not be running after PhaseDone")
	}

	status2 := NewSubagentStatus("test-456", "Test", "Do something")
	status2.SetPhase(PhaseError)
	if status2.IsRunning() {
		t.Error("Expected status to not be running after PhaseError")
	}
}
