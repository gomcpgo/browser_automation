package snapshot

// Params scopes a snapshot.
type Params struct {
	Selector        string
	MaxDepth        int
	InteractiveOnly bool
}

// Node is one element in the outline.
type Node struct {
	Depth    int    `json:"depth"`
	Tag      string `json:"tag"`
	Role     string `json:"role,omitempty"`
	Text     string `json:"text,omitempty"`
	Selector string `json:"selector"`
}

// ElementParams asks for details of a single element.
type ElementParams struct {
	Selector      string
	IncludeStyles []string
}

// ElementInfo describes one element.
type ElementInfo struct {
	Selector   string            `json:"selector"`
	MatchCount int               `json:"match_count"`
	Tag        string            `json:"tag"`
	Text       string            `json:"text,omitempty"`
	Attributes map[string]string `json:"attributes,omitempty"`
	Bounds     Bounds            `json:"bounds"`
	Visible    bool              `json:"visible"`
	HiddenBy   string            `json:"hidden_by,omitempty"`
	Styles     map[string]string `json:"styles,omitempty"`
}

// Bounds is an element's box in CSS pixels.
type Bounds struct {
	X      float64 `json:"x"`
	Y      float64 `json:"y"`
	Width  float64 `json:"width"`
	Height float64 `json:"height"`
}
