# Changelog

All notable changes to Hype will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

## [1.11.0] - 2025-08-23

### Changed
- **📚 Simplified README**: Restructured documentation to focus on Hype as a modern Lua runtime
  - Runtime-first approach with REPL, Run, Build progression
  - Reduced from 1100+ lines to ~220 lines for clarity
  - Cleaner structure: Getting Started → Usage → Modules → Plugins
  - Positioned as "Modern Lua Runtime" instead of "Packager"

### Added
- **📝 Development Files**: Added Claude development files to .gitignore
  - PRPs (Project Request Protocols) directory
  - Claude command configurations
  - Development documentation files

### Improved
- **🎯 Documentation Focus**: Clear emphasis on Hype's core mission as a Lua runtime
- **🚀 Getting Started Experience**: Streamlined installation and quick start guides
- **🔌 Plugin Documentation**: Simplified examples for both Lua and Go plugins

## [1.10.0] - Previous Release

## [1.7.4] - 2025-07-24

### Added
- **🔐 4096-bit RSA Key Support**: Enhanced `crypto.generate_jwk()` to accept optional key size parameter
  - Supported sizes: 2048, 3072, and 4096 bits
  - Works with all RSA algorithms (RS256/384/512, PS256/384/512)
  - Maintains backward compatibility with default key sizes

### Enhanced
- **📚 Documentation**: Updated API docs and examples to showcase 4096-bit key usage
- **🧪 Examples**: Added 4096-bit RSA key demonstration to crypto-basic.lua

### Technical
- Modified both `crypto_functions.go` and `builder.go` for consistent key size support
- Added validation for supported key sizes with clear error messages
- Thoroughly tested PS512 with 4096-bit keys for maximum security

## [1.7.3] - 2025-07-24

### Added
- **🔐 SHA-256/384/512 Hashing**: New cryptographic hash functions in crypto module
  - `crypto.sha256(data)` - SHA-256 hashing
  - `crypto.sha384(data)` - SHA-384 hashing
  - `crypto.sha512(data)` - SHA-512 hashing
  - `crypto.hash(algorithm, data)` - Generic hash function
  - `crypto.deep_hash(value, [algorithm])` - Deep hash for complex data structures

### Enhanced
- **📚 Documentation**: Updated crypto module docs with hashing examples
- **🧪 Examples**: Enhanced crypto-basic.lua with hashing demonstrations

## [1.7.2] - 2025-07-24

### Added
- **🔐 RSA-PSS Support**: Added PS256, PS384, PS512 algorithms to crypto module
  - Full RSA-PSS (Probabilistic Signature Scheme) implementation
  - Randomized signatures for enhanced security
  - Compatible with existing JWK infrastructure

### Fixed
- **🔧 Crypto Module**: Fixed RSA JWK compatibility with hbwallet-generated keys

## [1.7.1] - 2025-07-18

### Fixed
- **🔧 Build System**: Fixed unused `reflect` import issue when building scripts without plugins
- **🌍 Cross-compilation**: Enhanced `hype build` to properly respect GOOS/GOARCH environment variables
- **⚙️ CI/CD Support**: Improved compatibility with GitHub Actions and automated build systems

### Added
- **📖 Documentation**: Enhanced README with detailed cross-compilation examples
- **🔗 Examples**: Added LMDB Tree Viewer as showcase project demonstrating cross-platform builds
- **🛠️ GitHub Actions**: Templates and examples for automated multi-platform releases

### Technical
- Template engine now conditionally includes `reflect` import only when plugins are present
- Build process respects external GOOS/GOARCH environment variables for precise platform targeting
- Improved error messages and debugging output for build failures

## [1.5.0] - 2025-07-09

### Added
- **🔌 Plugin System**: Complete plugin system with versioned Lua modules
  - Support for `name@version` syntax (e.g., `fs@1.0.0`)
  - Automatic plugin discovery in conventional directories
  - Version validation and exact version matching
  - Multiple plugin loading with custom aliases
  - Plugin embedding in built executables (zero-dependency deployment)
  - Plugin configuration files with YAML support
- **📚 Comprehensive Plugin Documentation**:
  - Complete plugin development guide with step-by-step tutorials
  - Plugin system reference with API documentation
  - Real-world plugin examples (filesystem, JSON, HTTP utilities)
  - Best practices and testing strategies
- **🛠️ CLI Plugin Integration**:
  - `--plugins` flag for both `run` and `build` commands
  - `--plugins-config` for YAML configuration files
  - Enhanced help text with plugin usage examples
- **📦 Example Plugins**:
  - Filesystem plugin (v1.0.0) with basic file operations
  - Enhanced filesystem plugin (v2.0.0) with copy, move, delete
  - Comprehensive test scripts for plugin validation

### Enhanced
- **CLI Interface**: Updated help text and examples for plugin usage
- **Documentation**: Added plugin system to README and created detailed guides
- **Build System**: Plugin embedding support for standalone executables
- **Development Workflow**: Plugin discovery and loading for rapid iteration

### Plugin Specification Formats
- `fs` - Simple name with auto-discovery
- `fs@1.0.0` - Name with specific version requirement
- `myfs=./path/to/plugin` - Custom alias with explicit path
- `myfs=./path/to/plugin@2.0.0` - Alias with path and version
- `github.com/user/plugin@v1.0.0` - Go module support (framework ready)

## [1.3.1] - 2024-06-24

### Fixed
- HTTP signatures build error when creating standalone executables
- HTML injection issue in examples page that rendered code as HTML
- Examples page layout inconsistencies with homepage and API pages

### Added
- Complete HTTP signatures implementation in runtime template for standalone executables
- Professional code viewer using Prism.js for syntax highlighting
- Direct links to GitHub examples repository in documentation

### Changed
- Refactored examples page to showcase actual GitHub examples instead of embedded code
- Improved documentation website styling consistency across all pages
- Enhanced mobile responsiveness and touch-friendly navigation

## [1.3.0] - 2024-06-24

### Added
- Release automation and checklist process
- Comprehensive mobile-friendly documentation website  
- Version verification in install scripts

### Changed
- Improved release process with automated scripts
- Enhanced mobile responsiveness across all documentation pages

## [1.2.0] - 2024-06-24

### Added
- **HTTP Signatures module** with RFC-compliant signing and verification
- **Crypto module** with JWK support for RSA, ECDSA, and Ed25519 algorithms
- **Cyberpunk documentation styling** with tron-like blue themes and dark gray text
- **Comprehensive examples** for crypto operations and HTTP signatures
- **Complete API reference** with practical use cases
- **Mobile-optimized documentation** with touch-friendly interfaces
- **Enhanced responsive design** for all screen sizes
- **Version command** now shows proper version information

### Changed
- **Command structure**: `hype eval` renamed to `hype run` for better clarity
- **Documentation website** completely redesigned with cyberpunk aesthetic
- **Code block positioning** optimized to prevent UI collisions
- **Navigation** improved for mobile devices with touch-friendly targets

### Fixed
- **Critical security vulnerability** in HTTP signature verification
- **Digest validation** now properly prevents tampered content acceptance
- **Mobile navigation** and touch target sizing issues
- **Cross-platform binary** building process improvements

### Security
- **HTTP signature tampering protection** through proper digest validation
- **Enhanced message integrity** protection for API communications
- **Multi-algorithm support** for cryptographic operations (RS256, ES256, EdDSA)

## [1.1.0] - 2024-06-XX

### Added
- Enhanced TUI features and dashboard examples
- Comprehensive TUI method support
- Advanced terminal user interface capabilities

### Changed
- Updated macOS install script process
- Improved documentation website structure

## [1.0.5] - 2024-06-XX

### Added
- Improved HTTP server performance and reliability
- Critical HTTP and TUI component fixes

## [1.0.4] - 2024-06-XX

### Added
- Comprehensive documentation website
- Improved macOS installation experience

### Fixed
- Critical HTTP and TUI component issues

## [1.0.0] - 2024-XX-XX

### Added
- Initial release of Hype
- **TUI (Terminal User Interface)** support with tview library
- **HTTP client and server** functionality
- **Embedded key-value database** with BoltDB
- **Cross-platform executable** building (Linux, macOS, Windows)
- **Lua script packaging** into standalone executables
- **Zero external dependencies** in final executables
- **Development mode** with direct script execution
- **Command line argument** support for packaged applications

### Features
- Package Lua scripts into standalone executables
- Built-in TUI library for creating terminal applications
- HTTP client and server support for web applications
- Embedded key-value database with BoltDB
- Transaction support with ACID properties
- Database iteration and querying with cursor support
- Simple deployment with single binary distribution
- Cross-platform support (Linux, macOS, Windows)

---

## Release Notes Format

Each release should include:

### Added
- New features and capabilities

### Changed
- Changes in existing functionality

### Deprecated
- Soon-to-be removed features

### Removed
- Features removed in this release

### Fixed
- Bug fixes

### Security
- Security improvements and vulnerability fixes

---

## Versioning Strategy

- **MAJOR**: Incompatible API changes
- **MINOR**: Backward-compatible functionality additions
- **PATCH**: Backward-compatible bug fixes

## Links

- [GitHub Releases](https://github.com/twilson63/hype/releases)
- [Documentation](https://twilson63.github.io/hype/)
- [Issue Tracker](https://github.com/twilson63/hype/issues)