package globals

import (
	"net"
	"sync"
	"tcp_sandbox/domain"
)

// Global maps for tenants and listeners.
var Tenants = make(map[string]*domain.Tenant)
var Listeners = make(map[string]net.Listener)

// Protects the tenants map & listeners map
var TenantsLock sync.Mutex
