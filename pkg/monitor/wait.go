package monitor

import (
	"context"
	"encoding/json"
	"fmt"
	"time"
)

const pollInterval = 100 * time.Millisecond

// WaitParams describes one wait_for call.
type WaitParams struct {
	Condition string
	Text      string
	Selector  string
	Pattern   string
	JS        string
	IdleMs    int
	SinceMs   int
	TimeoutMs int
}

// EvalBool runs a JS expression in the page and reports its truthiness.
type EvalBool func(js string) (bool, error)

// Wait polls the condition until it is met or the timeout expires. It returns
// the elapsed time in milliseconds.
func Wait(ctx context.Context, m *Monitor, eval EvalBool, p WaitParams) (int, error) {
	check, err := checker(m, eval, p)
	if err != nil {
		return 0, err
	}

	start := time.Now()
	deadline := start.Add(time.Duration(p.TimeoutMs) * time.Millisecond)

	timedOut := func() error {
		return fmt.Errorf("timed out after %dms waiting for %s", p.TimeoutMs, p.Condition)
	}

	for {
		ok, err := check()
		if err != nil {
			// A blocked page (a modal JS dialog, say) fails the poll rather than
			// answering it. Past the deadline that is a timeout, not a new error.
			if time.Now().After(deadline) {
				return int(time.Since(start).Milliseconds()), timedOut()
			}
			return int(time.Since(start).Milliseconds()), err
		}
		if ok {
			return int(time.Since(start).Milliseconds()), nil
		}
		if time.Now().After(deadline) {
			return int(time.Since(start).Milliseconds()), timedOut()
		}

		select {
		case <-ctx.Done():
			return int(time.Since(start).Milliseconds()), ctx.Err()
		case <-time.After(pollInterval):
		}
	}
}

func checker(m *Monitor, eval EvalBool, p WaitParams) (func() (bool, error), error) {
	switch p.Condition {
	case "text_visible", "text_gone":
		if p.Text == "" {
			return nil, fmt.Errorf("condition %q requires 'text'", p.Condition)
		}
		js := fmt.Sprintf(`() => %s(%s)`, jsTextVisible, quote(p.Text))
		want := p.Condition == "text_visible"
		return func() (bool, error) {
			v, err := eval(js)
			if err != nil {
				return false, err
			}
			return v == want, nil
		}, nil

	case "selector_visible", "selector_hidden":
		if p.Selector == "" {
			return nil, fmt.Errorf("condition %q requires 'selector'", p.Condition)
		}
		js := fmt.Sprintf(`() => %s(%s)`, jsSelectorVisible, quote(p.Selector))
		want := p.Condition == "selector_visible"
		return func() (bool, error) {
			v, err := eval(js)
			if err != nil {
				return false, err
			}
			return v == want, nil
		}, nil

	case "console_matches":
		if p.Pattern == "" {
			return nil, fmt.Errorf("condition %q requires 'pattern'", p.Condition)
		}
		if _, err := m.ConsoleMatches(p.Pattern, p.SinceMs); err != nil {
			return nil, err
		}
		return func() (bool, error) {
			return m.ConsoleMatches(p.Pattern, p.SinceMs)
		}, nil

	case "network_idle":
		idle := p.IdleMs
		if idle <= 0 {
			idle = 500
		}
		return func() (bool, error) {
			return m.NetworkIdle(idle), nil
		}, nil

	case "js_true":
		if p.JS == "" {
			return nil, fmt.Errorf("condition %q requires 'js'", p.Condition)
		}
		js := fmt.Sprintf(`() => !!(%s)`, p.JS)
		return func() (bool, error) {
			return eval(js)
		}, nil
	}

	return nil, fmt.Errorf("unknown condition %q (use text_visible, text_gone, selector_visible, "+
		"selector_hidden, console_matches, network_idle or js_true)", p.Condition)
}

func quote(s string) string {
	b, _ := json.Marshal(s)
	return string(b)
}

const jsVisible = `((el) => {
	if (!el) return false;
	const st = getComputedStyle(el);
	if (st.display === 'none' || st.visibility === 'hidden' || parseFloat(st.opacity) === 0) return false;
	const r = el.getBoundingClientRect();
	return r.width > 0 && r.height > 0;
})`

const jsTextVisible = `((needle) => {
	const visible = ` + jsVisible + `;
	const walk = document.createTreeWalker(document.body, NodeFilter.SHOW_TEXT);
	let n;
	while ((n = walk.nextNode())) {
		if (n.nodeValue && n.nodeValue.includes(needle) && visible(n.parentElement)) return true;
	}
	return false;
})`

const jsSelectorVisible = `((sel) => {
	const visible = ` + jsVisible + `;
	return Array.from(document.querySelectorAll(sel)).some(visible);
})`
