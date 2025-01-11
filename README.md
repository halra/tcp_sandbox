# TCP sandbox

A **Proof of Concept** showcasing a multi-tenant TCP server in Go that:
- Loads tenant configurations from a JSON file
- Dynamically adds/removes tenants
- Supports optional keep-alive messages in XML
- Demonstrates basic start/end byte framing of messages
- Offers a simple REST API to patch (create, update, or remove) tenants

> **Warning**: This project is provided as a **proof of concept** and comes **without any warranty** or guarantee. It is **not** intended for production use. Use at your own risk.

---

## Features

- **Multi-Tenant by Port**  
  Each port corresponds to one tenant, loaded from a JSON file.

- **Tenant Keep-Alive**  
  Tenants can optionally send XML keep-alive messages at a configurable interval.

- **Simple Auth vs. OAuth**  
  - Simple auth tokens (`X-Auth`)  
  - OAuth (client-credentials flow), auto-refreshing tokens.

- **Runtime Config Reload**  
  A background routine periodically re-checks the JSON file, adding or removing tenants on the fly.

- **PATCH Endpoint**  
  A REST endpoint (`/patch`) lets you create, update, or remove tenants at runtime.

---

## Installation

1. Clone the repository:

   ```bash
   git clone https://github.com/halra/tcp_sandbox
   cd yourrepo
   ```

2. Build the project:

   ```bash
   go build -o tcp_sandbox
   ```

3. Prepare your `tenants.json` file (see example in the repo).

4. Run the server:

   ```bash
   ./tcp_sandbox
   ```

---

## Usage

1. **Configure Tenants**  
   Edit the `tenants.json` to define your tenant ports, credentials, keep-alive settings, etc. The server will periodically reload this file.

2. **Test TCP Connections**  
   - For a tenant running on port `3000`, connect with `nc 127.0.0.1 3000`.  
   - Send messages framed by the configured start/end bytes (e.g., `0x02` ... `0x03`).

3. **Check Keep-Alive**  
   If a tenant has a keep-alive interval and file set, it will send periodic XML messages to connected clients.

4. **PATCH Endpoint**  
   Use `curl` or another tool to update or remove tenants at runtime:
   ```bash
   curl -X PATCH -H "Content-Type: application/json" \
     -d '[{"Port":"3000","Comment":"New comment"}]' \
     http://localhost:8080/patch
   ```
   Or remove a tenant:
   ```bash
   curl -X PATCH -H "Content-Type: application/json" \
     -d '[{"Port":"4000","Remove":true}]' \
     http://localhost:8080/patch
   ```

---

## Disclaimer

1. **No Warranty**  
   This code is provided **as is**, **without any warranty**, either expressed or implied. The entire risk as to the quality and performance is with you.

2. **Proof of Concept Only**  
   This project is purely for demonstration and educational purposes. It is **not** production-ready. Use, modify, and experiment at your own risk.

3. **Security**  
   There is **no** built-in security or encryption. If you plan to use any part of this code in a real environment, **implement TLS, authentication, and other security best practices**.

---

## License

This project is available under the [MIT License](LICENSE).  