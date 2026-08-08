# Capture tool — feature requests

Running notes for the planned Go/MCP screen-capture tool that will replace Playwright MCP for demo
capture. Playwright was built for UI testing; everything below is friction that comes from
re-purposing it. Add to this as we hit more.

Status: collecting. Nothing here is built.

## Blocking friction (hit on every run)

1. **Write screenshots anywhere.** Playwright MCP sandboxes output to the project root, so every
   shot has to be captured into `.playwright-mcp/` and copied out to the videos folder. It also
   silently wrote a file to the repo root when given a relative filename. Capture should take an
   absolute destination path and use it.

2. **Region capture with an explicit crop rect.** The demo need is "one zoomed-in part at a time."
   Today that means screenshotting the full viewport and redoing the zoom downstream in CSS
   (`transform: scale() translate()`) with hand-computed coordinates. Want `capture(x, y, w, h)` and
   `capture(selector)` returning just that region, already cropped.

3. **Capture scale independent of viewport size.** Viewport size decides layout; pixel density
   decides sharpness. Playwright ties them together — 1920×1080 viewport gives a 1920×1080 file, so
   a zoomed crop is upscaled and soft. Want viewport 1200×800 rendered at 3× for a crisp 3600×2400
   source to crop from.

4. **Capture on condition, not on a timer.** Waiting for a response means blind `sleep(8)`. Want
   "screenshot when this text appears", "when streaming starts", "when streaming ends", with a
   timeout — so a slow model doesn't produce an empty frame and a fast one doesn't waste seconds.

## Demo-specific (things a test tool would never need)

5. **Synthetic cursor overlay.** Playwright moves a real pointer but renders no cursor, so clicks
   are invisible in a screenshot. Demo video needs a visible cursor drawn at the pointer position,
   ideally with a click-ripple frame.

6. **Typing animation frames.** Currently faked by filling half the string, capturing, then filling
   the whole string. Want "type this text over N ms and capture a frame every X%" — real
   intermediate frames, evenly spaced, in one call.

7. **Burst capture during streaming.** Mid-stream frames (half-written response + spinner) are one
   of the best beats available, but catching one is luck. Want "capture every 400ms while the
   response streams" and pick the best frame afterwards.

8. **Scroll-and-stitch for long content.** A response taller than the viewport can't be captured in
   one frame today. Want a full-content capture that stitches, and a "scroll to element, then
   capture" primitive.

9. **Persistent injected CSS/JS.** The Nuxt devtools button has to be hidden with an injected style
   after every navigation. Want CSS/JS registered once and re-applied on every load.

## Nice to have

10. **Deterministic output filenames** returned in the response, so the compose step can reference
    them without a directory listing.

11. **Named viewport presets** (demo-wide, demo-tight, vertical-short) so runs are reproducible
    across sessions.

12. **Approval-gate handling.** Savant pauses on an MCP Server Access Request and waits for a human
    click. Capture needs to detect that state, capture it (it is a good demo beat — the human in the
    driver's seat), and then either approve it or hand back. Right now a run can silently stall.

13. **Stall detection.** A hung backend call looks identical to a slow one. Want "wait for X, and if
    the UI has not changed in N seconds, return a timeout" so a run reports a stall instead of
    burning wall-clock.

## Findings worth keeping

- **Page zoom is the sharpness lever, not DPR.** The webview reports `devicePixelRatio: 1` and the
  MCP resize tool cannot change it, so `scale: device` gives a flat 1× file. Setting
  `document.documentElement.style.zoom = '2'` before capture makes the browser genuinely re-render
  at 2× — a 736×221 element captured 1472×442, sharp, not upscaled. Layout reflows at the new zoom,
  so set zoom first, then locate elements.

- **Element capture works and beats cropping.** `screenshot(selector)` returns exactly the element's
  box, which removes the hand-computed CSS zoom math for single-element frames. Caveat: Playwright
  runs selectors in strict mode, so a class shared by two elements (`.savant-card`) errors out.
  Needs a unique selector.

- **Savant's message DOM has no test ids.** `data-testid` exists on shell elements (`chat-input`,
  `project-tab-chats`, sidebar items) but not on message bubbles, tool-call blocks or artifacts —
  the parts a demo actually frames. Capture currently leans on utility classes
  (`div[data-pending-approval]`, `.savant-card`) which will break on any refactor. Worth asking for
  stable ids on: user message, assistant message, tool-call block, artifact panel.

- **Injected CSS does not survive a reload.** The devtools-hiding style has to be re-injected after
  every navigation (see #9).

- **Dark theme frames better than light.** In light theme the user message has no bubble and floats
  on a large white field; in dark theme it gets a filled bubble and the content reads as a distinct
  block. Not decided yet — captured both in `test-run-2026-08-08/tests/`.
