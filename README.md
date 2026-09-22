# Browser Automation MCP Server

A Go MCP server that gives an LLM direct browser control, built on [go-rod](https://github.com/go-rod/rod)
(CDP, no Node.js). It replaces the Playwright MCP server for two uses:

1. **UI testing** — interact with a running web app, inspect console/network/DOM, wait on real
   conditions instead of timers.
2. **Demo capture** — sharp, region-scoped screenshots written to absolute paths.

See `docs/implementation-plan.md` for the design and `docs/*.md` for the pain points it addresses.

## What is different

- **One persistent browser and page** across tool calls. Console and network listeners attach at
  session start and buffer everything, so `get_console`, `get_requests` and `wait_for` see history,
  not just what happens after they are called.
- **`wait_for` on real conditions** with your own timeout — no fixed ceiling, no `sleep`.
- **Console objects arrive fully serialized.** `console.log('event', {data: {balance: 488}})` reads
  as `{"data":{"balance":488}}`, not `{data: Object}`.
- **Server-side filtering everywhere** so results stay small.
- **Screenshots go exactly where you say**, with element and rect capture, and page zoom as the
  sharpness lever.

## Setup

Requires Chrome or Chromium installed locally (rod finds it; it downloads one if absent).

```bash
./run.sh build      # builds bin/browser_automation
./run.sh test       # unit + integration tests (integration skips without Chrome)
```

MCP client configuration:

```json
{
  "mcpServers": {
    "browser_automation": {
      "command": "/absolute/path/to/browser_automation/bin/browser_automation",
      "env": {
        "BROWSER_AUTOMATION_CHROME_PATH": "/Applications/Google Chrome.app/Contents/MacOS/Google Chrome"
      }
    }
  }
}
```

## Configuration

| Variable | Required | Purpose |
|---|---|---|
| `BROWSER_AUTOMATION_CHROME_PATH` | no | Browser binary to launch. Unset means auto-detect, which downloads Chromium if nothing is installed. Set it to pin a specific browser (Chrome, Brave, Edge, Canary) or to avoid that download. |

Point it at the executable, not the macOS `.app` bundle — the path must end in
`Contents/MacOS/Google Chrome`. A missing path or a directory fails at startup with the reason, not
later on the first `start_session`.

Everything else is a per-call tool parameter: viewport size, `headed`, and `zoom` belong to
`start_session`, and waits take their own `timeout_ms`.

## Tools

### Session

| Tool | Params |
|---|---|
| `start_session` | `width`, `height`, `headed`, `zoom` |
| `close_session` | — |
| `navigate` | `url` |

`zoom` re-renders the page at that scale and is re-applied after every navigation. It is the way to
get sharp captures: an element screenshot at `zoom: 2` has roughly twice the pixels, genuinely
re-rendered rather than upscaled. Layout reflows at the new zoom.

### Inspect

| Tool | Params |
|---|---|
| `snapshot` | `selector`, `max_depth`, `interactive_only` |
| `get_element` | `selector`, `include_styles[]` |
| `evaluate` | `js` |

`snapshot` prints one line per meaningful element with a working CSS selector:

```
h1 "Fixture Page"  → #title
input [text]  → #name-input
button "Toggle dropdown"  → #toggle-dropdown
p "This section starts far below the initial viewport."  → #tall > p
```

`get_element` reports whether an element is really visible and what is hiding it (`display: none`,
`visibility: hidden`, `opacity: 0`, zero size, hidden ancestor), plus any computed styles you ask
for. `evaluate` covers app-state inspection generically — no framework-specific tools.

### Interact

| Tool | Params |
|---|---|
| `click` | `selector` |
| `type_text` | `selector`, `text`, `clear` |
| `press_key` | `key`, e.g. `"Meta+Enter"` |
| `hover` | `selector` |
| `scroll` | `selector` (scroll into view) or `x`/`y` delta |

Selectors are plain CSS. When one matches several elements the first is used and `match_count`
reports the total, so ambiguity is visible instead of fatal.

### Wait

| Tool | Params |
|---|---|
| `wait_for` | `condition`, `timeout_ms`, plus condition-specific params |

Conditions: `text_visible`, `text_gone` (`text`), `selector_visible`, `selector_hidden`
(`selector`), `console_matches` (`pattern`, optional `since_ms`), `network_idle` (`idle_ms`),
`js_true` (`js`). Returns `elapsed_ms`; a timeout is an error that also reports elapsed time.

### Observe

| Tool | Params |
|---|---|
| `get_console` | `pattern`, `levels[]`, `since_ms`, `max_results`, `expand_depth` |
| `get_requests` | `url_pattern`, `method`, `status[]`, `include_body`, `since_ms`, `max_results` |

Response bodies are captured when the response arrives (text responses up to 64KB), so they survive
a later navigation. Buffers hold the last 1000 console entries and 500 requests.

### Capture

| Tool | Params |
|---|---|
| `screenshot` | `output_path` (absolute, required), `selector` **or** `rect {x,y,width,height}`, `zoom` |

Returns the exact path written plus the image dimensions. `zoom` here applies to that shot only and
the session zoom is restored afterwards.

## Terminal mode

Every tool is drivable from the shell against the bundled fixture page, using the same handler layer
as MCP mode:

```bash
./run.sh serve-fixture              # serve the test page on :8899
./run.sh demo                       # scripted end-to-end run against it

go run ./cmd -navigate URL -snapshot
go run ./cmd -navigate URL -shot /abs/path.png -selector ".card" -zoom 2
go run ./cmd -navigate URL -wait "text_visible:Ready" -timeout 45000
go run ./cmd -navigate URL -console -pattern "credit"
go run ./cmd -navigate URL -click "#send" -wait-after "text_visible:Done" -requests -url-pattern "/api/" -body
```

`-wait` runs before interactions, `-wait-after` runs after them. `go run ./cmd -h` lists all flags.

## Testing

- `go test ./pkg/...` — pure Go: parameter extraction, buffer filtering, wait engine, capture
  validation.
- `go test ./test/` — real headless Chrome against `test/fixtures/testpage.html`, served by
  `httptest`. Each test pins one pain point: early wait return, console object expansion, filtered
  requests with body, compact snapshot whose selectors all resolve, hidden-element diagnosis,
  absolute-path capture, off-screen element capture, and the 2× zoom check. These skip cleanly when
  Chrome is not installed.

## Not implemented (later phases)

Persistent injected CSS/JS across navigations, full-page scroll-and-stitch, viewport presets,
fused capture-on-condition, typing-animation frames, burst capture, synthetic cursor overlay, video.
Iframe contents are not addressable: the `<iframe>` element itself appears in snapshots and
`get_element`, but selectors do not reach inside it.

**Known issues are listed in `docs/implementation-plan.md` under "Caveats and known issues."** JS
dialogs are dismissed automatically and reported in `get_console`, and tool calls are bounded (30s,
60s for `navigate`, `wait_for` by its own `timeout_ms`) — except `click`, `hover` and delta `scroll`,
which rod's mouse leaves unbounded.
