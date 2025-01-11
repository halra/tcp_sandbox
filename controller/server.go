package controller

import (
	// ...
	"encoding/json"
	"io"
	"log"
	"net/http"

	"tcp_sandbox/domain"
	"tcp_sandbox/globals"
	"tcp_sandbox/service"
)

// startRESTServer
func StartRESTServer() {
	http.HandleFunc("/patch", handlePatchTenants)
	log.Printf("REST server listening on :8080")
	err := http.ListenAndServe(":8080", nil)
	if err != nil {
		log.Fatalf("HTTP server failed: %v", err)
	}
}

// handlePatchTenants expects a JSON array or single object describing partial Tenant updates.
// If "Remove" is true in the incoming data for a Tenant, that tenant is removed from the system.
//
// Example PATCH/POST body for removing a tenant on port 4000:
//
//  [
//    {
//      "Port": "4000",
//      "Remove": true
//    }
//  ]
//
// Example for patching a tenant on port 3000:
//
//  [
//    {
//      "Port": "3000",
//      "Name": "NewTenantA",
//      "Comment": "Updated comment...",
//      "KeepAliveIntervalSec": 60
//    }
//  ]
//
func handlePatchTenants(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPatch && r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Read request body
	body, err := io.ReadAll(r.Body)
	if err != nil {
		log.Printf("Error reading body: %v", err)
		http.Error(w, "Bad request", http.StatusBadRequest)
		return
	}
	defer r.Body.Close()

	var patchTenants []domain.Tenant
	if err := json.Unmarshal(body, &patchTenants); err != nil {
		// single tenant
		var single domain.Tenant
		if err2 := json.Unmarshal(body, &single); err2 != nil {
			log.Printf("Invalid patch body: %v", err2)
			http.Error(w, "Invalid JSON", http.StatusBadRequest)
			return
		}
		patchTenants = append(patchTenants, single)
	}

	globals.TenantsLock.Lock()
	defer globals.TenantsLock.Unlock()

	for i := range patchTenants {
		pt := &patchTenants[i]

		if pt.Port == "" {
			// Port is our primary key
			log.Printf("Patch data missing 'Port' field; skipping entry: %+v", pt)
			continue
		}

		if pt.Remove {
			// remove this tenant
			_, ok := globals.Tenants[pt.Port]
			if !ok {
				log.Printf("Tenant on port %s not found; can't remove. Skipping.", pt.Port)
				continue
			}
			// Stop listener and remove from map
			service.StopTenantListener(pt.Port)
			delete(globals.Tenants, pt.Port)
			log.Printf("Removed tenant on port %s via PATCH request.", pt.Port)
		} else {
			existing, ok := globals.Tenants[pt.Port]
			if !ok {
				// Create a new tenant if not found
				globals.Tenants[pt.Port] = pt
				log.Printf("Created a new tenant on port %s (via PATCH).", pt.Port)
			} else {
				// Patch existing
				if pt.Name != "" {
					existing.Name = pt.Name
				}
				if pt.Comment != "" {
					existing.Comment = pt.Comment
				}
				if pt.StartByte != 0 {
					existing.StartByte = pt.StartByte
				}
				if pt.EndByte != 0 {
					existing.EndByte = pt.EndByte
				}
				if pt.SimpleAuthToken != "" {
					existing.SimpleAuthToken = pt.SimpleAuthToken
				}

				// OAuth credentials
				if pt.OAuthCredentials.ClientID != "" {
					existing.OAuthCredentials.ClientID = pt.OAuthCredentials.ClientID
				}
				if pt.OAuthCredentials.ClientSecret != "" {
					existing.OAuthCredentials.ClientSecret = pt.OAuthCredentials.ClientSecret
				}
				if pt.OAuthCredentials.TokenURL != "" {
					existing.OAuthCredentials.TokenURL = pt.OAuthCredentials.TokenURL
				}
				if len(pt.OAuthCredentials.Scopes) > 0 {
					existing.OAuthCredentials.Scopes = pt.OAuthCredentials.Scopes
				}

				// Keep-alive fields
				if pt.KeepAliveIntervalSec != 0 {
					existing.KeepAliveIntervalSec = pt.KeepAliveIntervalSec
				}
				if pt.KeepAliveFile != "" {
					existing.KeepAliveFile = pt.KeepAliveFile
				}
				log.Printf("Patched tenant on port %s: %+v", pt.Port, pt)
			}
		}
	}

	// Re-sync listeners to handle newly created or re-added tenants
	service.SyncListeners()

	// Save updated tenants to file
	if err := service.SaveTenantsToFile("tenants.json"); err != nil {
		log.Printf("Failed to save tenants after patch: %v", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
	w.Write([]byte(`{"status":"ok"}`))
}
