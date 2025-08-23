# Hype - Lua Script to Executable Packager

Hype is a powerful tool that packages Lua scripts into standalone executables with TUI (Terminal User Interface), HTTP, and embedded database support. It combines a Lua runtime with your scripts to create cross-platform applications with zero external dependencies.

## Features

- 📦 **Package Lua scripts into standalone executables**
- 🌍 **Cross-platform support** (Linux, macOS, Windows)
- 🖥️ **Built-in TUI library** for creating terminal applications
- 🌐 **HTTP client and server** support for web applications
- 🔌 **WebSocket server and client** for real-time communication
- 🗄️ **Embedded key-value database** with BoltDB
- 🔐 **Cryptography module** with JWK support, RSA/RSA-PSS/ECDSA/Ed25519 signatures, and SHA-256/384/512 hashing
- 🔄 **Transaction support** with ACID properties
- 🔍 **Database iteration and querying** with cursor support
- 📁 **Multi-file project support** with dependency bundling
- 🔌 **Plugin system** with versioned Lua modules
- ✨ **Zero external dependencies** in final executables
- 🚀 **Simple deployment** - single binary distribution
- 🎯 **Interactive REPL** with TUI interface for testing and exploration

## Installation

### Option 1: Download Pre-built Binaries (Recommended)

**macOS (Easy Install):**
```bash
curl -sSL https://raw.githubusercontent.com/twilson63/hype/main/install-mac.sh | bash
```

**Manual Download:**
1. Download the appropriate binary from [GitHub Releases](https://github.com/twilson63/hype/releases)
2. For macOS: `hype-darwin-amd64` (Intel) or `hype-darwin-arm64` (Apple Silicon)
3. Make executable: `chmod +x hype-*`
4. Move to PATH: `mv hype-* /usr/local/bin/hype`

**macOS Gatekeeper Fix:**
If macOS blocks the binary, run:
```bash
xattr -d com.apple.quarantine /path/to/hype
```

### Option 2: Build from Source

```bash
git clone https://github.com/twilson63/hype.git
cd hype
go build -o hype .
```

## Quick Start

```bash
# Create a simple TUI app
echo 'local app = tui.newApp()
local text = tui.newTextView("Hello from Hype!")
app:SetRoot(text, true)
app:Run()' > hello.lua

# Build and run
./hype build hello.lua -o hello
./hello
```

## Usage

### Command Line Interface

```bash
# Build a Lua script into an executable (auto-bundles dependencies)
./hype build script.lua

# Specify output name
./hype build script.lua -o myapp

# Build for different platforms
./hype build script.lua -t linux
./hype build script.lua -t windows
./hype build script.lua -t darwin

# Bundle multi-file Lua projects into single file (optional)
./hype bundle script.lua
./hype bundle script.lua -o bundled-script.lua

# Run a Lua script directly (development/testing)
./hype run script.lua

# Pass arguments to Lua scripts
./hype run server.lua -- --port 8080 --dir ./public

# Start interactive REPL (v1.9.0+)
./hype repl                    # TUI-based REPL
./hype repl --simple          # Simple terminal REPL
```

## Multi-File Projects

Hype supports multi-file Lua projects through its bundling system. You can split your code across multiple files and use `require()` to import them:

```lua
-- utils.lua
local M = {}

function M.greet(name)
    return "Hello, " .. (name or "World") .. "!"
end

return M
```

```lua
-- main.lua
local utils = require('./utils')
local http = require('http')

print(utils.greet("Hype"))

local server = http.newServer()
server:handle("/", function(req, res)
    res:json({ message = utils.greet("API User") })
end)
server:listen(8080)
```

**Building Multi-File Projects:**
```bash
# Build automatically bundles dependencies
./hype build main.lua -o myapp

# Optional: Bundle into single file first
./hype bundle main.lua -o bundled.lua
./hype build bundled.lua -o myapp
```

**Module Resolution:**
- Relative paths: `require('./utils')`, `require('../shared/helpers')`
- Module names: `require('utils')` (looks for `utils.lua` or `utils/init.lua`)
- Built-in modules: `require('http')`, `require('kv')`, `require('tui')`, `require('websocket')`, `require('crypto')` (always available)

## Development Mode

For faster development and testing, Hype provides a `run` command that runs Lua scripts directly without building executables:

```bash
# Run script directly (great for development)
./hype run myapp.lua

# Run multi-file projects (auto-resolves dependencies)
./hype run examples/showcase.lua
```

**Benefits of run mode:**
- ⚡ **Instant execution** - no build step required
- 🔄 **Rapid iteration** - test changes immediately  
- 🐛 **Easy debugging** - direct error output
- 📦 **Same APIs** - identical behavior to built executables
- 🛠️ **Development workflow** - perfect for prototyping

**When to use run vs build vs bundle:**
- Use `run` during development and testing
- Use `build` for production deployments (auto-handles dependencies)
- Use `bundle` when you need a single Lua file (optional step)

## Plugin System

Hype features a powerful plugin system that allows you to extend functionality with custom Lua modules. Plugins can be easily shared, versioned, and embedded into your applications.

### Using Plugins

```bash
# Use a plugin by name with automatic discovery
./hype run myapp.lua --plugins fs

# Specify exact version
./hype run myapp.lua --plugins fs@1.0.0

# Use multiple plugins
./hype run myapp.lua --plugins fs@1.0.0,json,http-utils@2.1.0

# Custom alias for plugins
./hype run myapp.lua --plugins myfs=./path/to/plugin@1.2.0

# Build with embedded plugins
./hype build myapp.lua --plugins fs@1.0.0 -o myapp
```

**In your Lua scripts:**
```lua
-- Use plugins like built-in modules
local fs = require("fs")
local content, err = fs.readFile("config.txt")
if content then
    print("File content:", content)
else
    print("Error:", err)
end
```

### Plugin Specification Formats

- **`fs`** - Simple name, auto-discovers in conventional locations
- **`fs@1.0.0`** - Name with specific version requirement  
- **`myfs=./path/to/plugin`** - Custom alias with explicit path
- **`myfs=./path/to/plugin@2.0.0`** - Alias with path and version
- **`github.com/user/plugin@v1.0.0`** - Go module (future support)

### Creating Lua Plugins

**Plugin Structure:**
```
my-plugin/
├── hype-plugin.yaml    # Plugin manifest
└── plugin.lua          # Main plugin code
```

**hype-plugin.yaml:**
```yaml
name: "my-plugin"
version: "1.0.0"
type: "lua"
main: "plugin.lua"
description: "Description of your plugin"
author: "your-name"
license: "MIT"
```

**plugin.lua:**
```lua
-- Plugin must return a table with functions
local myplugin = {}

function myplugin.hello(name)
    return "Hello, " .. (name or "World") .. "!"
end

function myplugin.calculate(a, b)
    if not a or not b then
        return nil, "Both arguments required"
    end
    return a + b, nil
end

return myplugin
```

**Usage in scripts:**
```lua
local myplugin = require("my-plugin")

local greeting = myplugin.hello("Hype")
print(greeting)  -- "Hello, Hype!"

local result, err = myplugin.calculate(5, 3)
if result then
    print("Result:", result)  -- "Result: 8"
else
    print("Error:", err)
end
```

### Plugin Discovery

Hype automatically searches for plugins in conventional locations:
- `./plugins/[name]/`
- `./examples/plugins/[name]/` 
- `./[name]-plugin/`
- `./examples/plugins/[name]-plugin/`

### Plugin Examples

See `examples/plugins/` for working plugin examples:
- **`fs-plugin`** - Filesystem operations (read, write, list, mkdir)
- **`fs-plugin-v2`** - Enhanced filesystem with copy, move, delete

```bash
# Try the filesystem plugin
./hype run examples/test-fs-plugin.lua --plugins fs@1.0.0

# Try the enhanced version
./hype run examples/test-versioned-plugins.lua --plugins fs=./examples/plugins/fs-plugin-v2@2.0.0
```

**📖 For comprehensive plugin documentation and development guide, see:**
- **[docs/PLUGINS.md](docs/PLUGINS.md)** - Complete plugin system reference
- **[docs/PLUGIN_DEVELOPMENT.md](docs/PLUGIN_DEVELOPMENT.md)** - Step-by-step plugin development guide

## Command Line Arguments

Hype scripts can access command line arguments through the global `arg` table:

```lua
-- Access command line arguments
print("Script name:", arg[0])
print("First argument:", arg[1])
print("Second argument:", arg[2])

-- Parse common patterns
local port = 8080
local directory = "./public"

for i = 1, #arg do
    if arg[i] == "--port" and arg[i+1] then
        port = tonumber(arg[i+1])
    elseif arg[i] == "--dir" and arg[i+1] then
        directory = arg[i+1]
    end
end

print("Server will run on port", port, "serving", directory)
```

**Usage:**
```bash
# In run mode (use -- to separate script args)
./hype run server.lua -- --port 3000 --dir /var/www

# In built executables (direct arguments)
./server --port 3000 --dir /var/www
```

## API Reference

📚 **NEW: Comprehensive Documentation Available!**
- **[Enhanced API Reference](https://twilson63.github.io/hype/api-enhanced.html)** - Interactive docs with real examples
- **[Complete Plugin Development Guide](docs/plugin-development-enhanced.md)** - Build powerful plugins
- **[AI-Friendly Reference](docs/llms.txt)** - For AI assistants and LLMs
- **[Full Technical Reference](docs/llms-full.txt)** - Everything about Hype

### TUI Module

Create beautiful terminal user interfaces:

```lua
local tui = require('tui') -- Available by default

-- Core components
local app = tui.newApp()              -- Main application
local textView = tui.newTextView(text) -- Text display
local inputField = tui.newInputField() -- Text input
local button = tui.newButton(label)   -- Clickable button
local flex = tui.newFlex()            -- Layout container
```

### HTTP Module

Build web applications and APIs with full HTTP client and server support:

```lua
local http = require('http')
```

#### HTTP Client

Make HTTP requests with support for headers and timeouts:

```lua
-- Simple GET request
local response, err = http.get("https://api.example.com/data")
if not err then
    print("Status:", response.status)
    print("Body:", response.body)
end

-- GET with options (headers, timeout)
local response, err = http.get("https://api.example.com/data", {
    timeout = 30,
    headers = { ["User-Agent"] = "Hype/1.0", ["Authorization"] = "Bearer token" }
})
```

#### HTTP Server

Create powerful web servers with routing and JSON responses:

```lua
local server = http.newServer()

-- Basic route handler
server:handle("/", function(req, res)
    res:write("Hello from Hype server!")
end)

-- JSON API endpoint
server:handle("/api/users", function(req, res)
    res:json({ 
        message = "Hello from Hype API!",
        method = req.method,
        url = req.url 
    })
end)

-- Handle different HTTP methods
server:handle("/api/data", function(req, res)
    if req.method == "GET" then
        res:json({ data = "Here's your data" })
    elseif req.method == "POST" then
        -- Access request body
        local body = req.body
        res:json({ received = body, status = "created" })
    else
        res:json({ error = "Method not allowed" })
    end
end)

-- Start server
server:listen(8080)
print("Server running on http://localhost:8080")

-- Keep server running (for standalone scripts)
while true do
    os.execute("sleep 1")
end
```

#### Server Methods

**Response Methods:**
- `res:write(text)` - Send plain text response
- `res:json(table)` - Send JSON response (auto-sets Content-Type)

**Request Properties:**
- `req.method` - HTTP method (GET, POST, etc.)
- `req.url` - Request URL path
- `req.body` - Request body content

**Server Methods:**
- `server:handle(path, handler)` - Add route handler
- `server:listen(port)` - Start server on port
- `server:stop()` - Stop server gracefully

### WebSocket Module

Build real-time applications with WebSocket support for bidirectional communication:

```lua
local websocket = require('websocket')
```

#### WebSocket Server

Create WebSocket servers for real-time communication:

```lua
local server = websocket.newServer()

-- Handle WebSocket connections
server:handle("/ws", function(conn)
    print("New WebSocket connection established")
    
    -- Set up event handlers
    conn:onMessage(function(message)
        print("Received:", message.data)
        -- Echo message back to client
        conn:send("Echo: " .. message.data)
    end)
    
    conn:onClose(function()
        print("Connection closed")
    end)
    
    conn:onError(function(err)
        print("WebSocket error:", err)
    end)
    
    -- Send welcome message
    conn:send("Welcome to WebSocket server!")
end)

-- Start server
server:listen(8080)
print("WebSocket server running at ws://localhost:8080/ws")

-- Keep server running
while true do
    os.execute("sleep 1")
end
```

#### WebSocket Client

Connect to WebSocket servers and handle real-time communication:

```lua
-- Connect to WebSocket server
local client = websocket.connect("ws://localhost:8080/ws")

if not client then
    print("Failed to connect to WebSocket server")
    os.exit(1)
end

-- Set up client event handlers
client:onMessage(function(message)
    print("Received:", message.data)
    print("Message type:", message.type) -- "text" or "binary"
end)

client:onClose(function()
    print("Connection closed")
end)

client:onError(function(err)
    print("WebSocket error:", err)
end)

-- Send messages
client:send("Hello from Lua client!")
client:sendBinary("Binary data")

-- Ping the server
client:ping()

-- Close connection when done
client:close()
```

#### WebSocket Methods

**Server Methods:**
- `websocket.newServer()` - Create new WebSocket server
- `server:handle(path, handler)` - Add WebSocket route handler
- `server:listen(port)` - Start server on port
- `server:stop()` - Stop server gracefully

**Client Methods:**
- `websocket.connect(url)` - Connect to WebSocket server

**Connection Methods (both server and client):**
- `conn:send(message)` - Send text message
- `conn:sendBinary(data)` - Send binary message
- `conn:onMessage(handler)` - Set message handler
- `conn:onClose(handler)` - Set close handler
- `conn:onError(handler)` - Set error handler
- `conn:close()` - Close connection
- `conn:ping()` - Send ping frame

**Message Object:**
- `message.data` - Message content as string
- `message.type` - Message type ("text" or "binary")

### Key-Value Database

Embedded database with no external dependencies:

```lua
local kv = require('kv')

-- Open database
local db, err = kv.open("./myapp.db")

-- Create bucket and basic operations
db:open_db("users")
db:put("users", "user1", "John Doe")
local value, err = db:get("users", "user1")
db:delete("users", "user1")

-- Transactions
local txn, err = db:begin_txn(false)
txn:put("users", "user2", "Jane Smith")
txn:commit()

-- Iteration and querying
local keys, err = db:keys("users", "admin:") -- Prefix search
db:foreach("users", function(key, value)
    print(key .. " = " .. value)
    return true -- continue iteration
end)

db:close()
```

### Crypto Module

Professional-grade cryptography with JWK (JSON Web Key) support:

```lua
local crypto = require('crypto')
```

#### Key Generation and Signatures

Generate cryptographic keys and create/verify digital signatures:

```lua
-- Generate keys (supports RS256, PS256, ES256, EdDSA, etc.)
local private_key = crypto.generate_jwk("ES256")
local public_key = crypto.jwk_to_public(private_key)

-- Generate 4096-bit RSA key for enhanced security
local rsa_4096 = crypto.generate_jwk("RS256", 4096)

-- Sign and verify data
local message = "Important message"
local signature = crypto.sign(private_key, message)
local is_valid = crypto.verify(public_key, message, signature)

-- Key management
local key_json = crypto.jwk_to_json(private_key)
local loaded_key = crypto.jwk_from_json(key_json)
local thumbprint = crypto.jwk_thumbprint(public_key)
```

#### Hashing Functions

Compute SHA-256/384/512 hashes with support for complex data structures:

```lua
-- Basic hashing
local sha256 = crypto.sha256("data to hash")
local sha384 = crypto.sha384("data to hash")
local sha512 = crypto.sha512("data to hash")

-- Generic hash function
local hash = crypto.hash("sha384", "data to hash")

-- Deep hash for complex data structures
local complex_data = {
    user = "alice",
    permissions = {"read", "write"},
    metadata = { version = "1.0", created = os.time() }
}

-- Produces consistent hash regardless of key order
local deep_hash = crypto.deep_hash(complex_data)
local deep_sha256 = crypto.deep_hash(complex_data, "sha256")
```

**Supported Algorithms:**
- **RSA:** RS256, RS384, RS512 (PKCS#1 v1.5)
- **RSA-PSS:** PS256, PS384, PS512 (PSS padding)
- **ECDSA:** ES256, ES384, ES512
- **EdDSA:** Ed25519

## Interactive REPL (v1.9.0+)

Hype includes a powerful interactive REPL (Read-Eval-Print Loop) for testing Lua code and exploring the API:

### TUI REPL (Default)

```bash
./hype repl
```

The TUI REPL provides:
- **Two-panel interface**: Output panel (top) and input panel (bottom)
- **Beautiful table formatting**: Lua tables are displayed with proper formatting
- **Command history**: Access previous commands with `:history` or `:h`
- **Special commands**:
  - `:help` - Show available commands
  - `:history` or `:h` - Display command history
  - `:h=N` - Recall command N from history
  - `:clear` - Clear the output panel
- **Full module access**: All Hype modules (tui, http, kv, crypto, ws) are available

### Simple REPL

```bash
./hype repl --simple
```

For a basic command-line REPL without the TUI interface.

### REPL Examples

```lua
-- Test expressions
hype> 2 + 2
4

-- Explore tables with nice formatting
hype> {name="John", age=30, hobbies={"coding", "gaming"}}
{
  ["age"] = 30,
  ["hobbies"] = {
    [1] = "coding",
    [2] = "gaming"
  },
  ["name"] = "John"
}

-- Access Hype modules
hype> crypto.sha256("test")
9f86d081884c7d659a2feaa0c55ad015a3bf4f1b2b0b822cd15d6c15b0f00a08

-- Use special commands
hype> :history
Command History:
  1: 2 + 2
  2: {name="John", age=30, hobbies={"coding", "gaming"}}
  3: crypto.sha256("test")

hype> :h=2  -- Recall command 2
Recalled: {name="John", age=30, hobbies={"coding", "gaming"}}
```

## Examples

### Simple TUI Application

```lua
local app = tui.newApp()
local textView = tui.newTextView("Hello, World from Hype!\n\nPress Ctrl+C to exit.")

textView:SetWrap(true)
textView:SetWordWrap(true)

app:SetRoot(textView, true)
app:Run()
```

### Interactive Form

```lua
local app = tui.newApp()
local flex = tui.newFlex()
local textView = tui.newTextView("Enter your information:")
local nameField = tui.newInputField()
local submitButton = tui.newButton("Submit")

nameField:SetLabel("Name: ")
nameField:SetPlaceholder("Enter your name")

submitButton:SetSelectedFunc(function()
    local name = nameField:GetText()
    textView:SetText("Hello, " .. name .. "!")
end)

flex:SetDirection(1) -- Column layout
flex:AddItem(textView, 1, 0, false)
flex:AddItem(nameField, 1, 0, true)
flex:AddItem(submitButton, 1, 0, false)

app:SetRoot(flex, true)
app:Run()
```

### Web Server with Database

```lua
local http = require('http')
local kv = require('kv')

-- Setup database
local db = kv.open("./users.db")
db:open_db("users")

-- Create web server
local server = http.newServer()

-- API endpoint to create user
server:handle("/users", function(req, res)
    if req.method == "POST" then
        local id = "user" .. os.time()
        db:put("users", id, req.body)
        res:json({ id = id, message = "User created" })
    elseif req.method == "GET" then
        local users = {}
        db:foreach("users", function(key, value)
            users[key] = value
            return true
        end)
        res:json(users)
    end
end)

print("Server starting on http://localhost:8080")
server:listen(8080)

-- Keep server running
while true do
    os.execute("sleep 1")
end
```

### Static File Web Server

```lua
-- Static file server with command line arguments
local http = require('http')

-- Parse command line arguments
local port = 8080
local directory = "./public"

for i = 1, #arg do
    if arg[i] == "--port" and arg[i+1] then
        port = tonumber(arg[i+1])
    elseif arg[i] == "--dir" and arg[i+1] then
        directory = arg[i+1]
    end
end

local server = http.newServer()

-- Serve static files
server:handle("/", function(req, res)
    local path = req.url == "/" and "/index.html" or req.url
    local filepath = directory .. path
    
    local file = io.open(filepath, "r")
    if file then
        local content = file:read("*all")
        file:close()
        res:write(content)
    else
        res:json({ error = "File not found", path = path })
    end
end)

print("Server running on http://localhost:" .. port)
server:listen(port)

-- Keep running
while true do
    os.execute("sleep 1")
end
```

**Usage:**
```bash
# Default settings (port 8080, ./public directory)
./hype run server.lua

# Custom port and directory
./hype run server.lua -- --port 3000 --dir /var/www
```

### Real-Time Chat Server (WebSocket)

```lua
-- WebSocket chat server with message broadcasting
local websocket = require('websocket')

local clients = {}  -- Store connected clients

local server = websocket.newServer()

-- Handle WebSocket connections
server:handle("/chat", function(conn)
    -- Add client to list
    table.insert(clients, conn)
    local clientId = #clients
    print("Client " .. clientId .. " connected")
    
    -- Send welcome message to new client
    conn:send("Welcome to the chat! You are client " .. clientId)
    
    -- Broadcast to all other clients
    for i, client in ipairs(clients) do
        if client ~= conn then
            client:send("Client " .. clientId .. " joined the chat")
        end
    end
    
    -- Handle incoming messages
    conn:onMessage(function(message)
        local msg = "Client " .. clientId .. ": " .. message.data
        print(msg)
        
        -- Broadcast message to all clients
        for i, client in ipairs(clients) do
            client:send(msg)
        end
    end)
    
    -- Handle client disconnect
    conn:onClose(function()
        -- Remove client from list
        for i, client in ipairs(clients) do
            if client == conn then
                table.remove(clients, i)
                break
            end
        end
        
        print("Client " .. clientId .. " disconnected")
        
        -- Notify other clients
        for i, client in ipairs(clients) do
            client:send("Client " .. clientId .. " left the chat")
        end
    end)
    
    conn:onError(function(err)
        print("Client " .. clientId .. " error:", err)
    end)
end)

-- Start the chat server
server:listen(8080)
print("Chat server running at ws://localhost:8080/chat")
print("Connect multiple WebSocket clients to test")

-- Keep server running
while true do
    os.execute("sleep 1")
end
```

**Test the chat server:**
```bash
# Run the server
./hype run chat-server.lua

# In another terminal, test with a simple client
echo 'local websocket = require("websocket")
local client = websocket.connect("ws://localhost:8080/chat")
client:onMessage(function(msg) print("Received:", msg.data) end)
client:send("Hello from client!")
os.execute("sleep 5")
client:close()' > chat-client.lua

./hype run chat-client.lua
```

### Plain Text Browser

```lua
local http = require('http')
local app = tui.newApp()
local flex = tui.newFlex()
local addressBar = tui.newInputField()
local contentView = tui.newTextView("")
local statusBar = tui.newTextView("Ready - Enter URL and press Enter")

addressBar:SetLabel("URL: ")
addressBar:SetPlaceholder("https://example.com")

local function loadPage(url)
    statusBar:SetText("Loading " .. url .. "...")
    local response, err = http.get(url)
    
    if err then
        contentView:SetText("Error: " .. err)
        statusBar:SetText("Error loading page")
    else
        -- Strip HTML tags for plain text display
        local content = response.body:gsub("<[^>]*>", "")
        contentView:SetText(content)
        statusBar:SetText("Loaded " .. url .. " (Status: " .. response.status .. ")")
    end
end

addressBar:SetDoneFunc(function(key)
    if key == 13 then -- Enter key
        local url = addressBar:GetText()
        if url ~= "" then
            loadPage(url)
        end
    end
end)

flex:SetDirection(1)
flex:AddItem(addressBar, 1, 0, true)
flex:AddItem(contentView, 0, 1, false)
flex:AddItem(statusBar, 1, 0, false)

app:SetRoot(flex, true)
app:Run()
```

## Database Operations

### Basic CRUD Operations

```lua
local kv = require('kv')
local db = kv.open("./app.db")

-- Create bucket
db:open_db("products")

-- Create
db:put("products", "prod1", "Laptop Computer")

-- Read
local product, err = db:get("products", "prod1")
if product then
    print("Product:", product)
end

-- Update
db:put("products", "prod1", "Gaming Laptop")

-- Delete
db:delete("products", "prod1")

db:close()
```

### Advanced Querying

```lua
-- List all keys
local keys, err = db:keys("products")
for i = 1, #keys do
    print("Key:", keys[i])
end

-- Prefix search
local adminProducts = db:keys("products", "admin:")

-- Iterate with callback
db:foreach("products", function(key, value)
    print(key .. " = " .. value)
    
    -- Early termination
    if key == "stop_here" then
        return false
    end
    
    return true -- continue
end)
```

### Transactions

```lua
-- Begin transaction
local txn, err = db:begin_txn(false) -- false = read-write
if err then
    error("Failed to start transaction: " .. err)
end

-- Perform operations
txn:put("users", "batch1", "User One")
txn:put("users", "batch2", "User Two")
txn:put("users", "batch3", "User Three")

-- Commit or rollback
local err = txn:commit()
if err then
    print("Transaction failed:", err)
    txn:abort()
else
    print("Transaction committed successfully")
end
```

## Building and Testing

```bash
# Build the hype tool
go build -o hype .

# Test with example scripts using run (faster for development)
./hype run examples/hello.lua
./hype run examples/kv-test.lua
./hype run examples/browser.lua
./hype run examples/webserver.lua
./hype run examples/websocket-server-simple.lua
./hype run examples/static-server.lua -- --port 3000
./hype run examples/showcase.lua

# Or build standalone executables
./hype build examples/hello.lua -o hello && ./hello
./hype build examples/kv-test.lua -o kv-test && ./kv-test
./hype build examples/browser.lua -o browser && ./browser
./hype build examples/webserver.lua -o webserver && ./webserver
./hype build examples/websocket-server-simple.lua -o websocket-server && ./websocket-server
./hype build examples/showcase.lua -o showcase && ./showcase
```

## Cross-Platform Builds

Hype supports cross-compilation for multiple platforms and architectures:

### Basic Cross-Platform Building
```bash
# Build for Linux
./hype build myapp.lua -t linux -o myapp-linux

# Build for Windows  
./hype build myapp.lua -t windows -o myapp-windows

# Build for macOS
./hype build myapp.lua -t darwin -o myapp-macos
```

### Advanced Cross-Compilation (v1.7.1+)
Use GOOS/GOARCH environment variables for precise platform targeting:

```bash
# Linux x86_64
GOOS=linux GOARCH=amd64 ./hype build myapp.lua -o myapp-linux-amd64

# Linux ARM64 (Raspberry Pi, etc.)
GOOS=linux GOARCH=arm64 ./hype build myapp.lua -o myapp-linux-arm64

# macOS Intel
GOOS=darwin GOARCH=amd64 ./hype build myapp.lua -o myapp-macos-intel

# macOS Apple Silicon
GOOS=darwin GOARCH=arm64 ./hype build myapp.lua -o myapp-macos-arm64

# Windows x86_64
GOOS=windows GOARCH=amd64 ./hype build myapp.lua -o myapp-windows.exe
```

### GitHub Actions / CI/CD
Perfect for automated builds:

```yaml
- name: Build Cross-Platform Binaries
  run: |
    GOOS=linux GOARCH=amd64 ./hype build myapp.lua -o myapp-linux-amd64
    GOOS=linux GOARCH=arm64 ./hype build myapp.lua -o myapp-linux-arm64
    GOOS=darwin GOARCH=amd64 ./hype build myapp.lua -o myapp-macos-intel
    GOOS=darwin GOARCH=arm64 ./hype build myapp.lua -o myapp-macos-arm64
    GOOS=windows GOARCH=amd64 ./hype build myapp.lua -o myapp-windows.exe
```

**Supported Platforms:**
- Linux: amd64, arm64, 386, arm
- macOS: amd64, arm64
- Windows: amd64, 386
- FreeBSD: amd64, arm64

## Use Cases

- 🖥️ **Terminal Applications**: Interactive CLI tools, system monitors, development utilities
- 🌐 **Web Applications**: REST APIs, web servers, microservices  
- 🔌 **Real-Time Applications**: Chat servers, live dashboards, streaming services, multiplayer games
- 🗄️ **Data Tools**: Database utilities, data processing scripts, ETL tools
- 📦 **Distributed Software**: Single-binary deployments, embedded systems
- 🔧 **DevOps Tools**: Build scripts, deployment automation, monitoring tools

## Why Hype?

- **Zero Dependencies**: No external libraries or runtimes needed
- **Small Binaries**: Efficient packaging with reasonable file sizes  
- **Cross-Platform**: Write once, run anywhere
- **Rich APIs**: TUI, HTTP, and database support built-in
- **Easy Deployment**: Single executable file distribution
- **Lua Simplicity**: Clean, readable scripting language
- **Go Performance**: Fast startup and execution times

## Contributing

Contributions are welcome! Please feel free to submit a Pull Request.

## License

MIT License - see [LICENSE](LICENSE) file for details.