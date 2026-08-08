package capture

import (
	"strings"
	"testing"
)

func TestValidate(t *testing.T) {
	cases := []struct {
		name    string
		params  Params
		wantErr string
	}{
		{"absolute path", Params{OutputPath: "/tmp/shot.png"}, ""},
		{"missing path", Params{}, "output_path is required"},
		{"relative path", Params{OutputPath: "shot.png"}, "must be absolute"},
		{
			"selector and rect",
			Params{OutputPath: "/tmp/a.png", Selector: ".card", Rect: &Rect{Width: 10, Height: 10}},
			"not both",
		},
		{
			"zero size rect",
			Params{OutputPath: "/tmp/a.png", Rect: &Rect{Width: 0, Height: 10}},
			"must be positive",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.params.Validate()
			if tc.wantErr == "" {
				if err != nil {
					t.Fatalf("expected no error, got %v", err)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
				t.Fatalf("expected error containing %q, got %v", tc.wantErr, err)
			}
		})
	}
}
