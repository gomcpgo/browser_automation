package monitor

import (
	"regexp"
	"strings"
	"sync"
	"time"
)

const (
	maxConsoleEntries = 1000
	maxRequests       = 500
)

// Monitor buffers console and network events for the lifetime of a session.
type Monitor struct {
	mu       sync.Mutex
	console  []ConsoleEntry
	requests []*Request
	byID     map[string]*Request

	inFlight     int
	lastActivity time.Time
}

// New creates an empty monitor.
func New() *Monitor {
	return &Monitor{
		byID:         make(map[string]*Request),
		lastActivity: time.Now(),
	}
}

// AddConsole appends a console entry, evicting the oldest when full.
func (m *Monitor) AddConsole(level string, args []interface{}) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.console = append(m.console, ConsoleEntry{Level: level, Time: time.Now(), Args: args})
	if len(m.console) > maxConsoleEntries {
		m.console = m.console[len(m.console)-maxConsoleEntries:]
	}
}

// RequestStarted records a new outgoing request.
func (m *Monitor) RequestStarted(id, url, method string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	r := &Request{ID: id, URL: url, Method: method, Start: time.Now()}
	m.requests = append(m.requests, r)
	m.byID[id] = r
	m.inFlight++
	m.lastActivity = time.Now()

	if len(m.requests) > maxRequests {
		drop := m.requests[:len(m.requests)-maxRequests]
		for _, d := range drop {
			delete(m.byID, d.ID)
		}
		m.requests = m.requests[len(m.requests)-maxRequests:]
	}
}

// ResponseReceived fills in the response metadata for a request.
func (m *Monitor) ResponseReceived(id string, status int, mimeType string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if r, ok := m.byID[id]; ok {
		r.Status = status
		r.MimeType = mimeType
	}
	m.lastActivity = time.Now()
}

// RequestFinished marks a request complete.
func (m *Monitor) RequestFinished(id string, size int) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if r, ok := m.byID[id]; ok && !r.Done {
		r.Done = true
		r.Size = size
		r.Duration = int(time.Since(r.Start).Milliseconds())
		m.inFlight--
	}
	m.lastActivity = time.Now()
}

// RequestFailed marks a request as failed.
func (m *Monitor) RequestFailed(id, errText string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if r, ok := m.byID[id]; ok && !r.Done {
		r.Done = true
		r.Error = errText
		r.Duration = int(time.Since(r.Start).Milliseconds())
		m.inFlight--
	}
	m.lastActivity = time.Now()
}

// SetBody stores a captured response body.
func (m *Monitor) SetBody(id, body string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if r, ok := m.byID[id]; ok {
		r.Body = body
	}
}

// Console returns the entries matching the filter, oldest first.
func (m *Monitor) Console(f ConsoleFilter) ([]ConsoleEntry, error) {
	var re *regexp.Regexp
	if f.Pattern != "" {
		var err error
		re, err = regexp.Compile(f.Pattern)
		if err != nil {
			return nil, err
		}
	}

	levels := map[string]bool{}
	for _, l := range f.Levels {
		levels[strings.ToLower(l)] = true
	}

	cutoff := time.Time{}
	if f.SinceMs > 0 {
		cutoff = time.Now().Add(-time.Duration(f.SinceMs) * time.Millisecond)
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	out := []ConsoleEntry{}
	for _, e := range m.console {
		if !cutoff.IsZero() && e.Time.Before(cutoff) {
			continue
		}
		if len(levels) > 0 && !levels[e.Level] {
			continue
		}
		if re != nil && !re.MatchString(RenderArgs(e.Args, 0)) {
			continue
		}
		entry := e
		if f.ExpandDepth > 0 {
			entry.Args = trimAll(e.Args, f.ExpandDepth)
		}
		out = append(out, entry)
	}

	if f.MaxResults > 0 && len(out) > f.MaxResults {
		out = out[len(out)-f.MaxResults:]
	}
	return out, nil
}

// Requests returns the requests matching the filter, oldest first.
func (m *Monitor) Requests(f RequestFilter) ([]Request, error) {
	var re *regexp.Regexp
	if f.URLPattern != "" {
		var err error
		re, err = regexp.Compile(f.URLPattern)
		if err != nil {
			return nil, err
		}
	}

	statuses := map[int]bool{}
	for _, s := range f.Status {
		statuses[s] = true
	}

	cutoff := time.Time{}
	if f.SinceMs > 0 {
		cutoff = time.Now().Add(-time.Duration(f.SinceMs) * time.Millisecond)
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	out := []Request{}
	for _, r := range m.requests {
		if !cutoff.IsZero() && r.Start.Before(cutoff) {
			continue
		}
		if re != nil && !re.MatchString(r.URL) {
			continue
		}
		if f.Method != "" && !strings.EqualFold(f.Method, r.Method) {
			continue
		}
		if len(statuses) > 0 && !statuses[r.Status] {
			continue
		}
		copyReq := *r
		if !f.IncludeBody {
			copyReq.Body = ""
		}
		out = append(out, copyReq)
	}

	if f.MaxResults > 0 && len(out) > f.MaxResults {
		out = out[len(out)-f.MaxResults:]
	}
	return out, nil
}

// ConsoleMatches reports whether any buffered console entry matches the regex.
// sinceMs of 0 searches the whole buffer, so history counts.
func (m *Monitor) ConsoleMatches(pattern string, sinceMs int) (bool, error) {
	entries, err := m.Console(ConsoleFilter{Pattern: pattern, SinceMs: sinceMs})
	if err != nil {
		return false, err
	}
	return len(entries) > 0, nil
}

// NetworkIdle reports whether nothing is in flight and no network event has
// happened for idle milliseconds.
func (m *Monitor) NetworkIdle(idleMs int) bool {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.inFlight > 0 {
		return false
	}
	return time.Since(m.lastActivity) >= time.Duration(idleMs)*time.Millisecond
}
