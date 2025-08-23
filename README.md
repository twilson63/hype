# Hype - Modern Lua Runtime

Hype is a Lua runtime with built-in modules for HTTP, WebSocket, TUI, cryptography, and embedded databases. Write your application in Lua and deploy it as a single executable.

## Getting Started

### Install

**macOS/Linux:**
```bash
curl -sSL https://raw.githubusercontent.com/twilson63/hype/main/install.sh | bash
```

**From source:**
```bash
git clone https://github.com/twilson63/hype.git
cd hype
go build -o hype .
```

**Download binaries:** [GitHub Releases](https://github.com/twilson63/hype/releases)

## Usage

### REPL - Interactive Lua

Start an interactive session to explore Hype's modules:

```bash
hype repl              # TUI REPL with syntax highlighting
hype repl --simple     # Simple command-line REPL
```

```lua
> crypto.sha256("hello")
2cf24dba5fb0a30e26e83b2ac5b9e29e1b161e5c1fa7425e73043362938b9824

> http.get("https://api.github.com")
{status = 200, body = "...", headers = {...}}
```

### Run - Development Mode

Execute Lua scripts directly with all built-in modules available:

```bash
hype run script.lua
hype run server.lua -- --port 8080
```

**hello.lua:**
```lua
local http = require('http')
local server = http.newServer()

server:handle("/", function(req, res)
    res:json({ message = "Hello from Hype!" })
end)

print("Server running at http://localhost:8080")
server:listen(8080)
```

### Build - Create Executables

Package your Lua application into a standalone binary:

```bash
hype build script.lua -o myapp

# Cross-platform builds
GOOS=linux GOARCH=amd64 hype build script.lua -o myapp-linux
GOOS=windows GOARCH=amd64 hype build script.lua -o myapp.exe
GOOS=darwin GOARCH=arm64 hype build script.lua -o myapp-mac
```

## Built-in Modules

Hype includes powerful modules accessible via `require()`:

- **`http`** - HTTP client/server with routing and JSON support
- **`websocket`** - WebSocket client/server for real-time communication
- **`kv`** - Embedded key-value database (BoltDB)
- **`crypto`** - Cryptography with JWK, signatures, and hashing
- **`tui`** - Terminal UI components for interactive applications

**Quick example using multiple modules:**
```lua
local http = require('http')
local kv = require('kv')
local crypto = require('crypto')

-- Open database
local db = kv.open("./data.db")
db:open_db("users")

-- Create API server
local server = http.newServer()

server:handle("/api/user", function(req, res)
    if req.method == "POST" then
        local id = crypto.sha256(req.body)
        db:put("users", id, req.body)
        res:json({ id = id, status = "created" })
    end
end)

server:listen(8080)
```

## Plugins

Extend Hype with custom modules written in Lua or Go.

### Using Plugins

```bash
# Run with plugins
hype run app.lua --plugins fs,json

# Build with embedded plugins
hype build app.lua --plugins fs@1.0.0 -o myapp

# Use specific versions
hype run app.lua --plugins fs@2.0.0,utils@1.5.0
```

### Lua Plugin

Create a plugin directory with manifest and code:

**myplugin/hype-plugin.yaml:**
```yaml
name: "myplugin"
version: "1.0.0"
type: "lua"
main: "plugin.lua"
```

**myplugin/plugin.lua:**
```lua
local M = {}

function M.greet(name)
    return "Hello, " .. (name or "World") .. "!"
end

return M
```

**Use in your script:**
```lua
local myplugin = require("myplugin")
print(myplugin.greet("Hype"))  -- "Hello, Hype!"
```

### Go Plugin

For performance-critical operations, create Go plugins:

**myplugin/plugin.go:**
```go
package main

import "github.com/yuin/gopher-lua"

func Hello(L *lua.LState) int {
    name := L.OptString(1, "World")
    L.Push(lua.LString("Hello, " + name + "!"))
    return 1
}

func Export(L *lua.LState) lua.LGFunction {
    return func(L *lua.LState) int {
        mod := L.NewTable()
        L.SetField(mod, "hello", L.NewFunction(Hello))
        L.Push(mod)
        return 1
    }
}
```

Build and use:
```bash
go build -buildmode=plugin -o myplugin.so myplugin/plugin.go
hype run app.lua --plugins myplugin=./myplugin.so
```

## Examples

```bash
# Interactive REPL exploration
hype repl

# Run examples
hype run examples/hello.lua
hype run examples/webserver.lua
hype run examples/websocket-chat.lua

# Build for production
hype build myapp.lua -o myapp
./myapp --port 8080
```

## Documentation

- **[API Reference](https://twilson63.github.io/hype/api-enhanced.html)** - Complete module documentation
- **[Plugin Development](docs/PLUGINS.md)** - Creating custom plugins
- **[Examples](examples/)** - Sample applications
- **[CLAUDE.md](CLAUDE.md)** - AI assistant instructions

## Platform Support

- **Linux**: amd64, arm64, arm, 386
- **macOS**: amd64 (Intel), arm64 (Apple Silicon)  
- **Windows**: amd64, 386
- **FreeBSD**: amd64, arm64

## License

MIT - See [LICENSE](LICENSE) file