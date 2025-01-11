package main

import (
	"log"

	"tcp_sandbox/controller"
	"tcp_sandbox/service"
)

func main() {
	const tenantFile = "tenants.json"

	if err := service.LoadTenantsFromFile(tenantFile); err != nil {
		log.Fatalf("Failed to load tenants from file: %v", err)
	}

	service.StartAllTenants()

	go controller.StartRESTServer()

	go service.StartTenantFileManager(tenantFile)

	select {} // block forever
}
