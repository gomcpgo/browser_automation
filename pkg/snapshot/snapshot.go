package snapshot

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/go-rod/rod"
)

// Outline returns a compact list of meaningful elements with a working CSS
// selector for each one.
func Outline(page *rod.Page, p Params) ([]Node, error) {
	if p.MaxDepth <= 0 {
		p.MaxDepth = 12
	}

	obj, err := page.Eval(outlineJS, p.Selector, p.MaxDepth, p.InteractiveOnly)
	if err != nil {
		return nil, fmt.Errorf("snapshot failed: %w", err)
	}

	raw := obj.Value.JSON("", "")
	if raw == "null" {
		return nil, fmt.Errorf("no element matches scope selector %q", p.Selector)
	}

	var nodes []Node
	if err := json.Unmarshal([]byte(raw), &nodes); err != nil {
		return nil, fmt.Errorf("failed to decode snapshot: %w", err)
	}
	return nodes, nil
}

// Format renders the outline as an indented text tree.
func Format(nodes []Node) string {
	var b strings.Builder
	for _, n := range nodes {
		b.WriteString(strings.Repeat("  ", n.Depth))
		b.WriteString(n.Tag)
		if n.Role != "" {
			b.WriteString(" [" + n.Role + "]")
		}
		if n.Text != "" {
			b.WriteString(" \"" + n.Text + "\"")
		}
		b.WriteString("  → " + n.Selector)
		b.WriteString("\n")
	}
	return b.String()
}

// Element returns details of the first element matching the selector.
func Element(page *rod.Page, p ElementParams) (*ElementInfo, error) {
	styles := p.IncludeStyles
	if styles == nil {
		styles = []string{}
	}

	obj, err := page.Eval(elementJS, p.Selector, styles)
	if err != nil {
		return nil, fmt.Errorf("get_element failed: %w", err)
	}

	raw := obj.Value.JSON("", "")
	if raw == "null" {
		return nil, fmt.Errorf("no element matches %q", p.Selector)
	}

	var info ElementInfo
	if err := json.Unmarshal([]byte(raw), &info); err != nil {
		return nil, fmt.Errorf("failed to decode element info: %w", err)
	}
	info.Selector = p.Selector
	return &info, nil
}

// jsSelectorHelper builds a selector for an element, preferring stable
// attributes over generated class names: #id, then data-testid-style hooks,
// then aria-label / title / placeholder, then any data-* attribute, then
// classes, then a structural path. Attribute selectors are tried bare first so
// results read like [aria-label="Updates"] on pages with scrambled classes.
const jsSelectorHelper = `
	const esc = (v) => (window.CSS && CSS.escape) ? CSS.escape(v) : v;
	const uniq = (s) => { try { return document.querySelectorAll(s).length === 1; } catch (e) { return false; } };
	const attrSel = (el, a) => {
		const v = el.getAttribute(a);
		if (!v || v.length > 200 || /[\n\r]/.test(v)) return null;
		const s = '[' + a + '="' + v.replace(/\\/g, '\\\\').replace(/"/g, '\\"') + '"]';
		if (uniq(s)) return s;
		const ts = el.tagName.toLowerCase() + s;
		if (uniq(ts)) return ts;
		return null;
	};
	const selFor = (el) => {
		if (el.id && uniq('#' + esc(el.id))) return '#' + esc(el.id);
		for (const a of ['data-testid', 'data-test-id', 'data-test', 'name', 'aria-label', 'title', 'placeholder']) {
			const s = attrSel(el, a);
			if (s) return s;
		}
		for (const attr of el.attributes) {
			if (attr.name.startsWith('data-')) {
				const s = attrSel(el, attr.name);
				if (s) return s;
			}
		}
		const cls = (typeof el.className === 'string')
			? el.className.trim().split(/\s+/).filter(Boolean).map(esc) : [];
		if (cls.length) {
			const s = el.tagName.toLowerCase() + '.' + cls.join('.');
			if (uniq(s)) return s;
		}
		const parts = [];
		let cur = el;
		while (cur && cur.nodeType === 1 && cur !== document.documentElement) {
			if (cur.id && uniq('#' + esc(cur.id))) { parts.unshift('#' + esc(cur.id)); break; }
			let part = cur.tagName.toLowerCase();
			const parent = cur.parentElement;
			if (parent) {
				const sibs = Array.from(parent.children).filter((c) => c.tagName === cur.tagName);
				if (sibs.length > 1) part += ':nth-of-type(' + (sibs.indexOf(cur) + 1) + ')';
			}
			parts.unshift(part);
			cur = parent;
		}
		return parts.join(' > ');
	};
`

const jsVisibleHelper = `
	const visible = (el) => {
		const st = getComputedStyle(el);
		if (st.display === 'none' || st.visibility === 'hidden') return false;
		const r = el.getBoundingClientRect();
		return r.width > 0 || r.height > 0;
	};
`

const outlineJS = `(scope, maxDepth, interactiveOnly) => {
	` + jsSelectorHelper + jsVisibleHelper + `
	const root = scope ? document.querySelector(scope) : document.body;
	if (!root) return null;

	const SKIP = new Set(['SCRIPT', 'STYLE', 'NOSCRIPT', 'TEMPLATE', 'HEAD', 'META', 'LINK', 'SVG', 'PATH']);
	const INTERACTIVE = 'a[href],button,input,select,textarea,summary,[role],[onclick],[contenteditable],[tabindex]';
	const MAX_NODES = 300;

	const trunc = (s, n) => {
		s = (s || '').replace(/\s+/g, ' ').trim();
		return s.length > n ? s.slice(0, n) + '…' : s;
	};

	const out = [];
	const walk = (el, depth) => {
		if (depth > maxDepth) return;
		for (const child of el.children) {
			if (out.length >= MAX_NODES) return;
			if (SKIP.has(child.tagName)) continue;
			if (!visible(child)) continue;

			const isInteractive = child.matches(INTERACTIVE);
			const isHeading = /^H[1-6]$/.test(child.tagName);
			const ownText = Array.from(child.childNodes)
				.filter((n) => n.nodeType === 3).map((n) => n.nodeValue).join(' ').trim();
			const meaningful = isInteractive || isHeading || ownText.length > 0 ||
				child.tagName === 'IMG' || child.tagName === 'IFRAME';
			const emit = meaningful && (!interactiveOnly || isInteractive);

			if (emit) {
				let role = child.getAttribute('role') || '';
				if (!role && child.tagName === 'INPUT') role = child.getAttribute('type') || 'text';
				let text = ownText;
				if (!text && (isInteractive || child.children.length === 0)) text = child.innerText || '';
				if (!text && (child.tagName === 'IMG' || child.tagName === 'IFRAME')) {
					text = child.getAttribute('alt') || child.getAttribute('src') || '';
				}
				out.push({
					depth: depth,
					tag: child.tagName.toLowerCase(),
					role: role,
					text: trunc(text, 80),
					selector: selFor(child),
				});
			}
			walk(child, emit ? depth + 1 : depth);
		}
	};
	walk(root, 0);
	return out;
}`

const elementJS = `(selector, styleNames) => {
	const els = document.querySelectorAll(selector);
	if (!els.length) return null;
	const el = els[0];
	const st = getComputedStyle(el);
	const r = el.getBoundingClientRect();

	let hiddenBy = '';
	if (st.display === 'none') hiddenBy = 'display: none';
	else if (st.visibility === 'hidden') hiddenBy = 'visibility: hidden';
	else if (parseFloat(st.opacity) === 0) hiddenBy = 'opacity: 0';
	else if (r.width === 0 || r.height === 0) hiddenBy = 'zero size';
	else if (!el.offsetParent && st.position !== 'fixed') hiddenBy = 'ancestor hidden';

	const attrs = {};
	for (const a of el.attributes) attrs[a.name] = a.value;

	const styles = {};
	for (const name of ['display', 'visibility', 'opacity'].concat(styleNames || [])) {
		styles[name] = st.getPropertyValue(name);
	}

	let text = (el.innerText || el.textContent || '').replace(/\s+/g, ' ').trim();
	if (text.length > 500) text = text.slice(0, 500) + '…';

	return {
		match_count: els.length,
		tag: el.tagName.toLowerCase(),
		text: text,
		attributes: attrs,
		bounds: { x: r.x, y: r.y, width: r.width, height: r.height },
		visible: hiddenBy === '',
		hidden_by: hiddenBy,
		styles: styles,
	};
}`
