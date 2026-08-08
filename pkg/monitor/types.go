package monitor

import "time"

// ConsoleEntry is one captured console message or uncaught exception.
type ConsoleEntry struct {
	Level string        `json:"level"`
	Time  time.Time     `json:"-"`
	Args  []interface{} `json:"args"`
}

// Request is one captured network request/response pair.
type Request struct {
	ID       string    `json:"-"`
	URL      string    `json:"url"`
	Method   string    `json:"method"`
	Status   int       `json:"status,omitempty"`
	MimeType string    `json:"mime_type,omitempty"`
	Size     int       `json:"size,omitempty"`
	Duration int       `json:"duration_ms,omitempty"`
	Error    string    `json:"error,omitempty"`
	Body     string    `json:"body,omitempty"`
	Start    time.Time `json:"-"`
	Done     bool      `json:"-"`
}

// ConsoleFilter narrows a console query.
type ConsoleFilter struct {
	Pattern     string
	Levels      []string
	SinceMs     int
	MaxResults  int
	ExpandDepth int
}

// RequestFilter narrows a network query.
type RequestFilter struct {
	URLPattern  string
	Method      string
	Status      []int
	IncludeBody bool
	SinceMs     int
	MaxResults  int
}
