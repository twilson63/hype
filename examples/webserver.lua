-- HTTP Server Example (SERVER FUNCTIONALITY REMOVED)
-- This example previously demonstrated HTTP server functionality
-- which has been removed from Hype. Only HTTP client functionality remains.

local http = require('http')

print("=== HTTP Server Functionality Removed ===")
print("HTTP server functionality has been removed from Hype.")
print("Only HTTP client functionality (http.get, http.post, etc.) is available.")
print("")
print("This example previously created an HTTP server on port 8080.")
print("For HTTP client examples, see test-http.lua")
print("")

-- Attempt to create server will fail with error message
local server, err = http.newServer()
if server == nil then
    print("Server creation failed (expected):", err)
else
    print("Unexpected: Server was created")
end

print("")
print("=== HTTP Client Example ===")
print("Testing HTTP client functionality...")

-- Test HTTP client functionality (this still works)
local response, err = http.get("https://httpbin.org/get")
if response then
    print("✓ HTTP client GET request successful")
    print("  Status:", response.status)
    print("  Response body length:", #response.body)
else
    print("✗ HTTP client GET request failed:", err)
end