package browser

import (
	"fmt"
	"os"
)

// ChromePathEnv names the environment variable that overrides which browser
// binary is launched.
const ChromePathEnv = "BROWSER_AUTOMATION_CHROME_PATH"

// ChromePathFromEnv returns the configured browser binary. An empty string means
// auto-detect, which also silently downloads Chromium when nothing is installed.
// Call this at startup so a bad path fails immediately rather than at the first
// start_session.
func ChromePathFromEnv() (string, error) {
	path := os.Getenv(ChromePathEnv)
	if path == "" {
		return "", nil
	}

	info, err := os.Stat(path)
	if err != nil {
		return "", fmt.Errorf("%s: cannot use %q: %w", ChromePathEnv, path, err)
	}
	if info.IsDir() {
		return "", fmt.Errorf("%s: %q is a directory, not a browser binary "+
			"(on macOS point at the executable inside the bundle, e.g. "+
			"'/Applications/Google Chrome.app/Contents/MacOS/Google Chrome')", ChromePathEnv, path)
	}

	return path, nil
}
