package models

import (
	"strings"
	"testing"
)

func TestNodeValidate(t *testing.T) {
	for _, tt := range []struct {
		name   string
		change func(*Node)
		valid  bool
	}{
		{"valid", func(n *Node) {}, true},
		{"hostname", func(n *Node) { n.Hostname = "web.example.com" }, true},
		{"IPv6", func(n *Node) { n.Hostname = "2001:db8::1" }, true},
		{"qssh", func(n *Node) { n.ConnectionType = "qssh" }, true},
		{"password", func(n *Node) { n.Auth.Method = AuthMethodPassword }, true},
		{"no tags", func(n *Node) { n.Tags = nil }, true},
		{"empty name", func(n *Node) { n.Name = "" }, false},
		{"long name", func(n *Node) { n.Name = strings.Repeat("a", 51) }, false},
		{"invalid name", func(n *Node) { n.Name = "web-prod" }, false},
		{"empty hostname", func(n *Node) { n.Hostname = "" }, false},
		{"invalid hostname", func(n *Node) { n.Hostname = "foo bar;baz" }, false},
		{"missing port", func(n *Node) { n.Port = 0 }, false},
		{"negative port", func(n *Node) { n.Port = -1 }, false},
		{"large port", func(n *Node) { n.Port = 65536 }, false},
		{"short username", func(n *Node) { n.Username = "a" }, false},
		{"long username", func(n *Node) { n.Username = strings.Repeat("a", 51) }, false},
		{"invalid connection", func(n *Node) { n.ConnectionType = "telnet" }, false},
		{"invalid tag", func(n *Node) { n.Tags = []string{"web-prod"} }, false},
		{"empty tag", func(n *Node) { n.Tags = []string{""} }, false},
		{"missing credential", func(n *Node) { n.Auth.CredentialID = "" }, false},
		{"invalid credential", func(n *Node) { n.Auth.CredentialID = "invalid" }, false},
		{"invalid auth", func(n *Node) { n.Auth.Method = "invalid" }, false},
	} {
		t.Run(tt.name, func(t *testing.T) {
			n := Node{Name: "web_01", Hostname: "192.0.2.1", Port: 22, Username: "admin", ConnectionType: "ssh", Tags: []string{"web", "prod_1"}, Auth: NodeAuth{CredentialID: "dd546d22-2efe-4ad3-9263-5b88640c2df4", Method: AuthMethodPrivateKey}}
			tt.change(&n)
			if err := n.Validate(); (err == nil) != tt.valid {
				t.Fatalf("Validate() = %v, want valid=%v", err, tt.valid)
			}
		})
	}
}
