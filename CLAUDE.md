# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

Hype is a Go-based tool that packages Lua scripts into standalone executables with built-in modules for HTTP client/server, WebSocket, TUI, embedded database (BoltDB), and cryptography. It creates cross-platform applications with zero external dependencies by embedding the Lua runtime and scripts into a single binary.

## Development Commands

### Building and Testing
```bash
# Build the main hype executable
make build

# Build development version with race detection
make dev

# Run tests
make test
go test ./...

# Run a single test
go test -run TestBuildExecutable

# Clean build artifacts
make clean

# Build releases for all platforms
make releases
```

### Core Commands
```bash
# Run Lua scripts directly (development mode)
./hype run script.lua
./hype run script.lua -- --arg1 value1 --arg2 value2

# Build standalone executables
./hype build script.lua -o output_name
./hype build script.lua -t windows -o myapp-windows.exe

# Cross-compilation with GOOS/GOARCH
GOOS=linux GOARCH=amd64 ./hype build script.lua -o myapp-linux-amd64
GOOS=darwin GOARCH=arm64 ./hype build script.lua -o myapp-macos-arm64
GOOS=windows GOARCH=amd64 ./hype build script.lua -o myapp-windows.exe

# Interactive REPL
./hype repl              # TUI REPL with visual interface
./hype repl --simple     # Simple command-line REPL

# Bundle multi-file projects (optional, build handles this automatically)
./hype bundle main.lua -o bundled.lua

# Plugin usage
./hype run script.lua --plugins fs@1.0.0
./hype build script.lua --plugins fs,lmdb -o app
```

### Release Management
```bash
# Pre-release validation
make pre-release-check
./scripts/pre-release-check.sh

# Create a release (interactive)
make release
./scripts/release.sh

# View version information
make version
```

## Architecture

### Core Components

**main.go**: CLI entry point using Cobra framework
- `build` command: Packages Lua scripts into executables
- `run` command: Executes Lua scripts directly  
- `repl` command: Interactive Lua REPL (TUI or simple mode)
- `bundle` command: Bundles multi-file projects
- `version` command: Shows version information

**builder.go**: Executable generation system
- Creates temporary Go runtime embedding the Lua script
- Generates complete Go application with all dependencies
- Cross-compiles for different platforms
- Uses Go's template system to inject Lua scripts
- Handles plugin embedding

**eval.go**: Direct script execution
- Sets up Lua state with all modules
- Handles plugin loading and registration
- Manages script arguments via global `arg` table

**bundle.go**: Multi-file bundling
- Resolves `require()` dependencies
- Merges multiple Lua files into single output
- Preserves module boundaries

**plugin.go**: Plugin system
- Discovers plugins in conventional locations
- Supports Lua and Go plugins (.so files)
- Version management with semver
- Dynamic loading and registration

**repl.go**: Interactive REPL
- Simple mode for basic CLI
- Command history and recall
- Pretty table formatting

**http_client.go**: HTTP client module
- All HTTP methods (GET, POST, PUT, DELETE, etc.)
- JSON response parsing
- Headers and timeout configuration

### Lua Module System

Built-in modules accessible via `require()`:
- **http**: HTTP client (all methods, JSON support, routing)
- **kv**: BoltDB-based key-value store with transactions and cursors
- **crypto**: Cryptography with JWK support (RSA/PSS, ECDSA, Ed25519, SHA hashing)
- **httpsig**: HTTP signatures for request signing and verification

### Plugin System

Plugins extend functionality with custom modules:
- Discovery locations: `./plugins/`, `./examples/plugins/`, `./<name>-plugin/`
- Manifest-based (`hype-plugin.yaml`) with name, version, type, main
- Lua plugins: Return module table from plugin.lua
- Go plugins: Compiled .so files with Export() function
- Version management with semver constraints
- Embedded into executables via --plugins flag

## Code Patterns

### Lua-Go Bridge
All modules use consistent userdata/metatable patterns:
- Go structs wrapped in Lua userdata
- Method dispatch through `__index` metamethods
- Error handling: return `nil, error_string` on failure
- Success: return value(s) without error

### Module Registration
```go
L.PreloadModule("modulename", func(L *lua.LState) int {
    mod := L.NewTable()
    L.SetFuncs(mod, map[string]lua.LGFunction{
        "function": luaFunction,
    })
    L.Push(mod)
    return 1
})
```

### Userdata Pattern
```go
const luaTypeNameTypeName = "TypeName"

func checkType(L *lua.LState, n int) *GoType {
    ud := L.CheckUserData(n)
    if v, ok := ud.Value.(*GoType); ok {
        return v
    }
    L.ArgError(n, "TypeName expected")
    return nil
}

func pushType(L *lua.LState, t *GoType) {
    ud := L.NewUserData()
    ud.Value = t
    L.SetMetatable(ud, L.GetTypeMetatable(luaTypeNameTypeName))
    L.Push(ud)
}
```

## Testing

### Unit Tests
```bash
# Run all tests
make test
go test ./...

# Run specific test
go test -run TestBuildExecutable

# Test with race detection
go test -race ./...
```

### Integration Testing
Example scripts in `examples/` serve as integration tests:
```bash
# Test basic functionality
./hype run examples/hello.lua
./hype run examples/kv-test.lua
./hype run examples/webserver.lua
./hype run examples/crypto-basic.lua

# Test plugins
./hype run examples/test-fs-plugin.lua --plugins fs@1.0.0

# Test building
./hype build examples/hello.lua -o test-hello
./test-hello
```

## Dependencies

Go 1.23+ (toolchain 1.24.3) with modules:
- `github.com/spf13/cobra@v1.8.1` - CLI framework
- `github.com/yuin/gopher-lua@v1.1.1` - Lua runtime
- `go.etcd.io/bbolt@v1.4.1` - Embedded database
- `gopkg.in/yaml.v2@v2.4.0` - YAML parsing for plugins

## Platform Support

Cross-compilation targets:
- Linux (amd64, arm64, arm, 386)
- macOS/Darwin (amd64, arm64)
- Windows (amd64, 386)
- FreeBSD (amd64, arm64)

Platform notes:
- macOS may require: `xattr -d com.apple.quarantine /path/to/hype`
- Windows executables get `.exe` extension automatically
- All platforms produce single-binary deployments
- Use GOOS/GOARCH environment variables for precise targeting
