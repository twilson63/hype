# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

Hype Lean Framework is a streamlined Go-based tool that packages Lua scripts into standalone executables with HTTP client, embedded database, cryptography, and HTTP signatures support. It embeds a Lua runtime with focused modules to create cross-platform applications with zero external dependencies.

## Development Commands

### Building and Testing
```bash
# Build the main hype executable
make build

# Build development version with race detection
make dev

# Run tests
make test

# Clean build artifacts
make clean

# Build releases for all platforms
make releases
```

### Running Scripts
```bash
# Run Lua scripts directly (recommended for development)
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
```

### Release Management
```bash
# Pre-release validation
make pre-release-check

# Create a release (interactive)
make release

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

**eval.go**: Direct script execution
- Sets up Lua state with all modules
- Handles plugin loading
- Manages script arguments

**plugin.go**: Plugin system implementation
- Supports Lua and Go plugins
- Version management
- Dynamic loading and registration

**http_client.go**: HTTP client implementation
- Support for all HTTP methods
- JSON response parsing
- Headers and timeout configuration

### Lua Module System

Built-in modules accessible via `require()`:
- **http**: HTTP client only (GET, POST, PUT, DELETE with headers/timeouts)
- **kv**: BoltDB-based key-value store with transactions
- **crypto**: Cryptography with JWK support (RSA, ECDSA, Ed25519, SHA hashing)
- **httpsig**: HTTP signatures for request signing and verification

### Plugin System

Plugins extend functionality with custom Lua modules:
- Discovery in `./plugins/`, `./examples/plugins/` directories
- Manifest-based (`hype-plugin.yaml`)
- Version management with semver
- Can be embedded into built executables

## Code Patterns

### Lua-Go Bridge
All modules use consistent userdata/metatable patterns:
```go
// Go struct wrapped in Lua userdata
// Method dispatch through __index metamethods
// Consistent error handling: nil + error string returns
```

### Module Registration
```go
L.PreloadModule("modulename", func(L *lua.LState) int {
    // Create module table
    // Register functions
    // Return module
})
```

## Testing

Test scripts with `./hype run script.lua` before building. Example scripts in `examples/` demonstrate all features and serve as integration tests.

## Dependencies

Go 1.23+ with key modules:
- `github.com/spf13/cobra` - CLI framework
- `github.com/yuin/gopher-lua` - Lua runtime
- `go.etcd.io/bbolt` - Embedded database
- `gopkg.in/yaml.v2` - YAML parsing for plugins

## Platform Support

Cross-compilation targets:
- Linux (amd64, arm64, arm)
- macOS/Darwin (amd64, arm64)
- Windows (amd64)

Platform notes:
- macOS may require: `xattr -d com.apple.quarantine /path/to/hype`
- Windows executables get `.exe` extension automatically
- All platforms produce single-binary deployments