package handler

import (
	"github.com/gomcpgo/browser_automation/pkg/snapshot"
	"github.com/gomcpgo/mcp/pkg/protocol"
)

func (h *Handler) handleSnapshot(args map[string]interface{}) (*protocol.CallToolResponse, error) {
	sess, err := h.requireSession()
	if err != nil {
		return errorResponse(err)
	}
	selector, err := optString(args, "selector")
	if err != nil {
		return errorResponse(err)
	}
	maxDepth, err := optInt(args, "max_depth", 12)
	if err != nil {
		return errorResponse(err)
	}
	interactiveOnly, err := optBool(args, "interactive_only", false)
	if err != nil {
		return errorResponse(err)
	}

	nodes, err := snapshot.Outline(sess.Page(), snapshot.Params{
		Selector:        selector,
		MaxDepth:        maxDepth,
		InteractiveOnly: interactiveOnly,
	})
	if err != nil {
		return errorResponse(err)
	}
	if len(nodes) == 0 {
		return textResponse("no visible elements matched")
	}
	return textResponse(snapshot.Format(nodes))
}

func (h *Handler) handleGetElement(args map[string]interface{}) (*protocol.CallToolResponse, error) {
	sess, err := h.requireSession()
	if err != nil {
		return errorResponse(err)
	}
	selector, err := reqString(args, "selector")
	if err != nil {
		return errorResponse(err)
	}
	styles, err := optStringSlice(args, "include_styles")
	if err != nil {
		return errorResponse(err)
	}

	info, err := snapshot.Element(sess.Page(), snapshot.ElementParams{
		Selector:      selector,
		IncludeStyles: styles,
	})
	if err != nil {
		return errorResponse(err)
	}
	return jsonResponse(info)
}

func (h *Handler) handleEvaluate(args map[string]interface{}) (*protocol.CallToolResponse, error) {
	sess, err := h.requireSession()
	if err != nil {
		return errorResponse(err)
	}
	js, err := reqString(args, "js")
	if err != nil {
		return errorResponse(err)
	}

	value, err := sess.Eval(js)
	if err != nil {
		return errorResponse(err)
	}
	return jsonResponse(map[string]interface{}{"value": value})
}
