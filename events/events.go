package events

// Publisher is injected into services. Services never depend on HTTP/WS packages.
type Event struct {
	Type      string `json:"type"`
	Action    string `json:"action"`
	Data      any    `json:"data"`
	Timestamp string `json:"timestamp"`
}
type Publisher func(userIDs []string, event Event)
