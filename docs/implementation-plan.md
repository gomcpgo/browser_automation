# Browser Automation MCP Server — Implementation Plan

**Status:** Phase 1 built. All 15 tools implemented and covered by tests; see `../README.md`.
**Date:** 2026-08-08

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
- Full-page scroll-and-stitch capture
- Named viewport presets
- Capture-on-condition as one fused call (`wait_for` + `screenshot`)

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

### Notes from the build

- `EvalOnNewDocument` takes a *script*, not a function expression: the console patch has to be
  wrapped in an IIFE there, unlike `page.Eval`.
- Console arguments are captured by patching `console.*` in the page and forwarding a serialized
  payload through a CDP binding (`Runtime.addBinding`), rather than reading remote object previews.
  That is what makes nested objects readable.
- Response bodies must be fetched when `Network.loadingFinished` fires; asking for them later, after
  a navigation, is too late. They are buffered eagerly for text responses up to 64KB.
