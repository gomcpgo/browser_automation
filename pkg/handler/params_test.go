package handler

import "testing"

func TestReqString(t *testing.T) {
	args := map[string]interface{}{"url": "http://localhost", "n": float64(3), "empty": ""}

	if v, err := reqString(args, "url"); err != nil || v != "http://localhost" {
		t.Fatalf("got %q, %v", v, err)
	}
	if _, err := reqString(args, "missing"); err == nil {
		t.Fatal("expected error for missing key")
	}
	if _, err := reqString(args, "n"); err == nil {
		t.Fatal("expected error for wrong type")
	}
	if _, err := reqString(args, "empty"); err == nil {
		t.Fatal("expected error for empty string")
	}
}

func TestOptIntCoercesJSONNumbers(t *testing.T) {
	args := map[string]interface{}{"width": float64(1920), "bad": "1920"}

	if v, err := optInt(args, "width", 800); err != nil || v != 1920 {
		t.Fatalf("got %d, %v", v, err)
	}
	if v, err := optInt(args, "missing", 800); err != nil || v != 800 {
		t.Fatalf("expected default 800, got %d, %v", v, err)
	}
	if _, err := optInt(args, "bad", 0); err == nil {
		t.Fatal("expected error for string value")
	}
}

func TestOptBoolAndFloat(t *testing.T) {
	args := map[string]interface{}{"headed": true, "zoom": float64(2.5), "bad": "yes"}

	if v, err := optBool(args, "headed", false); err != nil || !v {
		t.Fatalf("got %v, %v", v, err)
	}
	if _, err := optBool(args, "bad", false); err == nil {
		t.Fatal("expected error for string value")
	}
	if v, err := optFloat(args, "zoom", 1); err != nil || v != 2.5 {
		t.Fatalf("got %v, %v", v, err)
	}
	if v, err := optFloat(args, "missing", 1); err != nil || v != 1 {
		t.Fatalf("expected default 1, got %v, %v", v, err)
	}
}

func TestOptSlices(t *testing.T) {
	args := map[string]interface{}{
		"levels":   []interface{}{"warn", "error"},
		"status":   []interface{}{float64(200), float64(404)},
		"badItems": []interface{}{"a", float64(1)},
		"notList":  "warn",
	}

	levels, err := optStringSlice(args, "levels")
	if err != nil || len(levels) != 2 || levels[1] != "error" {
		t.Fatalf("got %v, %v", levels, err)
	}
	status, err := optIntSlice(args, "status")
	if err != nil || len(status) != 2 || status[1] != 404 {
		t.Fatalf("got %v, %v", status, err)
	}
	if _, err := optStringSlice(args, "badItems"); err == nil {
		t.Fatal("expected error for mixed types")
	}
	if _, err := optStringSlice(args, "notList"); err == nil {
		t.Fatal("expected error for non-array")
	}
	if v, err := optStringSlice(args, "missing"); err != nil || v != nil {
		t.Fatalf("expected nil, got %v, %v", v, err)
	}
}

func TestOptRect(t *testing.T) {
	args := map[string]interface{}{
		"rect": map[string]interface{}{
			"x": float64(10), "y": float64(20), "width": float64(300), "height": float64(150),
		},
		"badRect": "10,20",
	}

	r, err := optRect(args, "rect")
	if err != nil {
		t.Fatal(err)
	}
	if r.X != 10 || r.Y != 20 || r.Width != 300 || r.Height != 150 {
		t.Fatalf("unexpected rect %+v", r)
	}
	if _, err := optRect(args, "badRect"); err == nil {
		t.Fatal("expected error for non-object rect")
	}
	if r, err := optRect(args, "missing"); err != nil || r != nil {
		t.Fatalf("expected nil rect, got %v, %v", r, err)
	}
}
