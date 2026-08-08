package handler

import (
	"fmt"

	"github.com/gomcpgo/browser_automation/pkg/capture"
)

func reqString(args map[string]interface{}, key string) (string, error) {
	v, ok := args[key]
	if !ok || v == nil {
		return "", fmt.Errorf("%s is required", key)
	}
	s, ok := v.(string)
	if !ok {
		return "", fmt.Errorf("%s must be a string", key)
	}
	if s == "" {
		return "", fmt.Errorf("%s must not be empty", key)
	}
	return s, nil
}

func optString(args map[string]interface{}, key string) (string, error) {
	v, ok := args[key]
	if !ok || v == nil {
		return "", nil
	}
	s, ok := v.(string)
	if !ok {
		return "", fmt.Errorf("%s must be a string", key)
	}
	return s, nil
}

func optInt(args map[string]interface{}, key string, def int) (int, error) {
	v, ok := args[key]
	if !ok || v == nil {
		return def, nil
	}
	switch n := v.(type) {
	case float64:
		return int(n), nil
	case int:
		return n, nil
	default:
		return 0, fmt.Errorf("%s must be a number", key)
	}
}

func optFloat(args map[string]interface{}, key string, def float64) (float64, error) {
	v, ok := args[key]
	if !ok || v == nil {
		return def, nil
	}
	switch n := v.(type) {
	case float64:
		return n, nil
	case int:
		return float64(n), nil
	default:
		return 0, fmt.Errorf("%s must be a number", key)
	}
}

func optBool(args map[string]interface{}, key string, def bool) (bool, error) {
	v, ok := args[key]
	if !ok || v == nil {
		return def, nil
	}
	b, ok := v.(bool)
	if !ok {
		return false, fmt.Errorf("%s must be a boolean", key)
	}
	return b, nil
}

func optStringSlice(args map[string]interface{}, key string) ([]string, error) {
	v, ok := args[key]
	if !ok || v == nil {
		return nil, nil
	}
	list, ok := v.([]interface{})
	if !ok {
		return nil, fmt.Errorf("%s must be an array of strings", key)
	}
	out := make([]string, 0, len(list))
	for _, item := range list {
		s, ok := item.(string)
		if !ok {
			return nil, fmt.Errorf("%s must contain only strings", key)
		}
		out = append(out, s)
	}
	return out, nil
}

func optIntSlice(args map[string]interface{}, key string) ([]int, error) {
	v, ok := args[key]
	if !ok || v == nil {
		return nil, nil
	}
	list, ok := v.([]interface{})
	if !ok {
		return nil, fmt.Errorf("%s must be an array of numbers", key)
	}
	out := make([]int, 0, len(list))
	for _, item := range list {
		switch n := item.(type) {
		case float64:
			out = append(out, int(n))
		case int:
			out = append(out, n)
		default:
			return nil, fmt.Errorf("%s must contain only numbers", key)
		}
	}
	return out, nil
}

// optRect reads a {x, y, width, height} object.
func optRect(args map[string]interface{}, key string) (*capture.Rect, error) {
	v, ok := args[key]
	if !ok || v == nil {
		return nil, nil
	}
	m, ok := v.(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("%s must be an object with x, y, width and height", key)
	}
	r := &capture.Rect{}
	for name, dst := range map[string]*float64{
		"x": &r.X, "y": &r.Y, "width": &r.Width, "height": &r.Height,
	} {
		f, err := optFloat(m, name, 0)
		if err != nil {
			return nil, fmt.Errorf("%s.%s must be a number", key, name)
		}
		*dst = f
	}
	return r, nil
}
