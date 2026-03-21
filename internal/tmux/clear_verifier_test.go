package tmux

import (
	"errors"
	"testing"
	"time"
)

// TestWaitForIdleAfterClear_SettlesBeforePolling verifies that the settle
// delay is respected before idle polling begins.
func TestWaitForIdleAfterClear_SettlesBeforePolling(t *testing.T) {
	tm := newTestTmux(t)
	sessionName := "gt-test-clear-settle"

	_ = tm.KillSession(sessionName)
	if err := tm.NewSession(sessionName, ""); err != nil {
		t.Fatalf("NewSession: %v", err)
	}
	defer func() { _ = tm.KillSession(sessionName) }()

	start := time.Now()
	settleDelay := 300 * time.Millisecond
	// WaitForIdle will timeout since shell doesn't have ❯ prompt, but the
	// total elapsed time must include the settle delay. We use a short idle
	// timeout to keep the test fast.
	_ = tm.WaitForIdleAfterClear(sessionName, settleDelay, 500*time.Millisecond)
	elapsed := time.Since(start)

	// Must take at least the settle delay + idle timeout
	if elapsed < settleDelay {
		t.Errorf("completed in %v, expected at least %v settle delay", elapsed, settleDelay)
	}
}

// TestVerifyNudgeDelivery_BusyAgentReturnsNil verifies that when the agent
// is busy processing (not at prompt), verification succeeds immediately.
func TestVerifyNudgeDelivery_BusyAgentReturnsNil(t *testing.T) {
	tm := newTestTmux(t)
	sessionName := "gt-test-verify-busy"

	_ = tm.KillSession(sessionName)
	// Start a session running a long command (simulates busy agent)
	if err := tm.NewSessionWithCommand(sessionName, "", "sleep 60"); err != nil {
		t.Fatalf("NewSessionWithCommand: %v", err)
	}
	defer func() { _ = tm.KillSession(sessionName) }()

	time.Sleep(500 * time.Millisecond) // let sleep start

	// Agent is "busy" (running sleep, no prompt visible)
	err := tm.VerifyNudgeDelivery(sessionName, "test message", "> ", 200*time.Millisecond, 1)
	if err != nil {
		t.Fatalf("VerifyNudgeDelivery should succeed for busy agent: %v", err)
	}
}

// TestVerifyNudgeDelivery_IdleAgentRetries verifies that when the agent is
// idle at prompt, the function retries the nudge up to maxRetries times.
func TestVerifyNudgeDelivery_IdleAgentRetries(t *testing.T) {
	tm := newTestTmux(t)
	sessionName := "gt-test-verify-idle"

	_ = tm.KillSession(sessionName)
	if err := tm.NewSessionWithCommand(sessionName, "", `PS1="> " bash --norc --noprofile`); err != nil {
		t.Fatalf("NewSessionWithCommand: %v", err)
	}
	defer func() { _ = tm.KillSession(sessionName) }()

	time.Sleep(500 * time.Millisecond) // let shell start

	// Shell is at prompt "> " — this simulates an idle agent
	start := time.Now()
	err := tm.VerifyNudgeDelivery(sessionName, "echo test", "> ", 300*time.Millisecond, 2)
	elapsed := time.Since(start)

	// Should have attempted retries (taking at least 2 * verifyDelay)
	if elapsed < 600*time.Millisecond {
		t.Errorf("completed in %v, expected at least 600ms for 2 retries", elapsed)
	}

	// It's acceptable for this to succeed (shell might process the nudge)
	// or fail (shell stays at prompt). The key test is the retry timing.
	if err != nil && !errors.Is(err, ErrNudgeNotDelivered) {
		t.Errorf("unexpected error: %v (expected nil or ErrNudgeNotDelivered)", err)
	}
}

// TestVerifyNudgeDelivery_DeadSession verifies error on dead session.
func TestVerifyNudgeDelivery_DeadSession(t *testing.T) {
	tm := newTestTmux(t)

	err := tm.VerifyNudgeDelivery("gt-nonexistent-session-xyz", "test", "> ", 100*time.Millisecond, 1)
	if err == nil {
		t.Fatal("expected error for dead session")
	}
}
