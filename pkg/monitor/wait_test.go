package monitor

import (
	"context"
	"strings"
	"testing"
	"time"
)

func TestWaitReturnsEarlyWhenConditionIsMet(t *testing.T) {
	m := New()
	start := time.Now()
	eval := func(js string) (bool, error) {
		return time.Since(start) > 300*time.Millisecond, nil
	}

	elapsed, err := Wait(context.Background(), m, eval, WaitParams{
		Condition: "selector_visible", Selector: "#done", TimeoutMs: 45000,
	})
	if err != nil {
		t.Fatal(err)
	}
	if elapsed > 2000 {
		t.Fatalf("expected an early return, waited %dms", elapsed)
	}
}

func TestWaitTimesOut(t *testing.T) {
	m := New()
	eval := func(js string) (bool, error) { return false, nil }

	_, err := Wait(context.Background(), m, eval, WaitParams{
		Condition: "text_visible", Text: "Ready", TimeoutMs: 300,
	})
	if err == nil || !strings.Contains(err.Error(), "timed out") {
		t.Fatalf("expected timeout error, got %v", err)
	}
}

func TestWaitConsoleMatchesSeesHistory(t *testing.T) {
	m := New()
	m.AddConsole("log", []interface{}{"credit:balance:changed"})
	eval := func(js string) (bool, error) { return false, nil }

	elapsed, err := Wait(context.Background(), m, eval, WaitParams{
		Condition: "console_matches", Pattern: "balance", TimeoutMs: 1000,
	})
	if err != nil {
		t.Fatalf("expected the buffered message to satisfy the wait: %v", err)
	}
	if elapsed > 200 {
		t.Fatalf("expected an immediate match, waited %dms", elapsed)
	}
}

func TestWaitRejectsBadParams(t *testing.T) {
	m := New()
	eval := func(js string) (bool, error) { return true, nil }

	cases := []WaitParams{
		{Condition: "text_visible", TimeoutMs: 100},
		{Condition: "selector_visible", TimeoutMs: 100},
		{Condition: "console_matches", TimeoutMs: 100},
		{Condition: "js_true", TimeoutMs: 100},
		{Condition: "nonsense", TimeoutMs: 100},
	}
	for _, p := range cases {
		if _, err := Wait(context.Background(), m, eval, p); err == nil {
			t.Fatalf("expected an error for %+v", p)
		}
	}
}
