package service

import (
	"log"
	"net"
	"sync/atomic"
	"tcp_sandbox/domain"
	"tcp_sandbox/globals"
)

// -----------------------------------------------------------
// Utility & Logging
// -----------------------------------------------------------

func addConnection(t *domain.Tenant, conn net.Conn) {
	t.ConnectionsLock.Lock()
	defer t.ConnectionsLock.Unlock()
	t.Connections = append(t.Connections, conn)
}

func removeConnection(t *domain.Tenant, conn net.Conn) {
	t.ConnectionsLock.Lock()
	defer t.ConnectionsLock.Unlock()

	var updated []net.Conn
	for _, c := range t.Connections {
		if c != conn {
			updated = append(updated, c)
		}
	}
	t.Connections = updated
}

func logError(t *domain.Tenant, err error) {
	atomic.AddUint64(&t.Errors, 1)
	log.Printf("[ERROR][Tenant %q] %v", t.Name, err)
}

// printAllTenantsStatus logs a status overview of all tenants each time it’s called.
func printAllTenantsStatus() {
	globals.TenantsLock.Lock()
	defer globals.TenantsLock.Unlock()

	log.Println("======= Tenant Status =======")
	for _, t := range globals.Tenants {
		printTenantStatus(t)
	}
	log.Println("=============================")
}

func printTenantStatus(t *domain.Tenant) {
	received := atomic.LoadUint64(&t.BytesReceived)
	sent := atomic.LoadUint64(&t.BytesSent)
	errs := atomic.LoadUint64(&t.Errors)

	t.ConnectionsLock.Lock()
	connCount := len(t.Connections)
	t.ConnectionsLock.Unlock()

	log.Printf("[Status][Tenant %q on port %s]\n"+
		"  - Connections: %d\n"+
		"  - BytesReceived: %d | BytesSent: %d | Errors: %d\n"+
		"  - KeepAlive: Interval=%ds File=%s\n"+
		"  - Comment: %s\n",
		t.Name, t.Port,
		connCount,
		received, sent, errs,
		t.KeepAliveIntervalSec, t.KeepAliveFile,
		t.Comment,
	)
}
