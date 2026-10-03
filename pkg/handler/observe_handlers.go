package handler

import (
	"context"
	"fmt"
	"strings"

	"github.com/gomcpgo/browser_automation/pkg/monitor"
	"github.com/gomcpgo/mcp/pkg/protocol"
)

// maxConsoleLine caps one rendered console entry; real apps log multi-kilobyte
// error objects that would otherwise swamp the response.
const maxConsoleLine = 4000

func (h *Handler) handleWaitFor(ctx context.Context, args map[string]interface{}) (*protocol.CallToolResponse, error) {
	sess, err := h.requireSession()
	if err != nil {
		return errorResponse(err)
	}
	condition, err := reqString(args, "condition")
	if err != nil {
		return errorResponse(err)
	}
	text, err := optString(args, "text")
	if err != nil {
		return errorResponse(err)
	}
	selector, err := optString(args, "selector")
	if err != nil {
		return errorResponse(err)
	}
	pattern, err := optString(args, "pattern")
	if err != nil {
		return errorResponse(err)
	}
	js, err := optString(args, "js")
	if err != nil {
		return errorResponse(err)
	}
	idleMs, err := optInt(args, "idle_ms", 500)
	if err != nil {
		return errorResponse(err)
	}
	sinceMs, err := optInt(args, "since_ms", 0)
	if err != nil {
		return errorResponse(err)
	}
	timeoutMs, err := optInt(args, "timeout_ms", 10000)
	if err != nil {
		return errorResponse(err)
	}

	elapsed, err := sess.WaitFor(ctx, monitor.WaitParams{
		Condition: condition,
		Text:      text,
		Selector:  selector,
		Pattern:   pattern,
		JS:        js,
		IdleMs:    idleMs,
		SinceMs:   sinceMs,
		TimeoutMs: timeoutMs,
	})
	if err != nil {
		return errorResponse(fmt.Errorf("%v (elapsed %dms)", err, elapsed))
	}
	return jsonResponse(map[string]interface{}{
		"condition":  condition,
		"met":        true,
		"elapsed_ms": elapsed,
	})
}

func (h *Handler) handleGetConsole(args map[string]interface{}) (*protocol.CallToolResponse, error) {
	sess, err := h.requireSession()
	if err != nil {
		return errorResponse(err)
	}
	pattern, err := optString(args, "pattern")
	if err != nil {
		return errorResponse(err)
	}
	levels, err := optStringSlice(args, "levels")
	if err != nil {
		return errorResponse(err)
	}
	sinceMs, err := optInt(args, "since_ms", 0)
	if err != nil {
		return errorResponse(err)
	}
	maxResults, err := optInt(args, "max_results", 50)
	if err != nil {
		return errorResponse(err)
	}
	expandDepth, err := optInt(args, "expand_depth", 5)
	if err != nil {
		return errorResponse(err)
	}

	entries, err := sess.Monitor().Console(monitor.ConsoleFilter{
		Pattern:     pattern,
		Levels:      levels,
		SinceMs:     sinceMs,
		MaxResults:  maxResults,
		ExpandDepth: expandDepth,
	})
	if err != nil {
		return errorResponse(err)
	}
	if len(entries) == 0 {
		return textResponse("no console messages matched")
	}

	var b strings.Builder
	for _, e := range entries {
		line := monitor.RenderArgs(e.Args, 0)
		if len(line) > maxConsoleLine {
			line = line[:maxConsoleLine] + "…[truncated]"
		}
		fmt.Fprintf(&b, "[%s] %s\n", e.Level, line)
	}
	return textResponse(b.String())
}

func (h *Handler) handleGetRequests(args map[string]interface{}) (*protocol.CallToolResponse, error) {
	sess, err := h.requireSession()
	if err != nil {
		return errorResponse(err)
	}
	urlPattern, err := optString(args, "url_pattern")
	if err != nil {
		return errorResponse(err)
	}
	method, err := optString(args, "method")
	if err != nil {
		return errorResponse(err)
	}
	status, err := optIntSlice(args, "status")
	if err != nil {
		return errorResponse(err)
	}
	includeBody, err := optBool(args, "include_body", false)
	if err != nil {
		return errorResponse(err)
	}
	sinceMs, err := optInt(args, "since_ms", 0)
	if err != nil {
		return errorResponse(err)
	}
	maxResults, err := optInt(args, "max_results", 20)
	if err != nil {
		return errorResponse(err)
	}

	requests, err := sess.Monitor().Requests(monitor.RequestFilter{
		URLPattern:  urlPattern,
		Method:      method,
		Status:      status,
		IncludeBody: includeBody,
		SinceMs:     sinceMs,
		MaxResults:  maxResults,
	})
	if err != nil {
		return errorResponse(err)
	}
	if len(requests) == 0 {
		return textResponse("no requests matched")
	}
	return jsonResponse(requests)
}
