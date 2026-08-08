package browser

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/launcher"
	"github.com/go-rod/rod/lib/proto"

	"github.com/gomcpgo/browser_automation/pkg/monitor"
)

// Options configures a browser session.
type Options struct {
	Width  int
	Height int
	Headed bool
	Zoom   float64
}

// Session is one persistent browser + page. Console and network listeners are
// attached at start, so buffers hold history from the very first navigation.
type Session struct {
	mu       sync.Mutex
	launcher *launcher.Launcher
	browser  *rod.Browser
	page     *rod.Page
	mon      *monitor.Monitor
	opts     Options
}

// Start launches Chrome and opens the single page used by the session.
func Start(opts Options) (*Session, error) {
	if opts.Width <= 0 {
		opts.Width = 1280
	}
	if opts.Height <= 0 {
		opts.Height = 800
	}
	if opts.Zoom <= 0 {
		opts.Zoom = 1
	}

	l := launcher.New().Headless(!opts.Headed)
	if bin, exists := launcher.LookPath(); exists {
		l = l.Bin(bin)
	}
	controlURL, err := l.Launch()
	if err != nil {
		return nil, fmt.Errorf("failed to launch chrome: %w", err)
	}

	br := rod.New().ControlURL(controlURL)
	if err := br.Connect(); err != nil {
		l.Cleanup()
		return nil, fmt.Errorf("failed to connect to chrome: %w", err)
	}

	page, err := br.Page(proto.TargetCreateTarget{URL: "about:blank"})
	if err != nil {
		br.Close()
		l.Cleanup()
		return nil, fmt.Errorf("failed to open page: %w", err)
	}

	err = page.SetViewport(&proto.EmulationSetDeviceMetricsOverride{
		Width:             opts.Width,
		Height:            opts.Height,
		DeviceScaleFactor: 1,
	})
	if err != nil {
		br.Close()
		l.Cleanup()
		return nil, fmt.Errorf("failed to set viewport: %w", err)
	}

	s := &Session{
		launcher: l,
		browser:  br,
		page:     page,
		mon:      monitor.New(),
		opts:     opts,
	}

	if err := s.attachListeners(); err != nil {
		s.Close()
		return nil, err
	}

	return s, nil
}

// Close shuts down the browser.
func (s *Session) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	var err error
	if s.browser != nil {
		err = s.browser.Close()
		s.browser = nil
	}
	if s.launcher != nil {
		s.launcher.Cleanup()
		s.launcher = nil
	}
	s.page = nil
	return err
}

// Page exposes the live page for the snapshot and capture packages.
func (s *Session) Page() *rod.Page { return s.page }

// Monitor exposes the event buffers.
func (s *Session) Monitor() *monitor.Monitor { return s.mon }

// Zoom returns the session zoom level.
func (s *Session) Zoom() float64 { return s.opts.Zoom }

// Info returns the session settings and the current page URL and title.
func (s *Session) Info() map[string]interface{} {
	info := map[string]interface{}{
		"width":  s.opts.Width,
		"height": s.opts.Height,
		"headed": s.opts.Headed,
		"zoom":   s.opts.Zoom,
	}
	if pi, err := s.page.Info(); err == nil {
		info["url"] = pi.URL
		info["title"] = pi.Title
	}
	return info
}

// Navigate loads a URL, waits for the load event and re-applies session zoom.
func (s *Session) Navigate(url string) error {
	if err := s.page.Navigate(url); err != nil {
		return fmt.Errorf("navigation failed: %w", err)
	}
	if err := s.page.WaitLoad(); err != nil {
		return fmt.Errorf("page load failed: %w", err)
	}
	return s.ApplyZoom(s.opts.Zoom)
}

// ApplyZoom sets the page zoom, the documented sharpness lever. Layout reflows,
// so callers must locate elements after zooming.
func (s *Session) ApplyZoom(zoom float64) error {
	if zoom <= 0 {
		zoom = 1
	}
	js := fmt.Sprintf(`() => { document.documentElement.style.zoom = %q; }`, fmt.Sprintf("%g", zoom))
	if _, err := s.page.Eval(js); err != nil {
		return fmt.Errorf("failed to apply zoom: %w", err)
	}
	// Give the browser a frame to reflow at the new zoom.
	time.Sleep(150 * time.Millisecond)
	return nil
}

// Eval runs a JS expression and returns its JSON value.
func (s *Session) Eval(js string) (interface{}, error) {
	obj, err := s.page.Eval(fmt.Sprintf(`() => (%s)`, js))
	if err != nil {
		return nil, fmt.Errorf("evaluate failed: %w", err)
	}
	return obj.Value.Val(), nil
}

// evalRaw runs a full JS function expression as given.
func (s *Session) evalRaw(js string) (interface{}, error) {
	obj, err := s.page.Eval(js)
	if err != nil {
		return nil, err
	}
	return obj.Value.Val(), nil
}

// evalInto runs a full JS function expression and decodes the result into out.
func (s *Session) evalInto(js string, out interface{}) error {
	obj, err := s.page.Eval(js)
	if err != nil {
		return err
	}
	return json.Unmarshal([]byte(obj.Value.JSON("", "")), out)
}

// WaitFor blocks until the condition is met or the timeout expires.
func (s *Session) WaitFor(ctx context.Context, p monitor.WaitParams) (int, error) {
	eval := func(js string) (bool, error) {
		v, err := s.evalRaw(js)
		if err != nil {
			return false, err
		}
		b, _ := v.(bool)
		return b, nil
	}
	return monitor.Wait(ctx, s.mon, eval, p)
}
