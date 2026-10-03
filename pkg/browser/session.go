package browser

import (
	"context"
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
	Width      int
	Height     int
	Headed     bool
	Zoom       float64
	ChromePath string
}

// Session is one persistent page with console and network listeners attached,
// so buffers hold history from the moment the session was created.
//
// A session either owns its browser (Start launched it, Close kills it) or
// wraps a page someone else owns (Wrap; Close only detaches the listeners).
type Session struct {
	mu         sync.Mutex
	launcher   *launcher.Launcher
	browser    *rod.Browser
	page       *rod.Page
	mon        *monitor.Monitor
	opts       Options
	owned      bool
	stopEvents context.CancelFunc
}

// WrapOptions configures a session built around an existing page.
type WrapOptions struct {
	// Zoom is the CSS zoom re-applied after navigation and used for
	// screenshots. Zero means 1, which leaves the page untouched.
	Zoom float64
}

// Wrap builds a session around a page owned by the caller, for example one tab
// of a browser that must outlive this process. It attaches the console,
// network and dialog listeners and applies no viewport override. Close detaches
// the listeners and never closes the page or its browser.
func Wrap(page *rod.Page, opts WrapOptions) (*Session, error) {
	if page == nil {
		return nil, fmt.Errorf("cannot wrap a nil page")
	}
	if opts.Zoom <= 0 {
		opts.Zoom = 1
	}

	s := &Session{
		page:  page,
		mon:   monitor.New(),
		opts:  Options{Headed: true, Zoom: opts.Zoom},
		owned: false,
	}
	if err := s.attachListeners(); err != nil {
		s.Close()
		return nil, err
	}
	return s, nil
}

// Owned reports whether Close will shut the browser down (true) or only
// detach from a page owned elsewhere (false).
func (s *Session) Owned() bool { return s.owned }

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
	if opts.ChromePath != "" {
		l = l.Bin(opts.ChromePath)
	} else if bin, exists := launcher.LookPath(); exists {
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
		owned:    true,
	}

	if err := s.attachListeners(); err != nil {
		s.Close()
		return nil, err
	}

	return s, nil
}

// Close shuts down the browser of an owned session. For a wrapped session it
// only stops the event listeners; the page and browser are left exactly as
// they are.
func (s *Session) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.stopEvents != nil {
		s.stopEvents()
		s.stopEvents = nil
	}
	if !s.owned {
		s.page = nil
		return nil
	}

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

// opTimeout bounds a single tool call, and navTimeout a navigation. Without a
// bound, anything that blocks the page (a JS dialog, a runaway script) would
// hang the caller forever.
const (
	opTimeout  = 30 * time.Second
	navTimeout = 60 * time.Second

	// waitGrace lets the wait loop notice its own deadline before the page
	// context cuts the in-flight poll short.
	waitGrace = 500 * time.Millisecond
)

// Page exposes the live page, bounded by the per-call timeout, for the snapshot
// and capture packages.
func (s *Session) Page() *rod.Page { return s.page.Timeout(opTimeout) }

// Monitor exposes the event buffers.
func (s *Session) Monitor() *monitor.Monitor { return s.mon }

// Zoom returns the session zoom level.
func (s *Session) Zoom() float64 { return s.opts.Zoom }

// Info returns the session settings and the current page URL and title.
func (s *Session) Info() map[string]interface{} {
	info := map[string]interface{}{
		"headed": s.opts.Headed,
		"zoom":   s.opts.Zoom,
	}
	if s.opts.Width > 0 && s.opts.Height > 0 {
		info["width"] = s.opts.Width
		info["height"] = s.opts.Height
	}
	if pi, err := s.page.Info(); err == nil {
		info["url"] = pi.URL
		info["title"] = pi.Title
	}
	return info
}

// Navigate loads a URL, waits for the load event and re-applies session zoom.
func (s *Session) Navigate(url string) error {
	page := s.page.Timeout(navTimeout)

	if err := page.Navigate(url); err != nil {
		return fmt.Errorf("navigation failed: %w", err)
	}
	if err := page.WaitLoad(); err != nil {
		return fmt.Errorf("page load failed: %w", err)
	}
	return s.ApplyZoom(s.opts.Zoom)
}

// ApplyZoom sets the page zoom, the documented sharpness lever. It is
// idempotent: a page that already carries the zoom is left alone, so the reflow
// pause is only paid when the zoom actually changes. Layout reflows, so callers
// must locate elements after zooming.
func (s *Session) ApplyZoom(zoom float64) error {
	if zoom <= 0 {
		zoom = 1
	}

	obj, err := s.Page().Eval(`(want) => {
		if (document.documentElement.style.zoom === want) return true;
		document.documentElement.style.zoom = want;
		return false;
	}`, fmt.Sprintf("%g", zoom))
	if err != nil {
		return fmt.Errorf("failed to apply zoom: %w", err)
	}
	if obj.Value.Bool() {
		return nil
	}

	// Give the browser a frame to reflow at the new zoom.
	time.Sleep(150 * time.Millisecond)
	return nil
}

// Eval runs a JS expression and returns its JSON value.
func (s *Session) Eval(js string) (interface{}, error) {
	obj, err := s.Page().Eval(fmt.Sprintf(`() => (%s)`, js))
	if err != nil {
		return nil, fmt.Errorf("evaluate failed: %w", err)
	}
	return obj.Value.Val(), nil
}

// WaitFor blocks until the condition is met or the timeout expires. The wait's
// own deadline is pushed down into the page, so a blocked page fails the wait on
// time instead of hanging past it.
func (s *Session) WaitFor(ctx context.Context, p monitor.WaitParams) (int, error) {
	waitCtx, cancel := context.WithTimeout(ctx, time.Duration(p.TimeoutMs)*time.Millisecond+waitGrace)
	defer cancel()

	page := s.page.Context(waitCtx)
	eval := func(js string) (bool, error) {
		obj, err := page.Eval(js)
		if err != nil {
			return false, err
		}
		v, _ := obj.Value.Val().(bool)
		return v, nil
	}
	return monitor.Wait(ctx, s.mon, eval, p)
}
