-- Static File Web Server (SERVER FUNCTIONALITY REMOVED)
-- This example previously demonstrated static file serving functionality
-- which has been removed from Hype. Only HTTP client functionality remains.

local http = require('http')

print("=== Static File Server Functionality Removed ===")
print("HTTP server functionality has been removed from Hype.")
print("Static file serving is no longer available.")
print("Only HTTP client functionality (http.get, http.post, etc.) remains.")
print("")

-- Show original command line arguments that would have been used
print("Original command line arguments:")
for i = 1, #arg do
    print("  arg[" .. i .. "] = " .. arg[i])
end
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
print("=== Alternatives ===")
print("For static file serving, consider:")
print("  - Using a dedicated web server (nginx, Apache)")
print("  - Python's http.server module")
print("  - Node.js with express")
print("  - Go's net/http package")
print("")
print("For HTTP client functionality in Hype:")
print("  - http.get(url)")
print("  - http.post(url, body)")
print("  - http.put(url, body)")
print("  - http.delete(url)")
print("  - http.request(method, url, options)")
print("")
print("See test-http.lua for HTTP client examples.")