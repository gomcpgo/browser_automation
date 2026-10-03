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
	return setupZoom(t, 1)
}

// setupZoom is setup with an explicit session zoom.
func setupZoom(t *testing.T, zoom float64) (*mcpHandler.Handler, string) {
	t.Helper()

	if _, found := launcher.LookPath(); !found {
		t.Skip("Chrome not installed; skipping browser integration tests")
	}

	srv := httptest.NewServer(fixtures.Handler())
	t.Cleanup(srv.Close)

	h := mcpHandler.New("")
	t.Cleanup(h.Shutdown)

	call(t, h, "start_session", map[string]interface{}{
		"width": float64(1200), "height": float64(800), "zoom": zoom,
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

// TestDialogDoesNotStallSession covers caveat 1: an unhandled alert used to
// block the page and every later tool call, including wait_for's own timeout.
func TestDialogDoesNotStallSession(t *testing.T) {
	h, _ := setup(t)

	call(t, h, "click", map[string]interface{}{"selector": "#alert-btn"})

	// The dialog is dismissed, so the click handler runs to completion...
	call(t, h, "wait_for", map[string]interface{}{
		"condition": "text_visible", "text": "alert closed", "timeout_ms": float64(10000),
	})

	// ...later calls still work...
	res := decode(t, call(t, h, "evaluate", map[string]interface{}{"js": "1 + 1"}))
	if res["value"] != float64(2) {
		t.Fatalf("expected the session to keep working after a dialog, got %v", res["value"])
	}

	// ...and the dismissed dialog is reported rather than swallowed.
	out := call(t, h, "get_console", map[string]interface{}{"pattern": "dialog"})
	if !strings.Contains(out, "blocking dialog") {
		t.Fatalf("expected the dialog message in the console buffer: %s", out)
	}
}

// TestZoomSurvivesPageInitiatedReload covers caveat 2: the page dropping the
// zoom on its own reload used to yield a silent 1x capture.
func TestZoomSurvivesPageInitiatedReload(t *testing.T) {
	// Session zoom 2 with no per-shot zoom is the case that used to break: the
	// shot zoom matched the session value, so re-applying it looked unnecessary.
	h, _ := setupZoom(t, 2)
	dir := t.TempDir()

	before := decode(t, call(t, h, "screenshot", map[string]interface{}{
		"output_path": filepath.Join(dir, "before.png"), "selector": "#detail",
	}))
	if w, _ := before["width"].(float64); w != 400 {
		t.Fatalf("expected the 200px element captured at 2x, got %v", w)
	}

	call(t, h, "click", map[string]interface{}{"selector": "#reload-btn"})
	call(t, h, "wait_for", map[string]interface{}{
		"condition": "text_visible", "text": "Ready", "timeout_ms": float64(15000),
	})

	after := decode(t, call(t, h, "screenshot", map[string]interface{}{
		"output_path": filepath.Join(dir, "after.png"), "selector": "#detail",
	}))
	if before["width"] != after["width"] {
		t.Fatalf("zoom lost across a page-initiated reload: %v then %v",
			before["width"], after["width"])
	}
}

func TestToolsRequireSession(t *testing.T) {
	if _, found := launcher.LookPath(); !found {
		t.Skip("Chrome not installed; skipping browser integration tests")
	}

	h := mcpHandler.New("")
	msg := callExpectingError(t, h, "snapshot", map[string]interface{}{})
	if !strings.Contains(msg, "start_session") {
		t.Fatalf("expected a start_session hint, got %s", msg)
	}
}

// TestInsertTextFillsWithoutSending covers chat composers where Enter submits:
// four lines go in as soft breaks, nothing is sent until an explicit Enter, and
// one Enter then sends exactly one message carrying all four lines.
func TestInsertTextFillsWithoutSending(t *testing.T) {
	h, _ := setup(t)

	verse := "ഓം ഗണപതയേ നമഃ\nശ്രീ ഹനുമതേ നമഃ\nബുദ്ധിർബലം യശോ ധൈര്യം\nനിർഭയത്വമരോഗതാ"
	res := decode(t, call(t, h, "insert_text", map[string]interface{}{
		"selector": "#composer", "text": verse,
	}))
	if n, _ := res["lines_inserted"].(float64); n != 4 {
		t.Fatalf("expected 4 lines inserted, got %v", res["lines_inserted"])
	}
	echo, _ := res["text"].(string)
	if len(strings.Split(strings.TrimSpace(echo), "\n")) != 4 {
		t.Fatalf("expected 4 lines in the composer, got %q", echo)
	}
	if !strings.Contains(echo, "ഹനുമതേ") {
		t.Fatalf("Malayalam text did not survive insertion: %q", echo)
	}

	sent := decode(t, call(t, h, "evaluate", map[string]interface{}{
		"js": "document.querySelectorAll('#sent li').length",
	}))
	if sent["value"] != float64(0) {
		t.Fatalf("insert_text must not send; %v messages were sent", sent["value"])
	}

	call(t, h, "press_key", map[string]interface{}{"key": "Enter"})
	sent = decode(t, call(t, h, "evaluate", map[string]interface{}{
		"js": "document.querySelectorAll('#sent li').length",
	}))
	if sent["value"] != float64(1) {
		t.Fatalf("expected exactly one message after Enter, got %v", sent["value"])
	}
	body := decode(t, call(t, h, "evaluate", map[string]interface{}{
		"js": "document.querySelector('#sent li').textContent",
	}))
	if got, _ := body["value"].(string); len(strings.Split(got, "\n")) != 4 || !strings.Contains(got, "ധൈര്യം") {
		t.Fatalf("sent message lost its lines: %q", got)
	}

	// clear replaces whatever is there; newline "none" flattens.
	call(t, h, "insert_text", map[string]interface{}{"selector": "#composer", "text": "old"})
	res = decode(t, call(t, h, "insert_text", map[string]interface{}{
		"selector": "#composer", "text": "a\nb", "clear": true, "newline": "none",
	}))
	if echo, _ := res["text"].(string); strings.TrimSpace(echo) != "a b" {
		t.Fatalf("expected cleared, flattened text, got %q", echo)
	}

	msg := callExpectingError(t, h, "insert_text", map[string]interface{}{
		"selector": "#composer", "text": "x", "newline": "tab",
	})
	if !strings.Contains(msg, "newline must be") {
		t.Fatalf("unexpected error for a bad newline mode: %s", msg)
	}
}

// TestFindByTextLabelAndRole covers locating elements on pages without stable
// ids or classes; every returned selector must resolve to exactly one element.
func TestFindByTextLabelAndRole(t *testing.T) {
	h, _ := setup(t)

	assertUnique := func(sel string) {
		t.Helper()
		res := decode(t, call(t, h, "get_element", map[string]interface{}{"selector": sel}))
		if count, _ := res["match_count"].(float64); count != 1 {
			t.Fatalf("selector %q matched %v elements", sel, count)
		}
	}

	byLabel := decode(t, call(t, h, "find", map[string]interface{}{"aria_label": "send message"}))
	matches, _ := byLabel["matches"].([]interface{})
	if len(matches) != 1 {
		t.Fatalf("expected one aria-label match, got %v", byLabel)
	}
	m := matches[0].(map[string]interface{})
	if m["tag"] != "button" || m["role"] != "button" || m["name"] != "Send message" {
		t.Fatalf("unexpected aria-label match: %v", m)
	}
	sel, _ := m["selector"].(string)
	if !strings.Contains(sel, `aria-label="Send message"`) {
		t.Fatalf("expected an aria-label based selector, got %q", sel)
	}
	assertUnique(sel)

	// Text search returns the innermost element, not every ancestor.
	byText := decode(t, call(t, h, "find", map[string]interface{}{"text": "toggle dropdown"}))
	matches, _ = byText["matches"].([]interface{})
	if len(matches) != 1 {
		t.Fatalf("expected one innermost text match, got %v", byText)
	}
	m = matches[0].(map[string]interface{})
	if m["selector"] != "#toggle-dropdown" {
		t.Fatalf("expected the button itself, got %v", m)
	}

	// Role, explicit and implicit, combined with another criterion.
	byRole := decode(t, call(t, h, "find", map[string]interface{}{"role": "textbox", "aria_label": "message"}))
	matches, _ = byRole["matches"].([]interface{})
	if len(matches) != 1 || matches[0].(map[string]interface{})["selector"] != "#composer" {
		t.Fatalf("expected the composer by role+label, got %v", byRole)
	}
	implicit := decode(t, call(t, h, "find", map[string]interface{}{"role": "textbox", "placeholder": "your name"}))
	matches, _ = implicit["matches"].([]interface{})
	if len(matches) != 1 || matches[0].(map[string]interface{})["selector"] != "#name-input" {
		t.Fatalf("expected the input by implicit role, got %v", implicit)
	}

	// Hidden elements are skipped unless asked for.
	hidden := call(t, h, "find", map[string]interface{}{"text": "Dropdown contents"})
	if !strings.Contains(hidden, "no elements matched") {
		t.Fatalf("hidden element should be skipped by default: %s", hidden)
	}
	shown := decode(t, call(t, h, "find", map[string]interface{}{"text": "Dropdown contents", "visible_only": false}))
	matches, _ = shown["matches"].([]interface{})
	if len(matches) != 1 || matches[0].(map[string]interface{})["visible"] != false {
		t.Fatalf("expected the hidden dropdown with visible=false, got %v", shown)
	}

	// Scoping and the max_results/total split.
	scoped := decode(t, call(t, h, "find", map[string]interface{}{"role": "button", "within": ".composer-wrap"}))
	if scoped["total"] != float64(1) {
		t.Fatalf("expected one button inside the composer wrapper, got %v", scoped)
	}
	limited := decode(t, call(t, h, "find", map[string]interface{}{"role": "button", "max_results": float64(2)}))
	matches, _ = limited["matches"].([]interface{})
	if total, _ := limited["total"].(float64); total < 3 || len(matches) != 2 {
		t.Fatalf("expected total >= 3 with 2 returned, got %v", limited)
	}
	for _, item := range matches {
		assertUnique(item.(map[string]interface{})["selector"].(string))
	}

	msg := callExpectingError(t, h, "find", map[string]interface{}{})
	if !strings.Contains(msg, "at least one") {
		t.Fatalf("unexpected error for empty criteria: %s", msg)
	}
}
