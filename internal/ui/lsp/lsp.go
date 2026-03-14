// Package lsp provides LSP-related types for the UI.
package lsp

// Status represents LSP status.
type Status string

const (
	StatusDisconnected Status = "disconnected"
	StatusConnecting   Status = "connecting"
	StatusConnected    Status = "connected"
	StatusError        Status = "error"
)

// Server represents an LSP server.
type Server struct {
	ID     string `json:"id"`
	Name   string `json:"name"`
	Status Status `json:"status"`
}

// State represents LSP state.
type State struct {
	Servers map[string]*Server `json:"servers"`
}

// NewState creates a new LSP state.
func NewState() *State {
	return &State{
		Servers: make(map[string]*Server),
	}
}

// AddServer adds a server to the state.
func (s *State) AddServer(server *Server) {
	s.Servers[server.ID] = server
}

// RemoveServer removes a server from the state.
func (s *State) RemoveServer(id string) {
	delete(s.Servers, id)
}

// GetServer returns a server by ID.
func (s *State) GetServer(id string) (*Server, bool) {
	server, ok := s.Servers[id]
	return server, ok
}

// AllServers returns all servers.
func (s *State) AllServers() []*Server {
	servers := make([]*Server, 0, len(s.Servers))
	for _, server := range s.Servers {
		servers = append(servers, server)
	}
	return servers
}

// IsConnected checks if any server is connected.
func (s *State) IsConnected() bool {
	for _, server := range s.Servers {
		if server.Status == StatusConnected {
			return true
		}
	}
	return false
}

// Client represents an LSP client.
type Client struct {
	Name  string
	State *State
}

// DiagnosticCounts represents diagnostic counts.
type DiagnosticCounts struct {
	Error   int
	Warning int
	Info    int
}

// GetDiagnosticCounts returns diagnostic counts.
func (c *Client) GetDiagnosticCounts() DiagnosticCounts {
	return DiagnosticCounts{}
}

// ConnectionState represents LSP connection state.
type ConnectionState int

const (
	ConnectionStateUnstarted ConnectionState = iota
	ConnectionStateStopped
	ConnectionStateStarting
	ConnectionStateRunning
	ConnectionStateError
	ConnectionStateDisabled
)
