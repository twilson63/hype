-- Simple HTTP client example (TUI browser removed)
local http = require('http')

-- Function to fetch and display webpage content
function fetch_page(url)
    print("Fetching: " .. url)
    
    local response, err = http.get(url)
    if err then
        print("Error: " .. err)
        return
    end
    
    print("Status: " .. response.status)
    print("Headers:")
    for k, v in pairs(response.headers) do
        print("  " .. k .. ": " .. v)
    end
    print("\nContent (first 500 chars):")
    local content = response.body
    if #content > 500 then
        content = content:sub(1, 500) .. "..."
    end
    print(content)
end

-- Example usage
print("Hype HTTP Client Example")
print("========================")
print()

-- Fetch a test page
fetch_page("https://httpbin.org/json")

print()
print("Note: TUI browser functionality has been removed from Hype.")
print("Use this HTTP module for web requests in your applications.")