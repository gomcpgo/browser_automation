package browser

import (
	"fmt"
	"strings"
	"unicode/utf8"

	"github.com/go-rod/rod/lib/proto"
)

// Newline modes for InsertText.
const (
	NewlineShiftEnter = "shift_enter"
	NewlineEnter      = "enter"
	NewlineNone       = "none"
)

// maxInsertEcho caps the element text echoed back after an insert.
const maxInsertEcho = 2000

const selectAllJS = `() => {
	if (typeof this.select === 'function' && 'value' in this) { this.select(); return; }
	const range = document.createRange();
	range.selectNodeContents(this);
	const sel = window.getSelection();
	sel.removeAllRanges();
	sel.addRange(range);
}`

// InsertResult reports what InsertText did.
type InsertResult struct {
	Selector      string `json:"selector"`
	MatchCount    int    `json:"match_count"`
	LinesInserted int    `json:"lines_inserted"`
	Text          string `json:"text"`
}

// InsertText fills an editor where Enter means "submit" (chat composers,
// rich-text editors) and inserts any Unicode text faithfully. Each line goes in
// through CDP Input.insertText, which fires beforeinput/input the way an IME
// commit does, so Lexical, ProseMirror and Slate accept it and complex scripts
// are not broken up into key events. Between lines it sends one key event
// (Shift+Enter by default). It never sends a trailing Enter: submitting is a
// separate, deliberate PressKey.
func (s *Session) InsertText(selector, text, newline string, clear bool) (*InsertResult, error) {
	switch newline {
	case "":
		newline = NewlineShiftEnter
	case NewlineShiftEnter, NewlineEnter, NewlineNone:
	default:
		return nil, fmt.Errorf("newline must be %q, %q or %q, got %q",
			NewlineShiftEnter, NewlineEnter, NewlineNone, newline)
	}

	el, count, err := s.element(selector)
	if err != nil {
		return nil, err
	}
	if err := el.Focus(); err != nil {
		return nil, fmt.Errorf("failed to focus %q: %w", selector, err)
	}

	if clear {
		// rod's SelectAllText only handles inputs; contenteditable needs a range.
		if _, err := el.Eval(selectAllJS); err != nil {
			return nil, fmt.Errorf("failed to select existing text in %q: %w", selector, err)
		}
		if err := s.PressKey("Backspace"); err != nil {
			return nil, fmt.Errorf("failed to clear %q: %w", selector, err)
		}
	}

	text = strings.ReplaceAll(text, "\r\n", "\n")
	var lines []string
	if newline == NewlineNone {
		lines = []string{strings.ReplaceAll(text, "\n", " ")}
	} else {
		lines = strings.Split(text, "\n")
	}

	page := s.Page()
	for i, line := range lines {
		if i > 0 {
			combo := "Shift+Enter"
			if newline == NewlineEnter {
				combo = "Enter"
			}
			if err := s.PressKey(combo); err != nil {
				return nil, fmt.Errorf("failed to insert line break after line %d: %w", i, err)
			}
		}
		if line == "" {
			continue
		}
		if err := (proto.InputInsertText{Text: line}).Call(page); err != nil {
			return nil, fmt.Errorf("failed to insert line %d into %q: %w", i+1, selector, err)
		}
	}

	echo, err := el.Text()
	if err != nil {
		echo = ""
	}
	if utf8.RuneCountInString(echo) > maxInsertEcho {
		r := []rune(echo)
		echo = string(r[:maxInsertEcho]) + "…"
	}

	return &InsertResult{
		Selector:      selector,
		MatchCount:    count,
		LinesInserted: len(lines),
		Text:          echo,
	}, nil
}
