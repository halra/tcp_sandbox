package domain

import (
	"net"
	"sync"
)

// Tenant holds data relevant to a particular tenant.
type Tenant struct {
	Name      string
	Port      string
	Comment   string
	StartByte byte
	EndByte   byte

	// Counters
	BytesReceived uint64
	BytesSent     uint64
	Errors        uint64

	// SimpleAuth
	SimpleAuthToken string

	// OAuth / Credentials
	OAuthCredentials OAuthCredentials

	// Keep Alive config
	KeepAliveIntervalSec int    // e.g. 30 -> send keep-alive every 30s
	KeepAliveFile        string // path to the tenant's keep-alive XML file

	// Connections are runtime-only; we omit them from JSON
	Connections     []net.Conn `json:"-"`
	ConnectionsLock sync.Mutex `json:"-"`

	//Message format
	MessageFormat string

	//TargetURL url
	Endpoint string

	Remove bool `json:"remove,omitempty"`
}
