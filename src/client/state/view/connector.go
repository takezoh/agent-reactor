package view

// ConnectorSection is a titled group of items in the main TUI display.
type ConnectorSection struct {
	Title string          `json:"title"`
	Items []ConnectorItem `json:"items,omitempty"`
}

// ConnectorItem is one entry within a ConnectorSection.
type ConnectorItem struct {
	Symbol string `json:"symbol"`
	Title  string `json:"title"`
	Meta   string `json:"meta"`
}
