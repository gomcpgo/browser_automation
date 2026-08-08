package handler

import (
	"github.com/gomcpgo/browser_automation/pkg/browser"
	"github.com/gomcpgo/mcp/pkg/protocol"
)

func (h *Handler) handleStartSession(args map[string]interface{}) (*protocol.CallToolResponse, error) {
	width, err := optInt(args, "width", 1280)
	if err != nil {
		return errorResponse(err)
	}
	height, err := optInt(args, "height", 800)
	if err != nil {
		return errorResponse(err)
	}
	headed, err := optBool(args, "headed", false)
	if err != nil {
		return errorResponse(err)
	}
	zoom, err := optFloat(args, "zoom", 1)
	if err != nil {
		return errorResponse(err)
	}

	h.mu.Lock()
	if h.session != nil {
		h.session.Close()
		h.session = nil
	}
	h.mu.Unlock()

	sess, err := browser.Start(browser.Options{
		Width:  width,
		Height: height,
		Headed: headed,
		Zoom:   zoom,
	})
	if err != nil {
		return errorResponse(err)
	}

	h.mu.Lock()
	h.session = sess
	h.mu.Unlock()

	return jsonResponse(sess.Info())
}

func (h *Handler) handleCloseSession() (*protocol.CallToolResponse, error) {
	h.mu.Lock()
	sess := h.session
	h.session = nil
	h.mu.Unlock()

	if sess == nil {
		return textResponse("no session was open")
	}
	if err := sess.Close(); err != nil {
		return errorResponse(err)
	}
	return textResponse("session closed")
}

func (h *Handler) handleNavigate(args map[string]interface{}) (*protocol.CallToolResponse, error) {
	sess, err := h.requireSession()
	if err != nil {
		return errorResponse(err)
	}
	url, err := reqString(args, "url")
	if err != nil {
		return errorResponse(err)
	}
	if err := sess.Navigate(url); err != nil {
		return errorResponse(err)
	}
	return jsonResponse(sess.Info())
}
