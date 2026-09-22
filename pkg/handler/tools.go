package handler

import (
	"encoding/json"

	"github.com/gomcpgo/mcp/pkg/protocol"
)

// GetTools returns the tool definitions exposed over MCP.
func (h *Handler) GetTools() []protocol.Tool {
	return []protocol.Tool{
		{
			Name: "start_session",
			Description: "Start a browser session. One browser and one page stay open across all " +
				"subsequent tool calls, and console/network capture begins immediately, so " +
				"get_console, get_requests and wait_for see the full history. Call this first. " +
				"Use zoom (not viewport size) to make screenshots sharper: zoom re-renders the page " +
				"at that scale, so a 2x zoom element screenshot has 2x the pixels. " +
				"JavaScript dialogs (alert/confirm/prompt) are dismissed automatically so they cannot " +
				"block the page; each one is reported in get_console at level warn.",
			InputSchema: json.RawMessage(`{
				"type": "object",
				"properties": {
					"width": {"type": "integer", "description": "Viewport width in CSS pixels (default 1280)"},
					"height": {"type": "integer", "description": "Viewport height in CSS pixels (default 800)"},
					"headed": {"type": "boolean", "description": "Show a real browser window instead of running headless (default false). Use when a human must click something, e.g. an approval gate."},
					"zoom": {"type": "number", "description": "Page zoom applied to every page and re-applied after navigation (default 1). 2 renders everything at 2x for sharp captures."}
				}
			}`),
		},
		{
			Name:        "close_session",
			Description: "Close the browser session and discard its console and network buffers.",
			InputSchema: json.RawMessage(`{"type": "object", "properties": {}}`),
		},
		{
			Name: "navigate",
			Description: "Navigate the session page to a URL and wait for the load event. Session " +
				"zoom is re-applied after the load.",
			InputSchema: json.RawMessage(`{
				"type": "object",
				"properties": {
					"url": {"type": "string", "description": "Absolute URL, e.g. http://localhost:3000/chat"}
				},
				"required": ["url"]
			}`),
		},
		{
			Name: "snapshot",
			Description: "Compact outline of the visible page: one line per meaningful element with a " +
				"CSS selector that is verified unique where possible. Far smaller than an " +
				"accessibility dump. Feed the printed selectors straight into click, type_text, " +
				"get_element or screenshot.",
			InputSchema: json.RawMessage(`{
				"type": "object",
				"properties": {
					"selector": {"type": "string", "description": "Scope the outline to this element's subtree (default: whole body)"},
					"max_depth": {"type": "integer", "description": "Maximum nesting levels to descend (default 12)"},
					"interactive_only": {"type": "boolean", "description": "List only links, buttons, inputs and other interactive elements (default false)"}
				}
			}`),
		},
		{
			Name: "get_element",
			Description: "Inspect one element: text, all attributes, bounds, whether it is actually " +
				"visible and what is hiding it (display:none, visibility, opacity, zero size or a " +
				"hidden ancestor), plus any computed styles you ask for. Use this when an element " +
				"is in the DOM but does not behave as expected.",
			InputSchema: json.RawMessage(`{
				"type": "object",
				"properties": {
					"selector": {"type": "string", "description": "CSS selector. If it matches several elements the first is used and match_count reports the total."},
					"include_styles": {"type": "array", "items": {"type": "string"}, "description": "Extra computed CSS properties to return, e.g. [\"z-index\", \"pointer-events\", \"overflow\"]. display, visibility and opacity are always included."}
				},
				"required": ["selector"]
			}`),
		},
		{
			Name:        "click",
			Description: "Click the first element matching the selector. Scrolls it into view first.",
			InputSchema: json.RawMessage(`{
				"type": "object",
				"properties": {
					"selector": {"type": "string", "description": "CSS selector of the element to click"}
				},
				"required": ["selector"]
			}`),
		},
		{
			Name:        "type_text",
			Description: "Type text into the first element matching the selector.",
			InputSchema: json.RawMessage(`{
				"type": "object",
				"properties": {
					"selector": {"type": "string", "description": "CSS selector of the input, textarea or contenteditable element"},
					"text": {"type": "string", "description": "Text to type"},
					"clear": {"type": "boolean", "description": "Clear existing content before typing (default false)"}
				},
				"required": ["selector", "text"]
			}`),
		},
		{
			Name: "press_key",
			Description: "Press a key or key combination on the focused element, e.g. \"Enter\", " +
				"\"Escape\", \"Meta+Enter\", \"Control+a\".",
			InputSchema: json.RawMessage(`{
				"type": "object",
				"properties": {
					"key": {"type": "string", "description": "Key name, optionally with modifiers joined by '+': Meta, Control, Shift, Alt plus a key such as Enter, Tab, Escape, ArrowDown or a single character"}
				},
				"required": ["key"]
			}`),
		},
		{
			Name:        "hover",
			Description: "Move the mouse over the first element matching the selector, to trigger hover states.",
			InputSchema: json.RawMessage(`{
				"type": "object",
				"properties": {
					"selector": {"type": "string", "description": "CSS selector of the element to hover"}
				},
				"required": ["selector"]
			}`),
		},
		{
			Name: "scroll",
			Description: "Scroll an element into view, or scroll the page by a pixel delta. Pass either " +
				"selector or x/y.",
			InputSchema: json.RawMessage(`{
				"type": "object",
				"properties": {
					"selector": {"type": "string", "description": "Scroll this element into view"},
					"x": {"type": "number", "description": "Horizontal scroll delta in pixels"},
					"y": {"type": "number", "description": "Vertical scroll delta in pixels (positive scrolls down)"}
				}
			}`),
		},
		{
			Name: "wait_for",
			Description: "Wait until a real condition holds, then return the elapsed time. Use this " +
				"instead of sleeping. Conditions: text_visible / text_gone (needs text), " +
				"selector_visible / selector_hidden (needs selector), console_matches (needs pattern; " +
				"searches buffered history too), network_idle (no in-flight requests for idle_ms), " +
				"js_true (needs js). Set timeout_ms as generously as the operation really needs.",
			InputSchema: json.RawMessage(`{
				"type": "object",
				"properties": {
					"condition": {"type": "string", "enum": ["text_visible", "text_gone", "selector_visible", "selector_hidden", "console_matches", "network_idle", "js_true"], "description": "Which condition to wait for"},
					"text": {"type": "string", "description": "For text_visible / text_gone: the text to look for in visible elements"},
					"selector": {"type": "string", "description": "For selector_visible / selector_hidden: the CSS selector"},
					"pattern": {"type": "string", "description": "For console_matches: a Go regular expression matched against rendered console messages"},
					"js": {"type": "string", "description": "For js_true: a JS expression evaluated in the page until it is truthy, e.g. \"document.querySelectorAll('.msg').length > 3\""},
					"idle_ms": {"type": "integer", "description": "For network_idle: quiet period required, in milliseconds (default 500)"},
					"since_ms": {"type": "integer", "description": "For console_matches: only consider messages from the last N milliseconds (default: whole session)"},
					"timeout_ms": {"type": "integer", "description": "Give up after this many milliseconds (default 10000). Long operations can use 60000+."}
				},
				"required": ["condition"]
			}`),
		},
		{
			Name: "get_console",
			Description: "Read buffered console output. Logged objects are captured fully serialized, " +
				"so nested payloads are readable instead of collapsing to \"Object\". Filter by " +
				"regex pattern, level and age to keep results small.",
			InputSchema: json.RawMessage(`{
				"type": "object",
				"properties": {
					"pattern": {"type": "string", "description": "Go regular expression matched against the rendered message"},
					"levels": {"type": "array", "items": {"type": "string"}, "description": "Levels to include: log, info, warn, error, debug"},
					"since_ms": {"type": "integer", "description": "Only messages from the last N milliseconds"},
					"max_results": {"type": "integer", "description": "Most recent N matches (default 50)"},
					"expand_depth": {"type": "integer", "description": "How deep to print nested objects before collapsing (default 5)"}
				}
			}`),
		},
		{
			Name: "get_requests",
			Description: "Read buffered network requests with status, timing and, on request, the " +
				"response body (captured at response time, so it survives navigation). Filter by URL " +
				"regex, method, status and age instead of dumping everything.",
			InputSchema: json.RawMessage(`{
				"type": "object",
				"properties": {
					"url_pattern": {"type": "string", "description": "Go regular expression matched against the request URL, e.g. \"/api/chat\""},
					"method": {"type": "string", "description": "HTTP method filter, e.g. POST"},
					"status": {"type": "array", "items": {"type": "integer"}, "description": "Only these response status codes"},
					"include_body": {"type": "boolean", "description": "Include captured response bodies (text responses up to 64KB)"},
					"since_ms": {"type": "integer", "description": "Only requests started in the last N milliseconds"},
					"max_results": {"type": "integer", "description": "Most recent N matches (default 20)"}
				}
			}`),
		},
		{
			Name: "evaluate",
			Description: "Evaluate a JavaScript expression in the page and return its JSON value. Use " +
				"it to read application state directly, e.g. app store contents, computed values or " +
				"element counts.",
			InputSchema: json.RawMessage(`{
				"type": "object",
				"properties": {
					"js": {"type": "string", "description": "JS expression, e.g. \"document.querySelectorAll('.message').length\" or \"window.__store.state\""}
				},
				"required": ["js"]
			}`),
		},
		{
			Name: "screenshot",
			Description: "Write a PNG to an exact absolute path. Capture the viewport, one element " +
				"(selector) or an explicit rect. Pass zoom to re-render at a higher scale for a sharp " +
				"crop: zoom 2 on an element gives roughly twice its CSS pixel dimensions. Returns the " +
				"path written and the image dimensions.",
			InputSchema: json.RawMessage(`{
				"type": "object",
				"properties": {
					"output_path": {"type": "string", "description": "Absolute path of the PNG to write. Parent directories are created."},
					"selector": {"type": "string", "description": "Capture just this element's box. Mutually exclusive with rect."},
					"rect": {
						"type": "object",
						"description": "Capture this region of the viewport, in CSS pixels. Mutually exclusive with selector.",
						"properties": {
							"x": {"type": "number"},
							"y": {"type": "number"},
							"width": {"type": "number"},
							"height": {"type": "number"}
						}
					},
					"zoom": {"type": "number", "description": "Render at this zoom for this shot only, then restore the session zoom. Layout reflows, so the element box changes too."}
				},
				"required": ["output_path"]
			}`),
		},
	}
}
