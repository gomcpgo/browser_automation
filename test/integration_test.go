package test

import (
	"context"
	"encoding/json"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/go-rod/rod/lib/launcher"

	mcpHandler "github.com/gomcpgo/browser_automation/pkg/handler"
	"github.com/gomcpgo/browser_automation/test/fixtures"
	"github.com/gomcpgo/mcp/pkg/protocol"
)

// setup starts the fixture server and a browser session pointed at it.
// It skips the test when Chrome is not installed.
func setup(t *testing.T) (*mcpHandler.Handler, string) {
	t.Helper()

	if _, found := launcher.LookPath(); !found {
		t.Skip("Chrome not installed; skipping browser integration tests")
	}

	srv := httptest.NewServer(fixtures.Handler())
	t.Cleanup(srv.Close)

	h := mcpHandler.New()
	t.Cleanup(h.Shutdown)

	call(t, h, "start_session", map[string]interface{}{
		"width": float64(1200), "height": float64(800),
	})
	call(t, h, "navigate", map[string]interface{}{"url": srv.URL})

	return h, srv.URL
}

func call(t *testing.T, h *mcpHandler.Handler, tool string, args map[string]interface{}) string {
	t.Helper()

	resp, err := h.CallTool(context.Background(), &protocol.CallToolRequest{Name: tool, Arguments: args})
	if err != nil {
		t.Fatalf("%s returned an error: %v", tool, err)
	}
	text := ""
	for _, c := range resp.Content {
		text += c.Text
	}
	if resp.IsError {
		t.Fatalf("%s failed: %s", tool, text)
	}
	return text
}

func callExpectingError(t *testing.T, h *mcpHandler.Handler, tool string, args map[string]interface{}) string {
	t.Helper()

	resp, err := h.CallTool(context.Background(), &protocol.CallToolRequest{Name: tool, Arguments: args})
	if err != nil {
		return err.Error()
	}
	text := ""
	for _, c := range resp.Content {
		text += c.Text
	}
	if !resp.IsError {
		t.Fatalf("expected %s to fail, got: %s", tool, text)
	}
	return text
}

func decode(t *testing.T, text string) map[string]interface{} {
	t.Helper()

	var m map[string]interface{}
	if err := json.Unmarshal([]byte(text), &m); err != nil {
		t.Fatalf("expected JSON, got %s", text)
	}
	return m
}

// TestWaitBeatsFixedTimeout covers the "sleep 45" pain point: the wait returns as
// soon as the delayed text lands, well before a generous timeout.
func TestWaitBeatsFixedTimeout(t *testing.T) {
	h, _ := setup(t)

	out := call(t, h, "wait_for", map[string]interface{}{
		"condition": "text_visible", "text": "Ready", "timeout_ms": float64(45000),
	})
	res := decode(t, out)

	elapsed, _ := res["elapsed_ms"].(float64)
	if elapsed > 10000 {
		t.Fatalf("expected an early return, waited %vms", elapsed)
	}

	call(t, h, "wait_for", map[string]interface{}{
		"condition": "text_gone", "text": "Loading...", "timeout_ms": float64(5000),
	})
}

func TestWaitTimeoutReportsElapsed(t *testing.T) {
	h, _ := setup(t)

	msg := callExpectingError(t, h, "wait_for", map[string]interface{}{
		"condition": "selector_visible", "selector": "#never-appears", "timeout_ms": float64(500),
	})
	if !strings.Contains(msg, "timed out") || !strings.Contains(msg, "elapsed") {
		t.Fatalf("unexpected timeout message: %s", msg)
	}
}

// TestConsoleObjectExpansion covers the "{data: Object}" pain point.
func TestConsoleObjectExpansion(t *testing.T) {
	h, _ := setup(t)

	out := call(t, h, "get_console", map[string]interface{}{"pattern": "credit"})
	if !strings.Contains(out, `"balance":488`) {
		t.Fatalf("nested object not expanded: %s", out)
	}
	if !strings.Contains(out, "credit:balance:changed") {
		t.Fatalf("expected the logged event type: %s", out)
	}

	// Filtering must actually narrow the output, so log a second message first.
	call(t, h, "wait_for", map[string]interface{}{
		"condition": "console_matches", "pattern": "status changed", "timeout_ms": float64(10000),
	})
	all := call(t, h, "get_console", map[string]interface{}{})
	if len(strings.Split(strings.TrimSpace(all), "\n")) <= len(strings.Split(strings.TrimSpace(out), "\n")) {
		t.Fatalf("pattern filter did not narrow results:\nall:\n%s\nfiltered:\n%s", all, out)
	}
}

// TestRequestFilteringWithBody covers the unfiltered-network-dump pain point.
func TestRequestFilteringWithBody(t *testing.T) {
	h, _ := setup(t)

	call(t, h, "click", map[string]interface{}{"selector": "#fetch-btn"})
	call(t, h, "wait_for", map[string]interface{}{
		"condition": "text_visible", "text": "got 2 items", "timeout_ms": float64(10000),
	})

	out := call(t, h, "get_requests", map[string]interface{}{
		"url_pattern": "/api/test", "include_body": true,
	})
	if !strings.Contains(out, "/api/test") {
		t.Fatalf("expected the API call: %s", out)
	}
	if !strings.Contains(out, "alpha") {
		t.Fatalf("expected the response body: %s", out)
	}
	if strings.Contains(out, "frame.html") {
		t.Fatalf("url_pattern did not exclude other requests: %s", out)
	}
}

// TestSnapshotIsCompactAndSelectorsWork covers the 3000-token-snapshot pain point.
func TestSnapshotIsCompactAndSelectorsWork(t *testing.T) {
	h, _ := setup(t)

	out := call(t, h, "snapshot", map[string]interface{}{})
	if len(out) > 4000 {
		t.Fatalf("snapshot is not compact: %d bytes", len(out))
	}
	if !strings.Contains(out, "#toggle-dropdown") {
		t.Fatalf("expected a selector for the toggle button: %s", out)
	}

	// Every suggested selector must actually resolve.
	for _, line := range strings.Split(strings.TrimSpace(out), "\n") {
		_, sel, found := strings.Cut(line, "→ ")
		if !found {
			t.Fatalf("line without a selector: %q", line)
		}
		sel = strings.TrimSpace(sel)
		res := decode(t, call(t, h, "get_element", map[string]interface{}{"selector": sel}))
		if count, _ := res["match_count"].(float64); count < 1 {
			t.Fatalf("suggested selector %q matched nothing", sel)
		}
	}

	interactiveOnly := call(t, h, "snapshot", map[string]interface{}{"interactive_only": true})
	if len(interactiveOnly) >= len(out) {
		t.Fatalf("interactive_only did not shrink the outline")
	}
	if strings.Contains(interactiveOnly, "#title") {
		t.Fatalf("interactive_only should not list the heading: %s", interactiveOnly)
	}
}

// TestHiddenElementReportsReason covers the "Export button mystery" pain point.
func TestHiddenElementReportsReason(t *testing.T) {
	h, _ := setup(t)

	res := decode(t, call(t, h, "get_element", map[string]interface{}{
		"selector": "#dropdown", "include_styles": []interface{}{"border-style"},
	}))
	if visible, _ := res["visible"].(bool); visible {
		t.Fatal("dropdown should start hidden")
	}
	if res["hidden_by"] != "display: none" {
		t.Fatalf("expected display: none, got %v", res["hidden_by"])
	}
	styles, _ := res["styles"].(map[string]interface{})
	if styles["display"] != "none" || styles["border-style"] != "solid" {
		t.Fatalf("unexpected styles: %v", styles)
	}

	call(t, h, "click", map[string]interface{}{"selector": "#toggle-dropdown"})
	call(t, h, "wait_for", map[string]interface{}{
		"condition": "selector_visible", "selector": "#dropdown", "timeout_ms": float64(3000),
	})

	res = decode(t, call(t, h, "get_element", map[string]interface{}{"selector": "#dropdown"}))
	if visible, _ := res["visible"].(bool); !visible {
		t.Fatalf("dropdown should be visible after the toggle: %v", res)
	}
}

func TestTypeTextAndEvaluate(t *testing.T) {
	h, _ := setup(t)

	call(t, h, "type_text", map[string]interface{}{"selector": "#name-input", "text": "Ada"})
	res := decode(t, call(t, h, "evaluate", map[string]interface{}{
		"js": "document.getElementById('name-input').value",
	}))
	if res["value"] != "Ada" {
		t.Fatalf("expected the typed value, got %v", res["value"])
	}

	call(t, h, "type_text", map[string]interface{}{
		"selector": "#name-input", "text": "Grace", "clear": true,
	})
	res = decode(t, call(t, h, "evaluate", map[string]interface{}{
		"js": "document.getElementById('name-input').value",
	}))
	if res["value"] != "Grace" {
		t.Fatalf("clear did not replace the old value, got %v", res["value"])
	}
}

// TestScreenshotToAbsolutePath covers the .playwright-mcp sandbox pain point.
func TestScreenshotToAbsolutePath(t *testing.T) {
	h, _ := setup(t)

	out := filepath.Join(t.TempDir(), "nested", "shot.png")
	res := decode(t, call(t, h, "screenshot", map[string]interface{}{"output_path": out}))

	if res["path"] != out {
		t.Fatalf("expected the exact path back, got %v", res["path"])
	}
	if _, err := os.Stat(out); err != nil {
		t.Fatalf("screenshot not written: %v", err)
	}

	msg := callExpectingError(t, h, "screenshot", map[string]interface{}{"output_path": "shot.png"})
	if !strings.Contains(msg, "absolute") {
		t.Fatalf("expected an absolute-path error, got %s", msg)
	}
}

// TestScrollAndElementScreenshot covers capture of an element below the fold.
func TestScrollAndElementScreenshot(t *testing.T) {
	h, _ := setup(t)

	call(t, h, "scroll", map[string]interface{}{"selector": "#tall-heading"})

	out := filepath.Join(t.TempDir(), "tall.png")
	res := decode(t, call(t, h, "screenshot", map[string]interface{}{
		"output_path": out, "selector": "#tall-heading",
	}))
	if w, _ := res["width"].(float64); w <= 0 {
		t.Fatalf("expected a non-empty element capture: %v", res)
	}
}

// TestZoomLeverProducesSharperCapture covers the sharpness finding: page zoom,
// not device pixel ratio, is what re-renders at a higher scale.
func TestZoomLeverProducesSharperCapture(t *testing.T) {
	h, _ := setup(t)
	dir := t.TempDir()

	base := decode(t, call(t, h, "screenshot", map[string]interface{}{
		"output_path": filepath.Join(dir, "detail-1x.png"), "selector": "#detail",
	}))
	zoomed := decode(t, call(t, h, "screenshot", map[string]interface{}{
		"output_path": filepath.Join(dir, "detail-2x.png"), "selector": "#detail", "zoom": float64(2),
	}))

	w1, _ := base["width"].(float64)
	w2, _ := zoomed["width"].(float64)
	if w1 <= 0 || w2 < w1*1.8 || w2 > w1*2.2 {
		t.Fatalf("expected the 2x capture to be about twice as wide: %v vs %v", w1, w2)
	}

	// The session zoom must be restored after a per-shot zoom.
	after := decode(t, call(t, h, "screenshot", map[string]interface{}{
		"output_path": filepath.Join(dir, "detail-after.png"), "selector": "#detail",
	}))
	if w3, _ := after["width"].(float64); w3 != w1 {
		t.Fatalf("session zoom not restored: %v vs %v", w1, w3)
	}
}

func TestRectCapture(t *testing.T) {
	h, _ := setup(t)

	out := filepath.Join(t.TempDir(), "rect.png")
	res := decode(t, call(t, h, "screenshot", map[string]interface{}{
		"output_path": out,
		"rect":        map[string]interface{}{"x": float64(0), "y": float64(0), "width": float64(300), "height": float64(150)},
	}))
	if res["width"] != float64(300) || res["height"] != float64(150) {
		t.Fatalf("expected a 300x150 crop, got %vx%v", res["width"], res["height"])
	}

	msg := callExpectingError(t, h, "screenshot", map[string]interface{}{
		"output_path": out,
		"selector":    "#detail",
		"rect":        map[string]interface{}{"width": float64(10), "height": float64(10)},
	})
	if !strings.Contains(msg, "not both") {
		t.Fatalf("expected mutual-exclusion error, got %s", msg)
	}
}

// TestIframeBehaviour pins down what v1 does with iframes: the frame element is
// addressable, its contents are not. Full frame support is a later phase.
func TestIframeBehaviour(t *testing.T) {
	h, _ := setup(t)

	res := decode(t, call(t, h, "get_element", map[string]interface{}{"selector": "#inner"}))
	if res["tag"] != "iframe" {
		t.Fatalf("expected the iframe element, got %v", res["tag"])
	}

	msg := callExpectingError(t, h, "get_element", map[string]interface{}{"selector": "#frame-text"})
	if !strings.Contains(msg, "no element matches") {
		t.Fatalf("unexpected error for in-frame selector: %s", msg)
	}
}

func TestToolsRequireSession(t *testing.T) {
	if _, found := launcher.LookPath(); !found {
		t.Skip("Chrome not installed; skipping browser integration tests")
	}

	h := mcpHandler.New()
	msg := callExpectingError(t, h, "snapshot", map[string]interface{}{})
	if !strings.Contains(msg, "start_session") {
		t.Fatalf("expected a start_session hint, got %s", msg)
	}
}
