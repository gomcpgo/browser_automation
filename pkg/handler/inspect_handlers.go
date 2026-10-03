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

// extractFindParams reads the find tool's arguments.
func extractFindParams(args map[string]interface{}) (snapshot.FindParams, error) {
	var p snapshot.FindParams
	var err error
	for key, dst := range map[string]*string{
		"text": &p.Text, "aria_label": &p.AriaLabel, "role": &p.Role,
		"title": &p.Title, "placeholder": &p.Placeholder, "within": &p.Within,
	} {
		if *dst, err = optString(args, key); err != nil {
			return p, err
		}
	}
	if p.VisibleOnly, err = optBool(args, "visible_only", true); err != nil {
		return p, err
	}
	if p.MaxResults, err = optInt(args, "max_results", 10); err != nil {
		return p, err
	}
	return p, p.Validate()
}

func (h *Handler) handleFind(args map[string]interface{}) (*protocol.CallToolResponse, error) {
	sess, err := h.requireSession()
	if err != nil {
		return errorResponse(err)
	}
	params, err := extractFindParams(args)
	if err != nil {
		return errorResponse(err)
	}
	result, err := snapshot.Find(sess.Page(), params)
	if err != nil {
		return errorResponse(err)
	}
	if result.Total == 0 {
		return textResponse("no elements matched; try a shorter text, a different role, or visible_only: false")
	}
	return jsonResponse(result)
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
