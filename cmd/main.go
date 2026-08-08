package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"

	mcpHandler "github.com/gomcpgo/browser_automation/pkg/handler"
	"github.com/gomcpgo/browser_automation/test/fixtures"

	"github.com/gomcpgo/mcp/pkg/handler"
	"github.com/gomcpgo/mcp/pkg/protocol"
	"github.com/gomcpgo/mcp/pkg/server"
)

func main() {
	var (
		serveFixtures string
		navigate      string
		doSnapshot    bool
		interactive   bool
		element       string
		eval          string
		click         string
		typeText      string
		pressKey      string
		hover         string
		scroll        string
		wait          string
		waitAfter     string
		timeout       int
		console       bool
		requests      bool
		withBody      bool
		pattern       string
		urlPattern    string
		shot          string
		selector      string
		zoom          float64
		width         int
		height        int
		headed        bool
	)

	flag.StringVar(&serveFixtures, "serve-fixtures", "", "Serve the test fixture page on this address (e.g. :8899) and block")
	flag.StringVar(&navigate, "navigate", "", "Navigate to this URL")
	flag.BoolVar(&doSnapshot, "snapshot", false, "Print a page outline")
	flag.BoolVar(&interactive, "interactive-only", false, "With -snapshot: list interactive elements only")
	flag.StringVar(&element, "element", "", "Inspect this selector")
	flag.StringVar(&eval, "eval", "", "Evaluate this JS expression")
	flag.StringVar(&click, "click", "", "Click this selector")
	flag.StringVar(&typeText, "type", "", "Type into a selector, as 'selector=text'")
	flag.StringVar(&pressKey, "press", "", "Press a key, e.g. 'Meta+Enter'")
	flag.StringVar(&hover, "hover", "", "Hover this selector")
	flag.StringVar(&scroll, "scroll", "", "Scroll this selector into view")
	flag.StringVar(&wait, "wait", "", "Wait for a condition before interacting, as 'condition:value', e.g. 'text_visible:Ready'")
	flag.StringVar(&waitAfter, "wait-after", "", "Wait for a condition after interacting, same format as -wait")
	flag.IntVar(&timeout, "timeout", 10000, "With -wait/-wait-after: timeout in milliseconds")
	flag.BoolVar(&console, "console", false, "Print buffered console messages")
	flag.BoolVar(&requests, "requests", false, "Print buffered network requests")
	flag.BoolVar(&withBody, "body", false, "With -requests: include response bodies")
	flag.StringVar(&pattern, "pattern", "", "With -console: message filter regex")
	flag.StringVar(&urlPattern, "url-pattern", "", "With -requests: URL filter regex")
	flag.StringVar(&shot, "shot", "", "Write a screenshot to this absolute path")
	flag.StringVar(&selector, "selector", "", "With -shot: capture this element only")
	flag.Float64Var(&zoom, "zoom", 1, "Page zoom (sharpness lever)")
	flag.IntVar(&width, "width", 1280, "Viewport width")
	flag.IntVar(&height, "height", 800, "Viewport height")
	flag.BoolVar(&headed, "headed", false, "Run with a visible browser window")
	flag.Parse()

	if serveFixtures != "" {
		fmt.Printf("Serving fixtures on http://127.0.0.1%s\n", serveFixtures)
		log.Fatal(http.ListenAndServe(serveFixtures, fixtures.Handler()))
	}

	h := mcpHandler.New()
	ctx := context.Background()

	terminalMode := navigate != "" || doSnapshot || element != "" || eval != "" || click != "" ||
		typeText != "" || pressKey != "" || hover != "" || scroll != "" || wait != "" ||
		waitAfter != "" || console || requests || shot != ""

	if terminalMode {
		defer h.Shutdown()

		run(ctx, h, "start_session", map[string]interface{}{
			"width": float64(width), "height": float64(height),
			"headed": headed, "zoom": zoom,
		})

		if navigate != "" {
			run(ctx, h, "navigate", map[string]interface{}{"url": navigate})
		}
		if wait != "" {
			run(ctx, h, "wait_for", waitArgs(wait, timeout))
		}
		if click != "" {
			run(ctx, h, "click", map[string]interface{}{"selector": click})
		}
		if typeText != "" {
			sel, text, found := strings.Cut(typeText, "=")
			if !found {
				log.Fatal("-type expects 'selector=text'")
			}
			run(ctx, h, "type_text", map[string]interface{}{"selector": sel, "text": text})
		}
		if pressKey != "" {
			run(ctx, h, "press_key", map[string]interface{}{"key": pressKey})
		}
		if hover != "" {
			run(ctx, h, "hover", map[string]interface{}{"selector": hover})
		}
		if scroll != "" {
			run(ctx, h, "scroll", map[string]interface{}{"selector": scroll})
		}
		if waitAfter != "" {
			run(ctx, h, "wait_for", waitArgs(waitAfter, timeout))
		}
		if doSnapshot {
			run(ctx, h, "snapshot", map[string]interface{}{"interactive_only": interactive})
		}
		if element != "" {
			run(ctx, h, "get_element", map[string]interface{}{"selector": element})
		}
		if eval != "" {
			run(ctx, h, "evaluate", map[string]interface{}{"js": eval})
		}
		if console {
			run(ctx, h, "get_console", map[string]interface{}{"pattern": pattern})
		}
		if requests {
			run(ctx, h, "get_requests", map[string]interface{}{"url_pattern": urlPattern, "include_body": withBody})
		}
		if shot != "" {
			args := map[string]interface{}{"output_path": shot, "zoom": zoom}
			if selector != "" {
				args["selector"] = selector
			}
			run(ctx, h, "screenshot", args)
		}
		return
	}

	// MCP server mode
	registry := handler.NewHandlerRegistry()
	registry.RegisterToolHandler(h)

	srv := server.New(server.Options{
		Name:     "browser-automation",
		Title:    "Browser Automation",
		Version:  "1.0.0",
		Registry: registry,
	})

	if err := srv.Run(); err != nil {
		h.Shutdown()
		log.Fatalf("Server error: %v", err)
	}
}

// waitArgs turns 'condition:value' into the right parameter for the condition.
func waitArgs(spec string, timeout int) map[string]interface{} {
	condition, value, _ := strings.Cut(spec, ":")
	args := map[string]interface{}{
		"condition":  condition,
		"timeout_ms": float64(timeout),
	}
	switch condition {
	case "text_visible", "text_gone":
		args["text"] = value
	case "selector_visible", "selector_hidden":
		args["selector"] = value
	case "console_matches":
		args["pattern"] = value
	case "js_true":
		args["js"] = value
	case "network_idle":
		if value != "" {
			var ms float64
			fmt.Sscanf(value, "%f", &ms)
			args["idle_ms"] = ms
		}
	}
	return args
}

func run(ctx context.Context, h *mcpHandler.Handler, tool string, args map[string]interface{}) {
	fmt.Printf("\n=== %s ===\n", tool)

	resp, err := h.CallTool(ctx, &protocol.CallToolRequest{Name: tool, Arguments: args})
	if err != nil {
		h.Shutdown()
		log.Fatalf("%s failed: %v", tool, err)
	}

	for _, c := range resp.Content {
		var data interface{}
		if err := json.Unmarshal([]byte(c.Text), &data); err == nil {
			pretty, _ := json.MarshalIndent(data, "", "  ")
			fmt.Println(string(pretty))
		} else {
			fmt.Println(c.Text)
		}
	}

	if resp.IsError {
		h.Shutdown()
		os.Exit(1)
	}
}
