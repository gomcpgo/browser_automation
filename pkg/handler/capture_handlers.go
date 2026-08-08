package handler

import (
	"github.com/gomcpgo/browser_automation/pkg/capture"
	"github.com/gomcpgo/mcp/pkg/protocol"
)

func (h *Handler) handleScreenshot(args map[string]interface{}) (*protocol.CallToolResponse, error) {
	sess, err := h.requireSession()
	if err != nil {
		return errorResponse(err)
	}
	outputPath, err := reqString(args, "output_path")
	if err != nil {
		return errorResponse(err)
	}
	selector, err := optString(args, "selector")
	if err != nil {
		return errorResponse(err)
	}
	rect, err := optRect(args, "rect")
	if err != nil {
		return errorResponse(err)
	}
	zoom, err := optFloat(args, "zoom", 0)
	if err != nil {
		return errorResponse(err)
	}

	params := capture.Params{OutputPath: outputPath, Selector: selector, Rect: rect}
	if err := params.Validate(); err != nil {
		return errorResponse(err)
	}

	// A per-shot zoom reflows the page, so apply it before locating elements
	// and restore the session zoom afterwards.
	if zoom > 0 && zoom != sess.Zoom() {
		if err := sess.ApplyZoom(zoom); err != nil {
			return errorResponse(err)
		}
		defer sess.ApplyZoom(sess.Zoom())
	}

	result, err := capture.Screenshot(sess.Page(), params)
	if err != nil {
		return errorResponse(err)
	}
	return jsonResponse(result)
}
