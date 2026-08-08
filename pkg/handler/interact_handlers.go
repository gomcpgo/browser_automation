package handler

import (
	"fmt"

	"github.com/gomcpgo/mcp/pkg/protocol"
)

func (h *Handler) handleClick(args map[string]interface{}) (*protocol.CallToolResponse, error) {
	sess, err := h.requireSession()
	if err != nil {
		return errorResponse(err)
	}
	selector, err := reqString(args, "selector")
	if err != nil {
		return errorResponse(err)
	}
	count, err := sess.Click(selector)
	if err != nil {
		return errorResponse(err)
	}
	return jsonResponse(map[string]interface{}{"clicked": selector, "match_count": count})
}

func (h *Handler) handleTypeText(args map[string]interface{}) (*protocol.CallToolResponse, error) {
	sess, err := h.requireSession()
	if err != nil {
		return errorResponse(err)
	}
	selector, err := reqString(args, "selector")
	if err != nil {
		return errorResponse(err)
	}
	text, err := optString(args, "text")
	if err != nil {
		return errorResponse(err)
	}
	clear, err := optBool(args, "clear", false)
	if err != nil {
		return errorResponse(err)
	}
	count, err := sess.TypeText(selector, text, clear)
	if err != nil {
		return errorResponse(err)
	}
	return jsonResponse(map[string]interface{}{"typed_into": selector, "match_count": count})
}

func (h *Handler) handlePressKey(args map[string]interface{}) (*protocol.CallToolResponse, error) {
	sess, err := h.requireSession()
	if err != nil {
		return errorResponse(err)
	}
	key, err := reqString(args, "key")
	if err != nil {
		return errorResponse(err)
	}
	if err := sess.PressKey(key); err != nil {
		return errorResponse(err)
	}
	return jsonResponse(map[string]interface{}{"pressed": key})
}

func (h *Handler) handleHover(args map[string]interface{}) (*protocol.CallToolResponse, error) {
	sess, err := h.requireSession()
	if err != nil {
		return errorResponse(err)
	}
	selector, err := reqString(args, "selector")
	if err != nil {
		return errorResponse(err)
	}
	count, err := sess.Hover(selector)
	if err != nil {
		return errorResponse(err)
	}
	return jsonResponse(map[string]interface{}{"hovered": selector, "match_count": count})
}

func (h *Handler) handleScroll(args map[string]interface{}) (*protocol.CallToolResponse, error) {
	sess, err := h.requireSession()
	if err != nil {
		return errorResponse(err)
	}
	selector, err := optString(args, "selector")
	if err != nil {
		return errorResponse(err)
	}
	x, err := optFloat(args, "x", 0)
	if err != nil {
		return errorResponse(err)
	}
	y, err := optFloat(args, "y", 0)
	if err != nil {
		return errorResponse(err)
	}

	if selector != "" {
		count, err := sess.ScrollToElement(selector)
		if err != nil {
			return errorResponse(err)
		}
		return jsonResponse(map[string]interface{}{"scrolled_to": selector, "match_count": count})
	}
	if x == 0 && y == 0 {
		return errorResponse(fmt.Errorf("pass a selector to scroll to, or a non-zero x/y delta"))
	}
	if err := sess.ScrollBy(x, y); err != nil {
		return errorResponse(err)
	}
	return jsonResponse(map[string]interface{}{"scrolled_by": map[string]float64{"x": x, "y": y}})
}
