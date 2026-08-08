package capture

import (
	"bytes"
	"fmt"
	"image/png"
	"os"
	"path/filepath"

	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/proto"
)

// Rect is an explicit crop region in CSS pixels.
type Rect struct {
	X      float64
	Y      float64
	Width  float64
	Height float64
}

// Params describes one screenshot.
type Params struct {
	OutputPath string
	Selector   string
	Rect       *Rect
}

// Result reports what was written.
type Result struct {
	Path       string `json:"path"`
	Width      int    `json:"width"`
	Height     int    `json:"height"`
	Bytes      int    `json:"bytes"`
	MatchCount int    `json:"match_count,omitempty"`
}

// Validate checks the parameters before a browser is touched.
func (p Params) Validate() error {
	if p.OutputPath == "" {
		return fmt.Errorf("output_path is required")
	}
	if !filepath.IsAbs(p.OutputPath) {
		return fmt.Errorf("output_path must be absolute, got %q", p.OutputPath)
	}
	if p.Selector != "" && p.Rect != nil {
		return fmt.Errorf("pass either selector or rect, not both")
	}
	if p.Rect != nil && (p.Rect.Width <= 0 || p.Rect.Height <= 0) {
		return fmt.Errorf("rect width and height must be positive")
	}
	return nil
}

// Screenshot captures the viewport, one element, or an explicit rect, and
// writes the PNG to the exact path given.
func Screenshot(page *rod.Page, p Params) (*Result, error) {
	if err := p.Validate(); err != nil {
		return nil, err
	}

	var (
		data       []byte
		err        error
		matchCount int
	)

	switch {
	case p.Selector != "":
		els, e := page.Elements(p.Selector)
		if e != nil {
			return nil, fmt.Errorf("invalid selector %q: %w", p.Selector, e)
		}
		if len(els) == 0 {
			return nil, fmt.Errorf("no element matches %q", p.Selector)
		}
		matchCount = len(els)
		if e := els[0].ScrollIntoView(); e != nil {
			return nil, fmt.Errorf("failed to scroll %q into view: %w", p.Selector, e)
		}
		data, err = els[0].Screenshot(proto.PageCaptureScreenshotFormatPng, 0)

	case p.Rect != nil:
		data, err = page.Screenshot(false, &proto.PageCaptureScreenshot{
			Format: proto.PageCaptureScreenshotFormatPng,
			Clip: &proto.PageViewport{
				X:      p.Rect.X,
				Y:      p.Rect.Y,
				Width:  p.Rect.Width,
				Height: p.Rect.Height,
				Scale:  1,
			},
		})

	default:
		data, err = page.Screenshot(false, &proto.PageCaptureScreenshot{
			Format: proto.PageCaptureScreenshotFormatPng,
		})
	}
	if err != nil {
		return nil, fmt.Errorf("screenshot failed: %w", err)
	}

	if err := os.MkdirAll(filepath.Dir(p.OutputPath), 0755); err != nil {
		return nil, fmt.Errorf("failed to create output directory: %w", err)
	}
	if err := os.WriteFile(p.OutputPath, data, 0644); err != nil {
		return nil, fmt.Errorf("failed to write screenshot: %w", err)
	}

	res := &Result{Path: p.OutputPath, Bytes: len(data), MatchCount: matchCount}
	if cfg, err := png.DecodeConfig(bytes.NewReader(data)); err == nil {
		res.Width = cfg.Width
		res.Height = cfg.Height
	}
	return res, nil
}
