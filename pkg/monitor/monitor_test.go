package monitor

import (
	"strings"
	"testing"
	"time"
)

func seedConsole(m *Monitor) {
	m.AddConsole("log", []interface{}{"event", map[string]interface{}{
		"type": "credit:balance:changed",
		"data": map[string]interface{}{"balance": float64(488)},
	}})
	m.AddConsole("warn", []interface{}{"slow response"})
	m.AddConsole("error", []interface{}{"boom"})
}

func TestConsolePatternAndLevelFilter(t *testing.T) {
	m := New()
	seedConsole(m)

	entries, err := m.Console(ConsoleFilter{Pattern: "credit"})
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 {
		t.Fatalf("expected 1 match, got %d", len(entries))
	}

	entries, err = m.Console(ConsoleFilter{Levels: []string{"warn", "error"}})
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 2 {
		t.Fatalf("expected 2 matches, got %d", len(entries))
	}

	if _, err := m.Console(ConsoleFilter{Pattern: "("}); err == nil {
		t.Fatal("expected error for invalid regex")
	}
}

func TestConsoleObjectsAreExpanded(t *testing.T) {
	m := New()
	seedConsole(m)

	entries, _ := m.Console(ConsoleFilter{Pattern: "credit"})
	rendered := RenderArgs(entries[0].Args, 0)

	if !strings.Contains(rendered, `"balance":488`) {
		t.Fatalf("nested value missing from %q", rendered)
	}
}

func TestConsoleExpandDepthTrims(t *testing.T) {
	m := New()
	seedConsole(m)

	entries, _ := m.Console(ConsoleFilter{Pattern: "credit", ExpandDepth: 1})
	rendered := RenderArgs(entries[0].Args, 0)

	if strings.Contains(rendered, "balance") {
		t.Fatalf("expected depth 1 to collapse nested object, got %q", rendered)
	}
	if !strings.Contains(rendered, "{…}") {
		t.Fatalf("expected collapse marker, got %q", rendered)
	}
}

func TestConsoleMaxResultsKeepsNewest(t *testing.T) {
	m := New()
	seedConsole(m)

	entries, _ := m.Console(ConsoleFilter{MaxResults: 1})
	if len(entries) != 1 || RenderArgs(entries[0].Args, 0) != "boom" {
		t.Fatalf("expected newest entry, got %v", entries)
	}
}

func TestConsoleSinceMs(t *testing.T) {
	m := New()
	m.AddConsole("log", []interface{}{"old"})
	m.console[0].Time = time.Now().Add(-5 * time.Second)
	m.AddConsole("log", []interface{}{"new"})

	entries, _ := m.Console(ConsoleFilter{SinceMs: 1000})
	if len(entries) != 1 || RenderArgs(entries[0].Args, 0) != "new" {
		t.Fatalf("expected only the recent entry, got %v", entries)
	}
}

func TestRequestFiltering(t *testing.T) {
	m := New()
	m.RequestStarted("1", "http://app/api/chat", "POST")
	m.ResponseReceived("1", 200, "application/json")
	m.RequestFinished("1", 120)
	m.SetBody("1", `{"ok":true}`)

	m.RequestStarted("2", "http://app/static/main.css", "GET")
	m.ResponseReceived("2", 200, "text/css")
	m.RequestFinished("2", 4000)

	m.RequestStarted("3", "http://app/api/missing", "GET")
	m.ResponseReceived("3", 404, "application/json")
	m.RequestFinished("3", 30)

	got, err := m.Requests(RequestFilter{URLPattern: "/api/"})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 {
		t.Fatalf("expected 2 api requests, got %d", len(got))
	}

	got, _ = m.Requests(RequestFilter{Method: "post"})
	if len(got) != 1 || got[0].URL != "http://app/api/chat" {
		t.Fatalf("expected the POST, got %v", got)
	}

	got, _ = m.Requests(RequestFilter{Status: []int{404}})
	if len(got) != 1 || got[0].Status != 404 {
		t.Fatalf("expected the 404, got %v", got)
	}

	got, _ = m.Requests(RequestFilter{URLPattern: "chat"})
	if got[0].Body != "" {
		t.Fatal("body should be omitted unless requested")
	}

	got, _ = m.Requests(RequestFilter{URLPattern: "chat", IncludeBody: true})
	if got[0].Body != `{"ok":true}` {
		t.Fatalf("expected body, got %q", got[0].Body)
	}
}

func TestNetworkIdle(t *testing.T) {
	m := New()
	m.RequestStarted("1", "http://app/api/slow", "GET")

	if m.NetworkIdle(0) {
		t.Fatal("should not be idle while a request is in flight")
	}

	m.RequestFinished("1", 10)
	if !m.NetworkIdle(0) {
		t.Fatal("should be idle once the request finished")
	}
	if m.NetworkIdle(5000) {
		t.Fatal("should not be idle for a 5s quiet period right after activity")
	}
}

func TestRingBufferEvictsOldest(t *testing.T) {
	m := New()
	for i := 0; i < maxConsoleEntries+10; i++ {
		m.AddConsole("log", []interface{}{"msg"})
	}
	if len(m.console) != maxConsoleEntries {
		t.Fatalf("expected buffer capped at %d, got %d", maxConsoleEntries, len(m.console))
	}

	for i := 0; i < maxRequests+5; i++ {
		m.RequestStarted(string(rune('a'+i%26))+string(rune(i)), "http://app/x", "GET")
	}
	if len(m.requests) != maxRequests {
		t.Fatalf("expected buffer capped at %d, got %d", maxRequests, len(m.requests))
	}
	if len(m.byID) > maxRequests {
		t.Fatalf("evicted requests left behind in the index: %d entries", len(m.byID))
	}
}
