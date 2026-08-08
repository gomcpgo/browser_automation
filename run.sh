#!/bin/bash
set -e

# Browser Automation MCP Server build/test script

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
cd "$SCRIPT_DIR"

command=$1
shift || true

case "$command" in
    build)
        echo "Building browser_automation..."
        mkdir -p bin
        go build -o bin/browser_automation ./cmd
        echo "Build complete: bin/browser_automation"
        ;;

    test)
        echo "Running all tests (integration tests skip without Chrome)..."
        go test ./...
        ;;

    test-unit)
        go test ./pkg/...
        ;;

    test-integration)
        go test ./test/ -v
        ;;

    install)
        go mod download
        go mod tidy
        ;;

    serve-fixture)
        PORT="${1:-8899}"
        go run ./cmd -serve-fixtures ":$PORT"
        ;;

    navigate)
        if [ -z "$1" ]; then
            echo "Usage: ./run.sh navigate <url>"
            exit 1
        fi
        go run ./cmd -navigate "$1" -snapshot
        ;;

    shot)
        if [ -z "$1" ] || [ -z "$2" ]; then
            echo "Usage: ./run.sh shot <url> <absolute_output_path> [selector] [zoom]"
            exit 1
        fi
        ARGS=(-navigate "$1" -shot "$2")
        [ -n "$3" ] && ARGS+=(-selector "$3")
        [ -n "$4" ] && ARGS+=(-zoom "$4")
        go run ./cmd "${ARGS[@]}"
        ;;

    wait)
        if [ -z "$1" ] || [ -z "$2" ]; then
            echo "Usage: ./run.sh wait <url> <condition:value> [timeout_ms]"
            echo "  e.g. ./run.sh wait http://localhost:3000 text_visible:Ready 45000"
            exit 1
        fi
        go run ./cmd -navigate "$1" -wait "$2" -timeout "${3:-10000}"
        ;;

    console)
        if [ -z "$1" ]; then
            echo "Usage: ./run.sh console <url> [pattern]"
            exit 1
        fi
        go run ./cmd -navigate "$1" -console -pattern "${2:-}"
        ;;

    requests)
        if [ -z "$1" ]; then
            echo "Usage: ./run.sh requests <url> [url_pattern]"
            exit 1
        fi
        go run ./cmd -navigate "$1" -requests -url-pattern "${2:-}" -body
        ;;

    demo)
        echo "=== Fixture smoke test ==="
        mkdir -p bin
        go build -o bin/browser_automation ./cmd

        bin/browser_automation -serve-fixtures :8899 >/dev/null 2>&1 &
        SERVER_PID=$!
        trap "kill $SERVER_PID 2>/dev/null" EXIT
        sleep 1

        bin/browser_automation \
            -navigate http://127.0.0.1:8899/ \
            -wait "text_visible:Ready" -timeout 15000 \
            -snapshot \
            -click "#fetch-btn" \
            -wait-after "text_visible:got 2 items" \
            -console \
            -requests -url-pattern "/api/" -body \
            -element "#dropdown" \
            -shot /tmp/browser-automation-demo.png -selector "#detail" -zoom 2

        echo ""
        echo "Wrote /tmp/browser-automation-demo.png"
        ;;

    clean)
        rm -rf bin
        echo "Clean complete"
        ;;

    *)
        echo "Browser Automation MCP Server"
        echo ""
        echo "Usage: $0 <command> [args]"
        echo ""
        echo "Commands:"
        echo "  build                                   Build the MCP server"
        echo "  test                                    Run all tests"
        echo "  test-unit                               Run unit tests only"
        echo "  test-integration                        Run browser integration tests"
        echo "  install                                 Download and tidy dependencies"
        echo "  serve-fixture [port]                    Serve the test page (default 8899)"
        echo "  navigate <url>                          Navigate and print a page outline"
        echo "  shot <url> <abs_path> [selector] [zoom] Screenshot to an absolute path"
        echo "  wait <url> <condition:value> [timeout]  Wait for a condition"
        echo "  console <url> [pattern]                 Print buffered console messages"
        echo "  requests <url> [url_pattern]            Print buffered network requests"
        echo "  demo                                    End-to-end run against the fixture page"
        echo "  clean                                   Remove build artifacts"
        ;;
esac
