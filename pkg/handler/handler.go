package handler

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"

	"github.com/gomcpgo/browser_automation/pkg/browser"
	"github.com/gomcpgo/mcp/pkg/protocol"
)

// SessionSource supplies the session that tool calls act on. It lets another
// server own the browser (for example persistent, attachable profiles) while
// reusing every page-level tool here. Current returns an error, mentioning
// what to call, when there is no session.
type SessionSource interface {
	Current() (*browser.Session, error)
}

// Handler implements the MCP protocol for browser automation. It either owns
// the one persistent session shared by every tool call (New) or asks a
// SessionSource for it (NewWithSource).
type Handler struct {
	mu         sync.Mutex
	session    *browser.Session
	chromePath string
	source     SessionSource
}

// New creates a handler with no session started yet. An empty chromePath lets
// rod auto-detect the browser.
func New(chromePath string) *Handler {
	return &Handler{chromePath: chromePath}
}

// NewWithSource creates a handler whose session comes from src. start_session
// and close_session are then the host's job; calling them here fails.
func NewWithSource(src SessionSource) *Handler {
	return &Handler{source: src}
}

// ListTools returns the available tools.
func (h *Handler) ListTools(ctx context.Context) (*protocol.ListToolsResponse, error) {
	return &protocol.ListToolsResponse{Tools: h.GetTools()}, nil
}

// CallTool dispatches a tool call.
func (h *Handler) CallTool(ctx context.Context, req *protocol.CallToolRequest) (*protocol.CallToolResponse, error) {
	args := req.Arguments
	if args == nil {
		args = map[string]interface{}{}
	}

	switch req.Name {
	case "start_session":
		if h.source != nil {
			return errorResponse(fmt.Errorf("start_session is managed by the host server"))
		}
		return h.handleStartSession(args)
	case "close_session":
		if h.source != nil {
			return errorResponse(fmt.Errorf("close_session is managed by the host server"))
		}
		return h.handleCloseSession()
	case "navigate":
		return h.handleNavigate(args)
	case "snapshot":
		return h.handleSnapshot(args)
	case "get_element":
		return h.handleGetElement(args)
	case "click":
		return h.handleClick(args)
	case "type_text":
		return h.handleTypeText(args)
	case "insert_text":
		return h.handleInsertText(args)
	case "find":
		return h.handleFind(args)
	case "press_key":
		return h.handlePressKey(args)
	case "hover":
		return h.handleHover(args)
	case "scroll":
		return h.handleScroll(args)
	case "wait_for":
		return h.handleWaitFor(ctx, args)
	case "get_console":
		return h.handleGetConsole(args)
	case "get_requests":
		return h.handleGetRequests(args)
	case "evaluate":
		return h.handleEvaluate(args)
	case "screenshot":
		return h.handleScreenshot(args)
	}

	return nil, fmt.Errorf("unknown tool: %s", req.Name)
}

// requireSession returns the active session or an explanatory error.
func (h *Handler) requireSession() (*browser.Session, error) {
	if h.source != nil {
		return h.source.Current()
	}

	h.mu.Lock()
	defer h.mu.Unlock()

	if h.session == nil {
		return nil, fmt.Errorf("no browser session; call start_session first")
	}
	return h.session, nil
}

// Shutdown closes the session if one is open.
func (h *Handler) Shutdown() {
	h.mu.Lock()
	defer h.mu.Unlock()

	if h.session != nil {
		h.session.Close()
		h.session = nil
	}
}

func jsonResponse(v interface{}) (*protocol.CallToolResponse, error) {
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return errorResponse(fmt.Errorf("failed to encode response: %w", err))
	}
	return textResponse(string(data))
}

func textResponse(text string) (*protocol.CallToolResponse, error) {
	return &protocol.CallToolResponse{
		Content: []protocol.ToolContent{{Type: "text", Text: text}},
	}, nil
}

func errorResponse(err error) (*protocol.CallToolResponse, error) {
	return &protocol.CallToolResponse{
		IsError: true,
		Content: []protocol.ToolContent{{Type: "text", Text: err.Error()}},
	}, nil
}
