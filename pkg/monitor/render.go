package monitor

import (
	"encoding/json"
	"strings"
)

// RenderArgs turns console arguments into a single line. Strings print raw,
// everything else is JSON. depth of 0 means no trimming.
func RenderArgs(args []interface{}, depth int) string {
	parts := make([]string, 0, len(args))
	for _, a := range args {
		if s, ok := a.(string); ok {
			parts = append(parts, s)
			continue
		}
		v := a
		if depth > 0 {
			v = trim(a, depth)
		}
		b, err := json.Marshal(v)
		if err != nil {
			parts = append(parts, "<unserializable>")
			continue
		}
		parts = append(parts, string(b))
	}
	return strings.Join(parts, " ")
}

func trimAll(args []interface{}, depth int) []interface{} {
	out := make([]interface{}, len(args))
	for i, a := range args {
		out[i] = trim(a, depth)
	}
	return out
}

// trim replaces nested values deeper than depth with a placeholder.
func trim(v interface{}, depth int) interface{} {
	switch t := v.(type) {
	case map[string]interface{}:
		if depth <= 1 {
			return "{…}"
		}
		out := make(map[string]interface{}, len(t))
		for k, val := range t {
			out[k] = trim(val, depth-1)
		}
		return out
	case []interface{}:
		if depth <= 1 {
			return "[…]"
		}
		out := make([]interface{}, len(t))
		for i, val := range t {
			out[i] = trim(val, depth-1)
		}
		return out
	default:
		return v
	}
}
