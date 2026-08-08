# Playwright MCP Server - Testing Experience and Improvement Suggestions

**Date**: 2025-11-12
**Context**: Experience using Playwright MCP server for testing SimpleHTMLDocgen integration
**Overall Rating**: 7/10 - Good for basic testing, significant gaps for complex scenarios

## What Worked Well

### 1. Navigation and Basic Interactions
- `browser_navigate` worked flawlessly
- `browser_click` reliably found and clicked elements
- `browser_snapshot` provided excellent accessibility tree view
- The ref-based element selection was accurate

### 2. Visibility into Application State
- Console messages were invaluable for debugging
- Network requests would have been useful (didn't use it much)
- The accessibility tree snapshot is much better than raw HTML

### 3. Screenshot Capability
- `browser_take_screenshot` was perfect for visual verification
- Saved screenshots to a local directory for review
- Helped confirm the UI was actually rendering correctly

## Issues and Pain Points

### 1. Cannot See What's Inside iframes
- **Problem**: The document panel uses an iframe to display HTML content
- **Impact**: Could see the iframe element exists, but couldn't verify if content was actually rendering inside it
- **What I Tried**: Taking screenshots shows the iframe area but not the content within
- **Use Case**: Need to verify that the HTML invoice actually displays, not just that an iframe exists
- **Current Workaround**: Have to trust that if metadata loads, content probably loads too

### 2. Dropdown/Menu Visibility Issues
- **Problem**: Clicked the Export button but couldn't tell if a dropdown menu appeared
- **What Happened**:
  - Click registered (button became active)
  - No visible menu in the snapshot
  - No errors in console
- **Possible Causes**:
  - Menu rendered but outside viewport
  - Z-index issues hiding menu
  - Menu state didn't toggle
- **What Would Help**:
  - Ability to check if specific elements appeared after an action
  - CSS/style inspection to see if menu has `display: none` or is positioned off-screen
  - Visual diff between snapshots

### 3. Limited State Inspection
- **Problem**: Cannot inspect Vue component state or data
- **Example**: Couldn't check if `showExportMenu` variable was true or false
- **Impact**: Had to guess why dropdown didn't appear instead of debugging directly
- **What Would Help**:
  - Ability to evaluate JavaScript and get component data
  - Access to Vue DevTools-like inspection

### 4. No Wait for Dynamic Content
- **Problem**: Had to manually add `sleep` commands to wait for page load
- **Example**: After navigation, had to `sleep 3` before taking snapshot
- **Impact**: Tests are slower and timing-dependent
- **What Would Help**:
  - Built-in wait for specific elements
  - Wait for network idle
  - Wait for Vue to finish rendering

### 5. Console Message Filtering
- **Problem**: Console had many unrelated logs mixed with important errors
- **What Worked**: Could filter by `onlyErrors: true`
- **What's Missing**:
  - Can't filter by log level (LOG, INFO, WARN, ERROR)
  - Can't search console messages by pattern
  - Hard to find specific log messages in long output

### 6. Limited Form Interaction
- **Problem**: Didn't test this, but noticed `browser_fill_form` seems limited
- **What's Missing**:
  - No way to type into contentEditable elements
  - No way to interact with custom Vue components (like custom dropdowns)
  - No drag-and-drop support (would be useful for document editing)

### 7. No Visual Regression Testing
- **Problem**: Screenshots are saved but not compared
- **What Would Help**:
  - Ability to compare current screenshot with baseline
  - Visual diff highlighting
  - Threshold for acceptable differences

### 8. Cannot Verify File Downloads
- **Problem**: Export functionality triggers file downloads
- **Impact**: Can't verify that export actually produces correct files
- **What Would Help**:
  - Intercept download events
  - Access downloaded file contents
  - Verify file metadata (name, type, size)

## Features That Would Have Helped

### Priority 1: Critical Gaps

#### 1. iframe Content Access
- **Feature**: `browser_get_iframe_content` or similar
- **Parameters**: iframe ref/selector
- **Returns**: Snapshot of iframe's internal document
- **Use Case**: Verify HTML document actually renders in SimpleHTMLPanel

#### 2. Wait for Element/Condition
- **Feature**: `browser_wait_for_element`
- **Parameters**: selector, timeout, condition (visible/hidden/exists)
- **Use Case**: Wait for dropdown menu to appear after click
- **Example**: `browser_wait_for_element("export-menu", {timeout: 5000, condition: "visible"})`

#### 3. Element Style Inspection
- **Feature**: `browser_get_computed_style`
- **Parameters**: element ref
- **Returns**: CSS properties (display, visibility, opacity, position, z-index)
- **Use Case**: Debug why dropdown menu isn't visible

### Priority 2: Enhanced Debugging

#### 4. JavaScript Evaluation with Return Values
- **Feature**: Enhanced `browser_evaluate` that returns structured data
- **Current**: Returns basic evaluation result
- **Needed**: Access to Vue component data, store state
- **Use Case**: Check if `showExportMenu` is true
- **Example**: `browser_evaluate("$refs.exportMenu.showMenu")`

#### 5. Network Request Details
- **Feature**: Enhanced `browser_network_requests` with filtering
- **Current**: Returns all requests
- **Needed**:
  - Filter by URL pattern
  - Include request/response bodies
  - Show request timing
- **Use Case**: Verify InvokeMCPTool request body and response

#### 6. Console Message Search
- **Feature**: `browser_console_search`
- **Parameters**: pattern (regex), level (LOG/INFO/WARN/ERROR)
- **Returns**: Matching console messages only
- **Use Case**: Find specific log like "parsed JSON result" in noisy console

#### 7. Snapshot Diff
- **Feature**: `browser_snapshot_diff`
- **Parameters**: two snapshot IDs or before/after action
- **Returns**: What changed in the accessibility tree
- **Use Case**: See exactly what appeared/disappeared after clicking Export button

### Priority 3: Advanced Testing

#### 8. Download Interception
- **Feature**: `browser_intercept_download`
- **Parameters**: wait for download, timeout
- **Returns**: File name, path to downloaded file
- **Use Case**: Verify export produces file and check its contents

#### 9. Visual Comparison
- **Feature**: `browser_compare_screenshot`
- **Parameters**: current screenshot, baseline path, threshold
- **Returns**: Diff image and similarity score
- **Use Case**: Verify document panel layout matches expected design

#### 10. Hover State Inspection
- **Feature**: Enhanced `browser_hover` with return value
- **Returns**: What changed after hover (tooltips, menu items, etc.)
- **Use Case**: Verify hover effects work correctly

#### 11. Form State Capture
- **Feature**: `browser_get_form_state`
- **Parameters**: form ref
- **Returns**: All form field values as JSON
- **Use Case**: Verify contentEditable changes before save

#### 12. Wait for Network Idle
- **Feature**: `browser_wait_for_network_idle`
- **Parameters**: timeout, max active requests
- **Returns**: When network becomes idle
- **Use Case**: Wait for page to fully load instead of arbitrary sleep

### Priority 4: Developer Experience

#### 13. Session Recording
- **Feature**: `browser_start_recording` / `browser_stop_recording`
- **Returns**: Video file of entire test session
- **Use Case**: Review what actually happened during failed test

#### 14. Breakpoint/Pause
- **Feature**: `browser_pause`
- **Description**: Pause test execution to manually inspect page
- **Use Case**: Interactive debugging when automated test hits unexpected state

#### 15. Snapshot History
- **Feature**: Automatic snapshot history with timeline
- **Description**: Keep history of all snapshots with timestamps
- **Use Case**: Review how page state changed over time

## Specific Pain Points in This Session

### 1. The Export Button Mystery

**What I Knew:**
- Button was clicked (ref worked)
- Button became active (snapshot showed this)
- No console errors

**What I Couldn't Determine:**
- Did menu actually render? (Not in snapshot)
- Is menu hidden by CSS? (No style inspection)
- Is Vue state updated? (No component inspection)
- Is menu outside viewport? (No bounds checking)

**What Would Have Helped:**
- `browser_get_computed_style(menu_ref)` → Check if display: none
- `browser_evaluate("component.showExportMenu")` → Check Vue state
- `browser_wait_for_element("export-menu", {visible: true, timeout: 2000})` → Confirm menu appears or times out
- `browser_snapshot_diff(before_click, after_click)` → See what changed

### 2. The iframe Content Gap

**What I Knew:**
- iframe element exists in DOM
- iframe has a ref
- Document metadata loaded successfully

**What I Couldn't Verify:**
- Does iframe have correct src attribute?
- Does HTML content actually render inside?
- Are styles applied correctly?
- Can you scroll the content?

**What Would Have Helped:**
- `browser_get_iframe_content(iframe_ref)` → Snapshot of iframe internals
- `browser_screenshot_element(iframe_ref)` → Screenshot just the iframe content
- `browser_evaluate_in_iframe(iframe_ref, "document.body.innerHTML")` → Get iframe HTML

### 3. The Timing Dance

**What I Did:**
- Navigate to page
- Sleep 3 seconds
- Take snapshot
- Click element
- Take another snapshot

**What Was Awkward:**
- Arbitrary sleep times
- Don't know if 3 seconds is enough or too much
- Can't tell when page is actually ready

**What Would Have Helped:**
- `browser_wait_for_network_idle()` → Wait for all requests to finish
- `browser_wait_for_element(expected_element)` → Wait for specific content
- Automatic waiting built into navigation

## Recommendations for Playwright MCP Server Improvements

### Phase 1: Critical Features (Highest ROI)
1. **iframe content access** - Unblocks testing embedded content
2. **Wait for element** - Makes tests more reliable
3. **Style inspection** - Essential for debugging visibility issues

### Phase 2: Enhanced Debugging
4. **Snapshot diff** - Shows exactly what changed
5. **Network request filtering** - Better API testing
6. **Console message search** - Faster debugging

### Phase 3: Advanced Capabilities
7. **Download interception** - Test file exports
8. **Visual comparison** - Catch UI regressions
9. **Form state capture** - Verify form handling

### Phase 4: Developer Experience
10. **Session recording** - Review test runs
11. **Wait for network idle** - Eliminate arbitrary waits
12. **Enhanced evaluate** - Access component state

## Overall Assessment

The Playwright MCP server is **very good for basic UI testing** but has significant gaps for:
- Testing complex Vue components with dynamic state
- Verifying embedded content (iframes, shadow DOM)
- Debugging why elements don't appear as expected
- Testing file operations (downloads, uploads)
- Visual regression testing

### Rating: 7/10

**Strengths:**
- ✅ Could verify basic functionality (navigation, clicks, element presence)
- ✅ Screenshots confirmed visual rendering
- ✅ Accessibility tree snapshots are excellent
- ✅ Console messages provide good debugging info

**Weaknesses:**
- ❌ Could not debug dropdown menu issue effectively
- ❌ Could not verify iframe content rendering
- ❌ Required manual wait times instead of smart waiting
- ❌ Limited state inspection for debugging complex issues

## Use Cases Where Playwright MCP Excels

1. **Basic navigation testing** - Pages, links, buttons
2. **Form submission** - Simple forms with standard inputs
3. **Visual verification** - Screenshot comparison (manual)
4. **Console error detection** - Catch JavaScript errors
5. **Element presence** - Verify elements exist in DOM

## Use Cases Where Playwright MCP Falls Short

1. **Complex component testing** - Vue/React component state
2. **Dynamic content** - Dropdowns, modals, tooltips
3. **Embedded content** - iframes, shadow DOM
4. **File operations** - Downloads, uploads
5. **Timing-dependent flows** - No smart waiting
6. **Visual regression** - No automated comparison
7. **Deep debugging** - Limited state inspection

## Conclusion

The Playwright MCP server is a **solid foundation** for UI testing but needs enhancements for production-grade testing of complex modern web applications. The most impactful improvements would be:

1. Smart waiting (network idle, element conditions)
2. iframe content access
3. Style/state inspection for debugging

These three features alone would have resolved the majority of pain points encountered during this testing session.
