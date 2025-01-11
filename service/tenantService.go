package service

import (
	"encoding/json"
	"fmt"
	"log"
	"net"
	"os"
	"tcp_sandbox/domain"
	"time"

	"tcp_sandbox/globals"
)

// -----------------------------------------------------------
// Tenant Manager (Loading JSON, Starting/Stopping Listeners)
// -----------------------------------------------------------

func StartTenantFileManager(filename string) {
	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()

	for {
		<-ticker.C
		if err := LoadTenantsFromFile(filename); err != nil {
			log.Printf("[ERROR] Could not reload tenants file: %v", err)
			continue
		}
		SyncListeners()
		if err := SaveTenantsToFile(filename); err != nil {
			log.Printf("[ERROR] Could not save tenants file: %v", err)
		}
		printAllTenantsStatus()
	}
}

// loadTenantsFromFile merges the file tenants into our global map.
func LoadTenantsFromFile(filename string) error {
	data, err := os.ReadFile(filename)
	if err != nil {
		return err
	}
	var fileTenants []domain.Tenant
	if err := json.Unmarshal(data, &fileTenants); err != nil {
		return fmt.Errorf("json unmarshal error: %w", err)
	}

	globals.TenantsLock.Lock()
	defer globals.TenantsLock.Unlock()

	filePorts := make(map[string]bool)

	for i := range fileTenants {
		ft := &fileTenants[i]
		port := ft.Port
		filePorts[port] = true

		existing, ok := globals.Tenants[port]
		if !ok {
			// new tenant
			globals.Tenants[port] = ft
		} else {
			// update existing's config, keep counters & tokens
			existing.Name = ft.Name
			existing.Comment = ft.Comment
			existing.StartByte = ft.StartByte
			existing.EndByte = ft.EndByte
			existing.SimpleAuthToken = ft.SimpleAuthToken
			existing.OAuthCredentials.ClientID = ft.OAuthCredentials.ClientID
			existing.OAuthCredentials.ClientSecret = ft.OAuthCredentials.ClientSecret
			existing.OAuthCredentials.TokenURL = ft.OAuthCredentials.TokenURL
			existing.OAuthCredentials.Scopes = ft.OAuthCredentials.Scopes

			// Keep-alive fields
			existing.KeepAliveIntervalSec = ft.KeepAliveIntervalSec
			existing.KeepAliveFile = ft.KeepAliveFile
			// Keep auth fields
			existing.Endpoint = ft.Endpoint
		}
	}

	// Mark removals
	for port := range globals.Tenants {
		if !filePorts[port] {
			// Mark removed
			globals.Tenants[port].Name = ""
		}
	}

	return nil
}

func SyncListeners() {
	for port, t := range globals.Tenants {
		if t.Name == "" {
			// removed tenant
			StopTenantListener(port)
			delete(globals.Tenants, port)
			continue
		}
		if _, ok := globals.Listeners[port]; !ok {
			if err := startTenantListener(port, t); err != nil {
				log.Printf("[ERROR] Failed to start listener for tenant %q on port %s: %v", t.Name, port, err)
			}
		}
	}
	for port, ln := range globals.Listeners {
		if _, ok := globals.Tenants[port]; !ok {
			// tenant no longer in memory
			log.Printf("Stopping listener on port %s (removed tenant)", port)
			_ = ln.Close()
			delete(globals.Listeners, port)
		}
	}
}

func SaveTenantsToFile(filename string) error {
	globals.TenantsLock.Lock()
	defer globals.TenantsLock.Unlock()

	var out []domain.Tenant
	for _, t := range globals.Tenants {
		out = append(out, *t)
	}
	data, err := json.MarshalIndent(out, "", "  ")
	if err != nil {
		return fmt.Errorf("json marshal error: %w", err)
	}
	return os.WriteFile(filename, data, 0644)
}

func StartAllTenants() {
	globals.TenantsLock.Lock()
	defer globals.TenantsLock.Unlock()

	for port, t := range globals.Tenants {
		if _, ok := globals.Listeners[port]; !ok {
			if err := startTenantListener(port, t); err != nil {
				log.Printf("[ERROR] Failed to start listener for tenant %q on port %s: %v", t.Name, port, err)
			}
		}
	}
}

// startTenantListener begins listening on a port; also starts keep-alive if needed.
func startTenantListener(port string, t *domain.Tenant) error {
	ln, err := net.Listen("tcp", ":"+port)
	if err != nil {
		return err
	}
	globals.Listeners[port] = ln

	log.Printf("Listening for tenant %q on port %s", t.Name, port)

	// Start keep-alive routine if configured
	StartKeepAliveRoutine(t)

	go func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				log.Printf("Listener stopped on port %s (tenant %q). Err: %v", port, t.Name, err)
				return
			}
			addConnection(t, conn)
			log.Printf("Accepted connection from %s for tenant %q (port %s)", conn.RemoteAddr(), t.Name, port)
			go handleConnection(conn, t)
		}
	}()
	return nil
}

func StopTenantListener(port string) {
	ln, ok := globals.Listeners[port]
	if !ok {
		return
	}
	log.Printf("Stopping listener on port %s", port)
	_ = ln.Close()
	delete(globals.Listeners, port)
}
