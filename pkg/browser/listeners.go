package browser

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/go-rod/rod/lib/proto"
)

const consoleBinding = "__baConsole"

// maxBodyBytes caps how much of a response body is buffered per request.
const maxBodyBytes = 64 * 1024

// attachListeners installs the console patch and starts the event pumps. It
// runs once at session start so buffers cover the whole session.
func (s *Session) attachListeners() error {
	if err := (proto.RuntimeEnable{}).Call(s.page); err != nil {
		return fmt.Errorf("failed to enable runtime events: %w", err)
	}
	if err := (proto.RuntimeAddBinding{Name: consoleBinding}).Call(s.page); err != nil {
		return fmt.Errorf("failed to add console binding: %w", err)
	}
	// EvalOnNewDocument runs a script, not a function, so the patch is wrapped
	// in an IIFE there and called directly below.
	if _, err := s.page.EvalOnNewDocument("(" + consolePatchJS + ")()"); err != nil {
		return fmt.Errorf("failed to install console patch: %w", err)
	}
	// Also patch the current document, which was created before the hook.
	if _, err := s.page.Eval(consolePatchJS); err != nil {
		return fmt.Errorf("failed to install console patch: %w", err)
	}
	if err := (proto.NetworkEnable{}).Call(s.page); err != nil {
		return fmt.Errorf("failed to enable network events: %w", err)
	}
	// Page events carry the dialog notifications handled below.
	if err := (proto.PageEnable{}).Call(s.page); err != nil {
		return fmt.Errorf("failed to enable page events: %w", err)
	}

	go s.page.EachEvent(
		func(e *proto.RuntimeBindingCalled) {
			if e.Name == consoleBinding {
				s.onConsolePayload(e.Payload)
			}
		},
		func(e *proto.RuntimeExceptionThrown) {
			s.mon.AddConsole("error", []interface{}{exceptionText(e)})
		},
		func(e *proto.NetworkRequestWillBeSent) {
			s.mon.RequestStarted(string(e.RequestID), e.Request.URL, e.Request.Method)
		},
		func(e *proto.NetworkResponseReceived) {
			s.mon.ResponseReceived(string(e.RequestID), e.Response.Status, e.Response.MIMEType)
		},
		func(e *proto.NetworkLoadingFinished) {
			id := string(e.RequestID)
			s.mon.RequestFinished(id, int(e.EncodedDataLength))
			// Bodies must be fetched while Chrome still holds them, but not from
			// inside the event loop goroutine.
			go s.captureBody(id, int(e.EncodedDataLength))
		},
		func(e *proto.NetworkLoadingFailed) {
			s.mon.RequestFailed(string(e.RequestID), e.ErrorText)
		},
		func(e *proto.PageJavascriptDialogOpening) {
			go s.dismissDialog(e)
		},
	)()

	return nil
}

// onConsolePayload decodes one patched-console message.
func (s *Session) onConsolePayload(payload string) {
	var msg struct {
		Level string        `json:"level"`
		Args  []interface{} `json:"args"`
	}
	if err := json.Unmarshal([]byte(payload), &msg); err != nil {
		s.mon.AddConsole("log", []interface{}{payload})
		return
	}
	s.mon.AddConsole(msg.Level, msg.Args)
}

// dismissDialog closes a JS dialog and records it. An unhandled alert, confirm
// or prompt blocks the page and every later tool call with it, so dialogs are
// always dismissed; the message shows up in get_console so it is not lost.
func (s *Session) dismissDialog(e *proto.PageJavascriptDialogOpening) {
	err := proto.PageHandleJavaScriptDialog{Accept: false}.Call(s.page)

	msg := fmt.Sprintf("dialog dismissed (%s): %s", e.Type, e.Message)
	if err != nil {
		msg = fmt.Sprintf("dialog (%s) could not be dismissed: %s: %v", e.Type, e.Message, err)
	}
	s.mon.AddConsole("warn", []interface{}{msg})
}

// captureBody stores a text response body so get_requests can return it later,
// after the page may have navigated away.
func (s *Session) captureBody(id string, size int) {
	if size > maxBodyBytes {
		return
	}
	res, err := proto.NetworkGetResponseBody{RequestID: proto.NetworkRequestID(id)}.Call(s.page)
	if err != nil || res.Base64Encoded {
		return
	}
	body := res.Body
	if len(body) > maxBodyBytes {
		body = body[:maxBodyBytes] + "…[truncated]"
	}
	s.mon.SetBody(id, body)
}

func exceptionText(e *proto.RuntimeExceptionThrown) string {
	d := e.ExceptionDetails
	if d == nil {
		return "uncaught exception"
	}
	parts := []string{d.Text}
	if d.Exception != nil && d.Exception.Description != "" {
		parts = append(parts, d.Exception.Description)
	}
	if d.URL != "" {
		parts = append(parts, fmt.Sprintf("at %s:%d", d.URL, d.LineNumber))
	}
	return strings.Join(parts, " ")
}

// consolePatchJS forwards console calls to the CDP binding with arguments
// serialized in full, so objects arrive readable instead of "{data: Object}".
const consolePatchJS = `() => {
	if (window.__baInstalled) return;
	window.__baInstalled = true;

	const MAX_DEPTH = 8, MAX_KEYS = 60, MAX_ITEMS = 100, MAX_STR = 2000;

	const ser = (v, d, seen) => {
		if (v === undefined) return '[undefined]';
		if (v === null) return null;
		const t = typeof v;
		if (t === 'string') return v.length > MAX_STR ? v.slice(0, MAX_STR) + '…' : v;
		if (t === 'number') return Number.isFinite(v) ? v : String(v);
		if (t === 'boolean') return v;
		if (t === 'bigint' || t === 'symbol') return String(v);
		if (t === 'function') return '[Function ' + (v.name || 'anonymous') + ']';
		if (v instanceof Error) {
			return { error: v.message, stack: String(v.stack || '').split('\n').slice(0, 6).join('\n') };
		}
		if (typeof Node !== 'undefined' && v instanceof Node) {
			const tag = (v.nodeName || '').toLowerCase();
			const id = v.id ? '#' + v.id : '';
			const cls = v.className && typeof v.className === 'string' ? '.' + v.className.trim().split(/\s+/).join('.') : '';
			return '<' + tag + id + cls + '>';
		}
		if (d >= MAX_DEPTH) return '[max depth]';
		if (seen.has(v)) return '[circular]';
		seen.add(v);
		try {
			if (Array.isArray(v)) {
				const arr = v.slice(0, MAX_ITEMS).map((x) => ser(x, d + 1, seen));
				if (v.length > MAX_ITEMS) arr.push('…' + (v.length - MAX_ITEMS) + ' more');
				return arr;
			}
			if (v instanceof Map) {
				const o = {};
				let i = 0;
				for (const [k, val] of v) {
					if (i++ >= MAX_KEYS) { o['…'] = 'more'; break; }
					o[String(k)] = ser(val, d + 1, seen);
				}
				return o;
			}
			if (v instanceof Set) return ser(Array.from(v), d, seen);
			const o = {};
			let i = 0;
			for (const k of Object.keys(v)) {
				if (i++ >= MAX_KEYS) { o['…'] = 'more'; break; }
				try { o[k] = ser(v[k], d + 1, seen); } catch (e) { o[k] = '[getter threw]'; }
			}
			return o;
		} finally {
			seen.delete(v);
		}
	};

	for (const level of ['log', 'info', 'warn', 'error', 'debug']) {
		const orig = console[level] ? console[level].bind(console) : null;
		console[level] = (...args) => {
			try {
				window.` + consoleBinding + `(JSON.stringify({
					level: level,
					args: args.map((a) => ser(a, 0, new WeakSet())),
				}));
			} catch (e) {}
			if (orig) orig(...args);
		};
	}
}`
