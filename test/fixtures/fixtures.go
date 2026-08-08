// Package fixtures serves the local test page used by the integration tests and
// by the terminal serve-fixtures mode.
package fixtures

import (
	"embed"
	"net/http"
)

//go:embed testpage.html frame.html
var files embed.FS

// Handler serves the fixture page plus a small JSON API endpoint.
func Handler() http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("/api/test", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"items":[{"id":1,"name":"alpha"},{"id":2,"name":"beta"}],"ok":true}`))
	})

	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		var name string
		switch r.URL.Path {
		case "/", "/testpage.html":
			name = "testpage.html"
		case "/frame.html":
			name = "frame.html"
		default:
			http.NotFound(w, r)
			return
		}
		data, err := files.ReadFile(name)
		if err != nil {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Write(data)
	})

	return mux
}
