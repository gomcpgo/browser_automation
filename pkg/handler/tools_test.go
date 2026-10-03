package handler

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/gomcpgo/browser_automation/pkg/browser"
	"github.com/gomcpgo/browser_automation/pkg/snapshot"
	"github.com/gomcpgo/mcp/pkg/protocol"
)

func TestToolsExclude(t *testing.T) {
	h := New("")
	all := h.Tools()
	filtered := h.Tools("start_session", "close_session")

	if len(filtered) != len(all)-2 {
		t.Fatalf("expected %d tools after excluding two, got %d", len(all)-2, len(filtered))
	}
	for _, tool := range filtered {
		if tool.Name == "start_session" || tool.Name == "close_session" {
			t.Fatalf("%s should have been excluded", tool.Name)
		}
	}

	names := map[string]bool{}
	for _, tool := range all {
		names[tool.Name] = true
	}
	for _, want := range []string{"insert_text", "find", "type_text", "screenshot"} {
		if !names[want] {
			t.Fatalf("tool %s missing from the list", want)
		}
	}
}

func TestExtractFindParams(t *testing.T) {
	p, err := extractFindParams(map[string]interface{}{
		"text": "Daily", "role": "listitem", "max_results": float64(3), "visible_only": false,
	})
	if err != nil {
		t.Fatal(err)
	}
	if p.Text != "Daily" || p.Role != "listitem" || p.MaxResults != 3 || p.VisibleOnly {
		t.Fatalf("unexpected params: %+v", p)
	}

	p, err = extractFindParams(map[string]interface{}{"aria_label": "Updates"})
	if err != nil {
		t.Fatal(err)
	}
	if !p.VisibleOnly || p.MaxResults != 10 {
		t.Fatalf("defaults not applied: %+v", p)
	}

	if _, err := extractFindParams(map[string]interface{}{"within": "#x"}); err == nil {
		t.Fatal("expected an error when no search criterion is given")
	}
	if _, err := extractFindParams(map[string]interface{}{"text": 5}); err == nil {
		t.Fatal("expected a type error for text")
	}
	if err := (snapshot.FindParams{}).Validate(); err == nil {
		t.Fatal("empty params must not validate")
	}
}

type fakeSource struct{ calls int }

func (f *fakeSource) Current() (*browser.Session, error) {
	f.calls++
	return nil, errors.New("no session; call start_session with a profile")
}

// A handler built from a source asks the source for every tool call and
// refuses to manage sessions itself.
func TestSourceBackedHandler(t *testing.T) {
	src := &fakeSource{}
	h := NewWithSource(src)
	ctx := context.Background()

	resp, err := h.CallTool(ctx, &protocol.CallToolRequest{Name: "snapshot", Arguments: map[string]interface{}{}})
	if err != nil {
		t.Fatal(err)
	}
	if !resp.IsError || !strings.Contains(resp.Content[0].Text, "start_session with a profile") {
		t.Fatalf("expected the source's error, got %+v", resp)
	}
	if src.calls != 1 {
		t.Fatalf("source should have been asked once, got %d", src.calls)
	}

	for _, name := range []string{"start_session", "close_session"} {
		resp, err := h.CallTool(ctx, &protocol.CallToolRequest{Name: name, Arguments: map[string]interface{}{}})
		if err != nil {
			t.Fatal(err)
		}
		if !resp.IsError || !strings.Contains(resp.Content[0].Text, "host server") {
			t.Fatalf("%s should be refused on a source-backed handler, got %+v", name, resp)
		}
	}

	// Shutdown must not panic or touch anything without an owned session.
	h.Shutdown()
}
