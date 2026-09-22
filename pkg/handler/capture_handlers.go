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

	// Apply the zoom unconditionally rather than trusting the session value: a
	// page that reloaded itself has dropped the zoom, and comparing would skip
	// re-applying it and capture at 1x. ApplyZoom is idempotent and reflows the
	// page, so it also has to run before elements are located.
	effective := zoom
	if effective <= 0 {
		effective = sess.Zoom()
	}
	if err := sess.ApplyZoom(effective); err != nil {
		return errorResponse(err)
	}
	if effective != sess.Zoom() {
		defer sess.ApplyZoom(sess.Zoom())
	}

	result, err := capture.Screenshot(sess.Page(), params)
	if err != nil {
		return errorResponse(err)
	}
	return jsonResponse(result)
}
