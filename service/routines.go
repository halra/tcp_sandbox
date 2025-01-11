package service

import (
	"encoding/xml"
	"fmt"
	"log"
	"os"
	"sync/atomic"
	"tcp_sandbox/domain"
	"time"

	"tcp_sandbox/globals"
)

// -----------------------------------------------------------
// Tenant Keep-Alive Routine
// -----------------------------------------------------------

// startKeepAliveRoutine checks if the tenant has KeepAliveIntervalSec > 0
// and KeepAliveFile != "". If yes, it launches a goroutine that sends
func StartKeepAliveRoutine(t *domain.Tenant) {
	if t.KeepAliveIntervalSec <= 0 || t.KeepAliveFile == "" {
		return
	}

	interval := time.Duration(t.KeepAliveIntervalSec) * time.Second
	go func(tenant *domain.Tenant) {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()

		for {
			<-ticker.C
			sendTenantKeepAlive(tenant)
		}
	}(t)
}

// sendTenantKeepAlive reads/updates the keep-alive XML file for the tenant,
// updates <sendTime> to now, writes it back, then sends the XML to all connections.
// TODO set the keepalive message in the tenant as a field
func sendTenantKeepAlive(t *domain.Tenant) {
	globals.TenantsLock.Lock() // to safely read from tenant's fields
	defer globals.TenantsLock.Unlock()

	ka, err := loadKeepAliveXML(t.KeepAliveFile)
	if err != nil {
		log.Printf("[WARN][Tenant %q] Could not load keep-alive file (%s): %v. Creating default.", t.Name, t.KeepAliveFile, err)
		ka = &domain.KeepAliveXML{
			TenantName: t.Name,
		}
	}

	now := time.Now().UTC().Format(time.RFC3339)
	ka.TenantName = t.Name
	ka.SendTime = now

	if wErr := saveKeepAliveXML(t.KeepAliveFile, ka); wErr != nil {
		log.Printf("[ERROR][Tenant %q] Could not write keep-alive file: %v", t.Name, wErr)
	}

	xmlBytes, marshalErr := xml.MarshalIndent(ka, "", "  ")
	if marshalErr != nil {
		log.Printf("[ERROR][Tenant %q] Could not marshal keep-alive XML: %v", t.Name, marshalErr)
		return
	}

	// Frame it with StartByte/EndByte
	framed := append([]byte{t.StartByte}, append(xmlBytes, t.EndByte)...)

	t.ConnectionsLock.Lock()
	defer t.ConnectionsLock.Unlock()
	for _, conn := range t.Connections {
		if conn == nil {
			continue
		}
		n, err := conn.Write(framed)
		if err != nil {
			logError(t, fmt.Errorf("keep-alive write error to %s: %v", conn.RemoteAddr(), err))
			continue
		}
		atomic.AddUint64(&t.BytesSent, uint64(n))
	}

	log.Printf("[Tenant %q] Keep-alive sent at %s to %d connection(s).", t.Name, now, len(t.Connections))
}

func loadKeepAliveXML(filePath string) (*domain.KeepAliveXML, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, err
	}
	var ka domain.KeepAliveXML
	if err := xml.Unmarshal(data, &ka); err != nil {
		return nil, err
	}
	return &ka, nil
}

func saveKeepAliveXML(filePath string, ka *domain.KeepAliveXML) error {
	data, err := xml.MarshalIndent(ka, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filePath, data, 0644)
}
