-- Simple HTTP Server Test (SERVER FUNCTIONALITY REMOVED)
-- This example previously demonstrated simple HTTP server functionality
-- which has been removed from Hype. Only HTTP client functionality remains.

local http = require('http')

print("=== HTTP Server Functionality Removed ===")
print("HTTP server functionality has been removed from Hype.")
print("Only HTTP client functionality (http.get, http.post, etc.) is available.")
print("")

-- Attempt to create server will fail with error message
print("Attempting to create server...")
local server, err = http.newServer()
if server == nil then
    print("✓ Server creation failed as expected:", err)
else
    print("✗ Unexpected: Server was created")
end

print("")
print("=== HTTP Client Alternative ===")
print("Use HTTP client functionality instead:")
print("  - http.get(url)")
print("  - http.post(url, body)")
print("  - http.put(url, body)")
print("  - http.delete(url)")
print("  - http.request(method, url, options)")
print("")
print("See test-http.lua for HTTP client examples.")