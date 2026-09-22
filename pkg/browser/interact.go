package browser

import (
	"fmt"
	"strings"

	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/input"
	"github.com/go-rod/rod/lib/proto"
)

// element resolves a selector, reporting how many nodes matched so an ambiguous
// selector is visible in the result rather than a hard error.
func (s *Session) element(selector string) (*rod.Element, int, error) {
	els, err := s.Page().Elements(selector)
	if err != nil {
		return nil, 0, fmt.Errorf("invalid selector %q: %w", selector, err)
	}
	if len(els) == 0 {
		return nil, 0, fmt.Errorf("no element matches %q", selector)
	}
	return els[0], len(els), nil
}

// Click clicks the first element matching the selector.
func (s *Session) Click(selector string) (int, error) {
	el, count, err := s.element(selector)
	if err != nil {
		return 0, err
	}
	if err := el.Click(proto.InputMouseButtonLeft, 1); err != nil {
		return count, fmt.Errorf("click failed on %q: %w", selector, err)
	}
	return count, nil
}

// TypeText types into the first element matching the selector.
func (s *Session) TypeText(selector, text string, clear bool) (int, error) {
	el, count, err := s.element(selector)
	if err != nil {
		return 0, err
	}
	if clear {
		if err := el.SelectAllText(); err != nil {
			return count, fmt.Errorf("failed to select existing text in %q: %w", selector, err)
		}
		if err := el.Input(""); err != nil {
			return count, fmt.Errorf("failed to clear %q: %w", selector, err)
		}
	}
	if err := el.Input(text); err != nil {
		return count, fmt.Errorf("failed to type into %q: %w", selector, err)
	}
	return count, nil
}

// Hover moves the mouse over the first element matching the selector.
func (s *Session) Hover(selector string) (int, error) {
	el, count, err := s.element(selector)
	if err != nil {
		return 0, err
	}
	if err := el.Hover(); err != nil {
		return count, fmt.Errorf("hover failed on %q: %w", selector, err)
	}
	return count, nil
}

// PressKey sends a key combination such as "Enter" or "Meta+Enter".
func (s *Session) PressKey(combo string) error {
	parts := strings.Split(combo, "+")
	keys := make([]input.Key, 0, len(parts))
	for _, p := range parts {
		k, err := parseKey(strings.TrimSpace(p))
		if err != nil {
			return err
		}
		keys = append(keys, k)
	}

	ka := s.Page().KeyActions()
	for _, k := range keys[:len(keys)-1] {
		ka = ka.Press(k)
	}
	ka = ka.Type(keys[len(keys)-1])
	if err := ka.Do(); err != nil {
		return fmt.Errorf("failed to press %q: %w", combo, err)
	}
	return nil
}

// ScrollToElement scrolls the first matching element into view.
func (s *Session) ScrollToElement(selector string) (int, error) {
	el, count, err := s.element(selector)
	if err != nil {
		return 0, err
	}
	if err := el.ScrollIntoView(); err != nil {
		return count, fmt.Errorf("scroll failed on %q: %w", selector, err)
	}
	return count, nil
}

// ScrollBy scrolls the page by a pixel delta.
func (s *Session) ScrollBy(x, y float64) error {
	if err := s.Page().Mouse.Scroll(x, y, 1); err != nil {
		return fmt.Errorf("scroll failed: %w", err)
	}
	return nil
}

var namedKeys = map[string]input.Key{
	"enter":      input.Enter,
	"return":     input.Enter,
	"tab":        input.Tab,
	"escape":     input.Escape,
	"esc":        input.Escape,
	"backspace":  input.Backspace,
	"delete":     input.Delete,
	"space":      input.Space,
	"home":       input.Home,
	"end":        input.End,
	"pageup":     input.PageUp,
	"pagedown":   input.PageDown,
	"arrowup":    input.ArrowUp,
	"arrowdown":  input.ArrowDown,
	"arrowleft":  input.ArrowLeft,
	"arrowright": input.ArrowRight,
	"up":         input.ArrowUp,
	"down":       input.ArrowDown,
	"left":       input.ArrowLeft,
	"right":      input.ArrowRight,
	"meta":       input.MetaLeft,
	"cmd":        input.MetaLeft,
	"command":    input.MetaLeft,
	"control":    input.ControlLeft,
	"ctrl":       input.ControlLeft,
	"shift":      input.ShiftLeft,
	"alt":        input.AltLeft,
	"option":     input.AltLeft,
}

func parseKey(name string) (input.Key, error) {
	if k, ok := namedKeys[strings.ToLower(name)]; ok {
		return k, nil
	}
	r := []rune(name)
	if len(r) == 1 {
		k := input.Key(r[0])
		if known(k) {
			return k, nil
		}
	}
	return 0, fmt.Errorf("unknown key %q", name)
}

// known reports whether rod has a keymap entry for the key. Key.Info panics
// when it does not.
func known(k input.Key) (ok bool) {
	defer func() {
		if recover() != nil {
			ok = false
		}
	}()
	_ = k.Info()
	return true
}
