# Browser Automation MCP Server — Implementation Plan

**Status:** Phase 1 built and working. All 15 tools implemented, `go test ./...` green (unit +
13 fixture-driven integration tests against real headless Chrome), acceptance checklist met — see
[Build status](#build-status) for what changed against this plan and
[Caveats and known issues](#caveats-and-known-issues) for the limits that came out of the build.
**Date:** 2026-08-08 (planned and built)

A Go MCP server that gives an LLM direct browser control for two use cases:

1. **UI testing** — interact with a running web app, inspect console/network/DOM, wait on real
   conditions instead of timers.
2. **Demo capture** — sharp, region-scoped screenshots written to absolute paths, captured on
   condition rather than on a timer.

It replaces the Playwright MCP server for both uses. The pain points driving the design are
documented in this folder:

- `playwright-mcp-experience-report.md` — testing gaps (fixed timeouts, truncated console objects,
  unfiltered network dumps, 3000-token snapshots)
- `playwright-mcp-testing-experience.md` — more testing gaps (iframe blindness, no style
  inspection, no smart waiting)
- `capture-tool-requests.md` — demo-capture friction (sandboxed output paths, no region capture,
  scale tied to viewport, capture-on-timer)

## Decisions made

| Decision | Choice | Rationale |
|---|---|---|
| Library | **go-rod** | Already proven in `html_image_creator`; sits directly on CDP (no Node.js); natively covers element waits, console/network events, iframe access, element screenshots. chromedp (used in `reddit`) is lower-level. |
| Element addressing | **CSS selectors only** | Simplest to build and debug; no hidden ref-map state in the server. `snapshot` output includes suggested selectors. Playwright-style refs can be added later if selectors prove unreliable. |
| V1 scope | **Full Phase 1** | All ~15 tools in one pass — testing and basic capture together. |
| Session model | **One persistent browser + page across tool calls** | The key difference from `html_image_creator` (which launches per shot). Console/network listeners attach at session start and buffer everything, so query tools and `wait_for` see history, not just what happens after they're called. |
| Output style | **Token-thrifty everywhere** | Every tool filters server-side and returns compact results. This alone fixes half the reported pain. |

## Module and structure

Module: `github.com/gomcpgo/browser_automation`, in this folder, following
`mcp/MCP_SERVER_DEVELOPMENT_GUIDE.md`:

```
browser_automation/
├── cmd/main.go            # thin entry + terminal mode flags
├── pkg/
│   ├── handler/           # MCP layer: tools.go, handler.go, param extraction
│   ├── browser/           # session lifecycle: launch (headless/headed), one persistent page, close
│   ├── snapshot/          # compact DOM outline + element inspection
│   ├── capture/           # screenshot: viewport/selector/rect, zoom lever, absolute output paths
│   └── monitor/           # ring buffers for console + network events; wait_for engine
├── docs/                  # this folder
└── run.sh
```

## Phase 1 tools (15)

### Session

| Tool | Params | Notes |
|---|---|---|
| `start_session` | `width`, `height`, `headed`, `zoom` | Headless by default; `headed` for approval-gate demos. `zoom` uses the documented `document.documentElement.style.zoom` sharpness lever (see findings in `capture-tool-requests.md`). |
| `close_session` | — | |
| `navigate` | `url` | Waits for load; re-applies session zoom after navigation. |

### Inspect

| Tool | Params | Notes |
|---|---|---|
| `snapshot` | `selector` (scope), `max_depth`, `interactive_only` | Compact text outline with a suggested CSS selector per element — not a full accessibility-tree YAML dump. |
| `get_element` | `selector`, `include_styles[]` | Text, attributes, bounds, visibility, and requested computed styles. Answers "is the dropdown actually hidden and why". |

### Interact

| Tool | Params |
|---|---|
| `click` | `selector` |
| `type_text` | `selector`, `text`, `clear` |
| `press_key` | `key` (e.g. `"Meta+Enter"`) |
| `hover` | `selector` |
| `scroll` | `selector` (scroll into view) or `x`/`y` delta |

### Wait

| Tool | Params | Notes |
|---|---|---|
| `wait_for` | `condition`, `timeout_ms`, condition-specific params | Conditions: `text_visible`, `text_gone`, `selector_visible`, `selector_hidden`, `console_matches`, `network_idle`, `js_true`. Returns elapsed time. This is the `sleep 45` killer. |

### Observe

| Tool | Params | Notes |
|---|---|---|
| `get_console` | `pattern`, `levels[]`, `since_ms`, `max_results`, `expand_depth` | Objects JSON-stringified server-side to `expand_depth` — no more `{data: Object}`. |
| `get_requests` | `url_pattern`, `method`, `status[]`, `include_body`, `since_ms`, `max_results` | Response bodies captured via CDP response events. |
| `evaluate` | `js` | Structured JSON return. Covers app-state inspection (e.g. Pinia stores) generically — no framework-specific tools. |

### Capture

| Tool | Params | Notes |
|---|---|---|
| `screenshot` | `output_path` (absolute, required), `selector` OR `rect {x,y,w,h}`, `zoom` | Writes exactly where told (fixes the `.playwright-mcp/` sandbox friction). Element capture via rod's element screenshot; rect capture via CDP clip. Returns the exact path written. |

## Later phases

**Phase 2 — demo polish:**
- Persistent injected CSS/JS, re-applied on every navigation (devtools-button hiding)
- Full-page scroll-and-stitch capture (also fixes caveat 4, viewport-clipped element capture)
- Named viewport presets
- Capture-on-condition as one fused call (`wait_for` + `screenshot`)

**Robustness, promoted out of the build (see [Caveats and known issues](#caveats-and-known-issues)):**
- Dialog handling + per-call page timeouts, so an `alert()` cannot stall the session (caveat 1)
- Always re-apply the effective zoom before capture (caveat 2)

**Phase 3 — deferred (complex demo captures):**
- Typing animation frames ("type over N ms, frame every X%")
- Burst capture during streaming
- Synthetic cursor overlay with click-ripple
- Video

**Non-goals:** Vue/Pinia-specific tools (generic `evaluate` covers it), session recording/replay,
visual diffing.

## Testing strategy

Three layers, all runnable via `run.sh` without any external app.

### 1. Unit tests (`go test ./pkg/...`)

Pure-Go tests, no browser:

- **handler**: parameter extraction from `map[string]interface{}` — required/optional fields,
  float64→int coercion, bad-type errors, the `selector`-vs-`rect` exclusivity on `screenshot`.
- **monitor**: console/network ring-buffer filtering — pattern, level, `since_ms`, `max_results`;
  object expansion depth; `url_pattern` matching.
- **capture**: output-path validation (absolute path required, directory creation), rect math.

### 2. Fixture-based integration tests (`go test ./test/`)

A local fixture HTML page, served by `httptest` from within the test (same pattern
`html_image_creator` uses), exercised through the real handler + a real headless Chrome. The
fixture is built to prove each pain-point fix deterministically:

`test/fixtures/testpage.html` contains:

- **Delayed text** — a node whose text changes from "Loading..." to "Ready" after 2s
  (`setTimeout`) → proves `wait_for text_gone` / `text_visible` beat a fixed timeout.
- **Console object logging** — `console.log('event', {type: 'credit:balance:changed', data:
  {balance: 488}})` on load → proves `get_console` expands objects instead of `{data: Object}`.
- **A fetch call** — button that fetches `/api/test` (served by the same fixture server, returns
  JSON) → proves `get_requests` filtering and `include_body`.
- **Interactive elements** — a text input, a button that toggles a dropdown
  (`display:none` ↔ visible) → proves `click`, `type_text`, `get_element` computed styles
  (the "Export button mystery"), `wait_for selector_visible`.
- **An iframe** with distinct inner content → documents current iframe behavior (full support is
  later; the test pins down what v1 does).
- **A tall section** below the fold → proves `scroll` + element `screenshot` of an off-screen
  element.
- **A high-detail region** (small text) → proves the zoom lever: capture at `zoom: 2` and assert
  the output image is ~2× the element's CSS pixel size.

Integration tests skip cleanly (`t.Skip`) when Chrome is not installed, so `go test ./...` stays
green on machines without it.

### 3. Terminal mode (manual/exploratory)

Per the dev guide, `cmd/main.go` gets flags so every tool is drivable from the shell before any
MCP client is involved:

```
./run.sh serve-fixture          # serve test/fixtures/ on a local port for manual poking
go run ./cmd -navigate URL -snapshot
go run ./cmd -navigate URL -shot /abs/path.png -selector ".card" -zoom 2
go run ./cmd -navigate URL -wait "text_visible:Ready" -timeout 10000
go run ./cmd -navigate URL -console -pattern "credit"
```

Terminal mode reuses the same handler layer as MCP mode, so what works in the terminal works over
MCP.

### Acceptance checklist (from the pain-point docs)

All verified by `go test ./test/`, one test per item:

- [x] A wait that needs 30s+ succeeds with `timeout_ms: 45000` and returns early when the
      condition is met (no fixed 5s ceiling, no `sleep`). — `TestWaitBeatsFixedTimeout`
- [x] A logged object is fully readable in `get_console` output without touching app source.
      — `TestConsoleObjectExpansion`
- [x] `get_requests` with a URL pattern returns only matching calls, with response body.
      — `TestRequestFilteringWithBody`
- [x] `snapshot` of the fixture page is a fraction of the size of a full a11y dump and every
      listed element's suggested selector actually works. — `TestSnapshotIsCompactAndSelectorsWork`
- [x] `screenshot` writes to an arbitrary absolute path and returns that path.
      — `TestScreenshotToAbsolutePath`
- [x] Element screenshot at `zoom: 2` is sharp (pixel dimensions ≈ 2× CSS box).
      — `TestZoomLeverProducesSharperCapture`
- [x] `get_element` on the hidden dropdown reports `display: none`.
      — `TestHiddenElementReportsReason`

## Build status

Everything in Phase 1 shipped: all 15 tools, the package layout above, unit tests, fixture-based
integration tests and terminal mode. Nothing was dropped. `go test ./...`, `go vet ./...` and
`gofmt -l .` are clean; `./run.sh demo` exercises the whole flow against the bundled fixture.

### How things were actually implemented

- **Console capture** patches `console.log/info/warn/debug/error` in the page and forwards a
  serialized payload through a CDP binding (`Runtime.addBinding` + `Page.addScriptToEvaluateOnNewDocument`),
  rather than reading remote-object previews. That is what makes nested objects readable. Uncaught
  exceptions come in separately via `Runtime.exceptionThrown` and are recorded at level `error`.
- **`EvalOnNewDocument` takes a *script*, not a function expression** — unlike `page.Eval`. The patch
  has to be wrapped in an IIFE there. Getting this wrong fails silently: the binding exists, the
  patch never runs, and the console buffer just stays empty. This was the one real bug in the build.
- **Response bodies are fetched eagerly** when `Network.loadingFinished` fires, from a separate
  goroutine (calling CDP from inside the event-loop callback would deadlock). Asking for a body later,
  after a navigation, is too late — Chrome has dropped it.
- **`snapshot` emits only meaningful elements** — interactive, headings, elements with their own text,
  `img`, `iframe` — and skips wrapper divs, recursing without emitting so the indent stays a clean
  outline. Capped at 300 nodes, text truncated to 80 chars.
- **Selector suggestion** tries, in order: unique `#id`, then a unique
  `tag[data-testid|data-test-id|data-test|name|aria-label|placeholder="..."]`, then a unique
  `tag.class.combo`, then a positional `tag:nth-of-type(n) > ...` path. Uniqueness is verified with
  `querySelectorAll(...).length === 1` before a selector is offered.

### Deviations from the plan

| Area | Plan | Built | Why |
|---|---|---|---|
| Ambiguous selectors | not specified | first match is used, `match_count` returned | Playwright's strict mode was listed as friction in `capture-tool-requests.md`; reporting beats erroring |
| `get_element` | text, attributes, bounds, visibility, styles | also `hidden_by` naming the cause | Directly answers the "Export button mystery"; `display`/`visibility`/`opacity` are always returned |
| Terminal mode | four example flags | added `-serve-fixtures`, `-wait-after`, `-body`, `-url-pattern` | Flags run in a fixed order, so a post-interaction wait needs its own flag; console and request filters needed separating |
| Fixture serving | `run.sh serve-fixture` | same, plus the fixture is a Go package embedding the HTML | One definition shared by `httptest` in the tests and the terminal flag |

### Defaults and limits chosen during the build

- `wait_for`: `timeout_ms` 10000, `idle_ms` 500, polled every 100ms.
- `get_console`: `max_results` 50, `expand_depth` 5. Page-side serialization caps at depth 8,
  60 keys per object, 100 array items, 2000 chars per string — `expand_depth` can trim below that
  but never recover beyond it.
- `get_requests`: `max_results` 20, bodies buffered for text responses up to 64KB.
- Buffers: 1000 console entries and 500 requests, oldest evicted.
- `start_session`: 1280×800, headless, zoom 1, `DeviceScaleFactor` 1 (zoom, not DPR, is the
  sharpness lever).

## Caveats and known issues

Ordered by how likely they are to bite.

### 1. An unhandled JS dialog stalls the session (no workaround in v1)

`alert()`, `confirm()` or `prompt()` blocks the page, and every later tool call blocks with it —
**including `wait_for`, whose `timeout_ms` does not fire**, because the poll's `Eval` never returns.
Verified: a call issued after an `alert()` was still hanging 15s later. There is no dialog handler
and no per-call CDP timeout. Fix when it matters: register `page.HandleDialog` at session start
(auto-dismiss, and report dialogs through `get_console`), and give each tool call a bounded page
timeout.

### 2. Session zoom is lost on a page-initiated reload

`navigate` re-applies zoom, and SPA route changes (`history.pushState`) keep it, but if the app
itself reloads the document the zoom resets to 1 while the session still believes it is 2. The
per-shot zoom in `screenshot` compares against the session value, so it skips re-applying and the
capture comes back at 1×. Verified with `location.reload()`. Fix: have `screenshot` always apply the
effective zoom before capturing instead of comparing.

### 3. `evaluate` takes an expression, not statements

The JS is wrapped as `() => (your_js)`, so `const x = 1; return x` is a syntax error. Multi-statement
code must be an IIFE: `(() => { const x = 1; return x })()`. Works, but it is not obvious from the
tool description.

### 4. Element capture is clipped to the viewport

An element taller or wider than the viewport is cut off — capturing `body` on the fixture (1680px
tall in an 800px viewport) returns 1280×800, not the full page. Full-page capture is Phase 2
(scroll-and-stitch); until then, size the viewport to the content or capture in sections.

### 5. Iframe contents are not addressable

Selectors only run in the main frame, so `click`/`get_element`/`screenshot` cannot reach inside an
`<iframe>`; the frame element itself appears in `snapshot` (with its `src`) and in `get_element`.
Pinned by `TestIframeBehaviour`. Console output *is* captured from same-origin iframes (verified —
the injected script applies to every frame in the target). Cross-origin (out-of-process) iframes and
web workers are separate CDP targets and are expected not to be captured; not verified.

### 6. Buffer and capture gaps

- A response body is fetched asynchronously after `loadingFinished`, so `get_requests` called
  immediately after an action may return the record without its body. Wait for the condition first.
- Binary (base64) bodies and anything over 64KB are skipped; the request metadata is still recorded.
- Responses served from Chrome's memory cache may never produce a body.
- `network_idle` only knows about CDP network events. WebSocket traffic is invisible to it, and a
  page that polls on a timer never goes idle.
- `console_matches` searches the whole buffer by default, so a stale match from earlier in the
  session satisfies the wait instantly. Scope it with `since_ms` when that matters.
- Regexes are Go RE2: no lookahead or backreferences.

### 7. One page, one caller

The session holds a single page. New tabs and `window.open` popups are not tracked, and downloads
are not handled. The handler mutex guards the session pointer, not the page: concurrent tool calls
would drive the same page at once. MCP calls are sequential in practice, so this has not been an
issue.

### 8. Smaller sharp edges

- `press_key` only accepts keys in rod's keymap (an unknown name is a clear error). Combos hold all
  but the last key, then type the last.
- `type_text` with `clear` uses select-all then insert; a custom editor that ignores `insertText`
  will not clear.
- `rect` coordinates are viewport CSS pixels *after* zoom is applied — zoom reflows layout, so
  locate the region at the zoom you intend to capture at.
- Applying zoom sleeps 150ms for reflow, so a zoomed screenshot costs ~300ms extra.
- If Chrome is not installed, rod downloads one on first launch, which makes that run slow.
