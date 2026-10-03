package snapshot

import (
	"encoding/json"
	"fmt"

	"github.com/go-rod/rod"
)

// FindParams describes what to look for. Criteria are ANDed; at least one of
// Text, AriaLabel, Role, Title or Placeholder is required.
type FindParams struct {
	Text        string `json:"text"`
	AriaLabel   string `json:"aria_label"`
	Role        string `json:"role"`
	Title       string `json:"title"`
	Placeholder string `json:"placeholder"`
	Within      string `json:"within"`
	VisibleOnly bool   `json:"visible_only"`
	MaxResults  int    `json:"max_results"`
}

// Validate checks that there is something to search for.
func (p FindParams) Validate() error {
	if p.Text == "" && p.AriaLabel == "" && p.Role == "" && p.Title == "" && p.Placeholder == "" {
		return fmt.Errorf("pass at least one of text, aria_label, role, title or placeholder")
	}
	return nil
}

// Match is one element found by Find.
type Match struct {
	Selector string `json:"selector"`
	Tag      string `json:"tag"`
	Role     string `json:"role,omitempty"`
	Name     string `json:"name,omitempty"`
	Text     string `json:"text,omitempty"`
	Visible  bool   `json:"visible"`
	Rect     Bounds `json:"rect"`
}

// FindResult is the list of matches plus how many there were in total.
type FindResult struct {
	Total   int     `json:"total"`
	Matches []Match `json:"matches"`
}

// Find locates elements by visible text, ARIA attributes and role rather than
// by id or class, for pages whose class names are generated. When one match
// contains another only the innermost is kept, so a text search returns the
// span holding the text rather than every ancestor up to body.
func Find(page *rod.Page, p FindParams) (*FindResult, error) {
	if err := p.Validate(); err != nil {
		return nil, err
	}
	if p.MaxResults <= 0 {
		p.MaxResults = 10
	}

	obj, err := page.Eval(findJS, p)
	if err != nil {
		return nil, fmt.Errorf("find failed: %w", err)
	}

	raw := obj.Value.JSON("", "")
	if raw == "null" {
		return nil, fmt.Errorf("no element matches within selector %q", p.Within)
	}

	var res FindResult
	if err := json.Unmarshal([]byte(raw), &res); err != nil {
		return nil, fmt.Errorf("failed to decode find result: %w", err)
	}
	if res.Matches == nil {
		res.Matches = []Match{}
	}
	return &res, nil
}

const jsRoleHelper = `
	const INPUT_ROLES = { checkbox: 'checkbox', radio: 'radio', range: 'slider', number: 'spinbutton',
		search: 'searchbox', submit: 'button', reset: 'button', button: 'button', image: 'button',
		hidden: '', file: '', color: '', date: '', time: '', 'datetime-local': '', month: '', week: '' };
	const TAG_ROLES = { button: 'button', textarea: 'textbox', select: 'combobox', option: 'option',
		li: 'listitem', ul: 'list', ol: 'list', img: 'img', nav: 'navigation', main: 'main',
		header: 'banner', footer: 'contentinfo', aside: 'complementary', form: 'form', table: 'table',
		tr: 'row', td: 'cell', th: 'columnheader', dialog: 'dialog', article: 'article',
		section: 'region', summary: 'button', details: 'group', progress: 'progressbar',
		hr: 'separator', h1: 'heading', h2: 'heading', h3: 'heading', h4: 'heading', h5: 'heading',
		h6: 'heading', menu: 'list', fieldset: 'group', output: 'status' };
	const roleOf = (el) => {
		const explicit = el.getAttribute('role');
		if (explicit) return explicit.trim().split(/\s+/)[0];
		const tag = el.tagName.toLowerCase();
		if (tag === 'a' || tag === 'area') return el.hasAttribute('href') ? 'link' : '';
		if (tag === 'input') {
			const t = (el.getAttribute('type') || 'text').toLowerCase();
			return t in INPUT_ROLES ? INPUT_ROLES[t] : 'textbox';
		}
		if (el.isContentEditable && el.hasAttribute('contenteditable')) return 'textbox';
		return TAG_ROLES[tag] || '';
	};
	const nameOf = (el) => {
		const own = el.getAttribute('aria-label');
		if (own) return own.trim();
		const by = el.getAttribute('aria-labelledby');
		if (by) {
			const t = by.split(/\s+/).map((id) => { const n = document.getElementById(id); return n ? n.innerText || n.textContent : ''; }).join(' ').trim();
			if (t) return t;
		}
		for (const a of ['title', 'alt', 'placeholder']) {
			const v = el.getAttribute(a);
			if (v) return v.trim();
		}
		return '';
	};
`

const findJS = `(p) => {
	` + jsSelectorHelper + jsVisibleHelper + jsRoleHelper + `
	const root = p.within ? document.querySelector(p.within) : document.body;
	if (!root) return null;

	// Lower-case and collapse whitespace (including NBSP) so "log in" matches "log\u00a0in".
	const lc = (s) => (s || '').toLowerCase().replace(/\s+/g, ' ').trim();
	const trunc = (s, n) => {
		s = (s || '').replace(/\s+/g, ' ').trim();
		return s.length > n ? s.slice(0, n) + '…' : s;
	};
	const text = lc(p.text), label = lc(p.aria_label), title = lc(p.title), placeholder = lc(p.placeholder);
	const SKIP = new Set(['SCRIPT', 'STYLE', 'NOSCRIPT', 'TEMPLATE', 'HEAD', 'META', 'LINK']);
	const MAX_CANDIDATES = 500;

	const candidates = [];
	for (const el of root.querySelectorAll('*')) {
		if (SKIP.has(el.tagName)) continue;
		if (p.role && roleOf(el) !== p.role) continue;
		if (label && !lc(el.getAttribute('aria-label')).includes(label)) continue;
		if (title && !lc(el.getAttribute('title')).includes(title)) continue;
		if (placeholder && !lc(el.getAttribute('placeholder')).includes(placeholder)) continue;
		// textContent is a cheap superset check; innerText (layout-aware) confirms.
		if (text && !lc(el.textContent).includes(text)) continue;
		if (text && !lc(el.innerText).includes(text)) continue;
		if (p.visible_only && !visible(el)) continue;
		candidates.push(el);
		if (candidates.length >= MAX_CANDIDATES) break;
	}

	// Keep only the innermost matches.
	const inner = candidates.filter((el) => !candidates.some((other) => other !== el && el.contains(other)));

	const matches = inner.slice(0, p.max_results).map((el) => {
		const r = el.getBoundingClientRect();
		return {
			selector: selFor(el),
			tag: el.tagName.toLowerCase(),
			role: roleOf(el),
			name: trunc(nameOf(el), 120),
			text: trunc(el.innerText || el.value || '', 120),
			visible: visible(el),
			rect: { x: r.x, y: r.y, width: r.width, height: r.height },
		};
	});
	return { total: inner.length, matches: matches };
}`
