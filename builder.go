package main

import (
	"context"
	"crypto/sha256"
	"crypto/sha512"
	"encoding/base64"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"text/template"
	"time"

	"github.com/yuin/gopher-lua"
)

// Removed embed for now - we'll generate everything at build time

type BuildConfig struct {
	ScriptPath               string
	OutputName               string
	Target                   string
	ScriptContent            string
	PluginSpecs              []PluginSpec
	PluginRegistry           *PluginRegistry
	PluginRegistrationCode   string
	PluginImports            string
	PluginDependencies       []string
	PluginSourceFiles        []string
	HasPlugins               bool
}


func buildExecutable(scriptPath, outputName, target string) error {
	return buildExecutableWithPlugins(scriptPath, outputName, target, []PluginSpec{})
}

func buildExecutableWithPlugins(scriptPath, outputName, target string, pluginSpecs []PluginSpec) error {
	config := &BuildConfig{
		ScriptPath:  scriptPath,
		OutputName:  outputName,
		Target:      target,
		PluginSpecs: pluginSpecs,
	}

	if config.OutputName == "" {
		base := filepath.Base(scriptPath)
		config.OutputName = strings.TrimSuffix(base, filepath.Ext(base))
	}

	if config.Target == "current" {
		config.Target = runtime.GOOS
	}

	// Load plugins first if specified
	var availableModules map[string]bool
	if len(config.PluginSpecs) > 0 {
		config.PluginRegistry = NewPluginRegistry()
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
		defer cancel()
		
		if err := config.PluginRegistry.LoadPlugins(ctx, config.PluginSpecs); err != nil {
			return fmt.Errorf("failed to load plugins: %w", err)
		}
		defer config.PluginRegistry.Close()
		
		// Generate plugin registration code
		if err := generatePluginCode(config); err != nil {
			return fmt.Errorf("failed to generate plugin code: %w", err)
		}
		
		config.HasPlugins = true
		
		// Create a map of available modules from plugins
		availableModules = make(map[string]bool)
		for _, plugin := range config.PluginRegistry.plugins {
			availableModules[plugin.Name()] = true
		}
	} else {
		config.PluginRegistrationCode = ""
		config.PluginImports = ""
		config.PluginDependencies = []string{}
		config.HasPlugins = false
		availableModules = make(map[string]bool)
	}

	// Auto-bundle dependencies if they exist, taking into account plugin modules
	bundledContent, err := resolveDependenciesWithModules(scriptPath, make(map[string]bool), availableModules)
	if err != nil {
		return fmt.Errorf("failed to resolve dependencies: %w", err)
	}
	// Escape the script content for safe embedding in Go code
	config.ScriptContent = strconv.Quote(bundledContent)

	tempDir, err := os.MkdirTemp("", "luax-build-*")
	if err != nil {
		return fmt.Errorf("failed to create temp directory: %w", err)
	}
	defer os.RemoveAll(tempDir)

	if err := generateRuntimeCode(tempDir, config); err != nil {
		return fmt.Errorf("failed to generate runtime code: %w", err)
	}
	
	// Copy plugin source files to build directory
	if err := copyPluginSourceFiles(tempDir, config); err != nil {
		return fmt.Errorf("failed to copy plugin source files: %w", err)
	}

	if err := buildExecutableFromRuntime(tempDir, config); err != nil {
		return fmt.Errorf("failed to build executable: %w", err)
	}

	return nil
}

func generatePluginCode(config *BuildConfig) error {
	if config.PluginRegistry == nil {
		return nil
	}
	
	var registrationCode strings.Builder
	var deps []string
	var pluginSourceFiles []string
	
	// For now, we'll embed Lua plugins as string constants and register them dynamically
	// For Go plugins, we'll embed the source code directly
	
	registrationCode.WriteString("\t// Register plugin modules\n")
	
	for _, plugin := range config.PluginRegistry.plugins {
		// For Lua plugins, we can use direct registration
		if wrapper, ok := plugin.(*LuaPluginWrapper); ok {
			pluginName := plugin.Name()
			registrationCode.WriteString(fmt.Sprintf("\t// Register %s plugin\n", pluginName))
			registrationCode.WriteString(fmt.Sprintf("\tL.PreloadModule(\"%s\", func(L *lua.LState) int {\n", pluginName))
			registrationCode.WriteString(fmt.Sprintf("\t\tpluginCode := `%s`\n", strings.ReplaceAll(wrapper.content, "`", "` + \"`\" + `")))
			registrationCode.WriteString("\t\ttempL := lua.NewState()\n")
			registrationCode.WriteString("\t\tdefer tempL.Close()\n")
			registrationCode.WriteString("\t\tif err := tempL.DoString(pluginCode); err != nil {\n")
			registrationCode.WriteString("\t\t\tfmt.Fprintf(os.Stderr, \"Error loading plugin: %v\\n\", err)\n")
			registrationCode.WriteString("\t\t\tos.Exit(1)\n")
			registrationCode.WriteString("\t\t}\n")
			registrationCode.WriteString("\t\tpluginTable := tempL.Get(-1)\n")
			registrationCode.WriteString("\t\tif pluginTable.Type() == lua.LTTable {\n")
			registrationCode.WriteString("\t\t\tnewTable := L.NewTable()\n")
			registrationCode.WriteString("\t\t\tpluginTable.(*lua.LTable).ForEach(func(key, value lua.LValue) {\n")
			registrationCode.WriteString("\t\t\t\tL.SetField(newTable, key.String(), value)\n")
			registrationCode.WriteString("\t\t\t})\n")
			registrationCode.WriteString("\t\t\tL.Push(newTable)\n")
			registrationCode.WriteString("\t\t} else {\n")
			registrationCode.WriteString("\t\t\tL.Push(lua.LNil)\n")
			registrationCode.WriteString("\t\t}\n")
			registrationCode.WriteString("\t\treturn 1\n")
			registrationCode.WriteString("\t})\n")
		} else if _, ok := plugin.(*GoPluginWrapper); ok {
			// For Go plugins, we need to embed the plugin code directly
			pluginName := plugin.Name()
			
			// Find the plugin source file
			pluginSourcePath, err := findPluginSourceFile(config.PluginSpecs, pluginName)
			if err != nil {
				return fmt.Errorf("failed to find plugin source for %s: %w", pluginName, err)
			}
			
			pluginSourceFiles = append(pluginSourceFiles, pluginSourcePath)
			
			registrationCode.WriteString(fmt.Sprintf("\t// Register %s Go plugin\n", pluginName))
			registrationCode.WriteString(fmt.Sprintf("\t%sPluginInstance := NewPlugin()\n", pluginName))
			registrationCode.WriteString(fmt.Sprintf("\t// Use reflection to call Register method\n"))
			registrationCode.WriteString(fmt.Sprintf("\tpluginValue := reflect.ValueOf(%sPluginInstance)\n", pluginName))
			registrationCode.WriteString(fmt.Sprintf("\tregisterMethod := pluginValue.MethodByName(\"Register\")\n"))
			registrationCode.WriteString(fmt.Sprintf("\tif registerMethod.IsValid() {\n"))
			registrationCode.WriteString(fmt.Sprintf("\t\tresults := registerMethod.Call([]reflect.Value{reflect.ValueOf(L)})\n"))
			registrationCode.WriteString(fmt.Sprintf("\t\tif len(results) > 0 && !results[0].IsNil() {\n"))
			registrationCode.WriteString(fmt.Sprintf("\t\t\tif err, ok := results[0].Interface().(error); ok {\n"))
			registrationCode.WriteString(fmt.Sprintf("\t\t\t\tfmt.Fprintf(os.Stderr, \"Error registering plugin %s: %%v\\n\", err)\n", pluginName))
			registrationCode.WriteString(fmt.Sprintf("\t\t\t\tos.Exit(1)\n"))
			registrationCode.WriteString(fmt.Sprintf("\t\t\t}\n"))
			registrationCode.WriteString(fmt.Sprintf("\t\t}\n"))
			registrationCode.WriteString(fmt.Sprintf("\t}\n"))
		}
		deps = append(deps, plugin.Dependencies()...)
	}
	
	config.PluginRegistrationCode = registrationCode.String()
	config.PluginDependencies = deps
	config.PluginSourceFiles = pluginSourceFiles
	
	return nil
}

// findPluginSourceFile finds the source file for a plugin
func findPluginSourceFile(pluginSpecs []PluginSpec, pluginName string) (string, error) {
	for _, spec := range pluginSpecs {
		if spec.Name == pluginName || spec.Alias == pluginName {
			// For local plugins, find the plugin.go file
			if filepath.IsAbs(spec.Source) || strings.HasPrefix(spec.Source, "./") || strings.HasPrefix(spec.Source, "../") {
				pluginGoFile := filepath.Join(spec.Source, "plugin.go")
				if _, err := os.Stat(pluginGoFile); err == nil {
					return pluginGoFile, nil
				}
			}
		}
	}
	return "", fmt.Errorf("plugin source not found for %s", pluginName)
}

// copyPluginSourceFiles copies Go plugin source files to the build directory
func copyPluginSourceFiles(tempDir string, config *BuildConfig) error {
	for _, pluginSourceFile := range config.PluginSourceFiles {
		// Read the plugin source file
		sourceContent, err := os.ReadFile(pluginSourceFile)
		if err != nil {
			return fmt.Errorf("failed to read plugin source %s: %w", pluginSourceFile, err)
		}
		
		// Get the filename
		filename := filepath.Base(pluginSourceFile)
		
		// Create a modified version that doesn't include the HypePlugin interface
		// and changes the package to main
		modifiedContent := strings.ReplaceAll(string(sourceContent), "package main", "package main\n\n// Plugin code embedded in build")
		
		// Remove the HypePlugin interface definition if present
		modifiedContent = removeHypePluginInterface(modifiedContent)
		
		// Write to the build directory
		destPath := filepath.Join(tempDir, filename)
		if err := os.WriteFile(destPath, []byte(modifiedContent), 0644); err != nil {
			return fmt.Errorf("failed to write plugin source to %s: %w", destPath, err)
		}
	}
	
	return nil
}

// removeHypePluginInterface removes the HypePlugin interface definition from plugin code
func removeHypePluginInterface(content string) string {
	// Remove the interface definition
	lines := strings.Split(content, "\n")
	var result []string
	inInterface := false
	
	for _, line := range lines {
		if strings.Contains(line, "type HypePlugin interface") {
			inInterface = true
			continue
		}
		if inInterface && strings.Contains(line, "}") {
			inInterface = false
			continue
		}
		if !inInterface {
			result = append(result, line)
		}
	}
	
	return strings.Join(result, "\n")
}

func generateRuntimeCode(tempDir string, config *BuildConfig) error {
	// First, copy the http_client.go file to the temp directory
	httpClientPath := filepath.Join(tempDir, "http_client.go")
	httpClientContent, err := os.ReadFile("http_client.go")
	if err != nil {
		return fmt.Errorf("failed to read http_client.go: %w", err)
	}
	
	// Replace package main with package main (already correct)
	if err := os.WriteFile(httpClientPath, httpClientContent, 0644); err != nil {
		return fmt.Errorf("failed to write http_client.go: %w", err)
	}
	
	runtimeTemplate := `package main

import (
	"bytes"
	"crypto"
	"crypto/ecdsa"
	"crypto/ed25519"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/sha512"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math/big"
	"os"
	{{if .HasPlugins}}"reflect"{{end}}
	"sort"
	"strconv"
	"strings"
	"time"
	"github.com/yuin/gopher-lua"
	"go.etcd.io/bbolt"
)

const luaScript = {{.ScriptContent}}

func main() {
	L := lua.NewState()
	defer L.Close()

	// Open standard libraries
	L.PreloadModule("_G", lua.OpenBase)
	L.PreloadModule("package", lua.OpenPackage)
	L.PreloadModule("coroutine", lua.OpenCoroutine)
	L.PreloadModule("table", lua.OpenTable)
	L.PreloadModule("io", lua.OpenIo)
	L.PreloadModule("os", lua.OpenOs)
	L.PreloadModule("string", lua.OpenString)
	L.PreloadModule("math", lua.OpenMath)
	L.PreloadModule("debug", lua.OpenDebug)
	
	// Set up command line arguments
	setupCommandLineArgs(L)
	
	// Register HTTP module
	RegisterHTTPModule(L)

	// Register KV module
	registerKVModule(L)

	// Register TUI functions
	registerTUIFunctions(L)

	// Register Crypto module
	registerCryptoModule(L)

	// Register HTTP Signatures module
	registerHTTPSigModule(L)

	// Register WebSocket stub
	registerWebSocketStub(L)

{{.PluginRegistrationCode}}

	if err := L.DoString(luaScript); err != nil {
		fmt.Fprintf(os.Stderr, "Error running Lua script: %v\n", err)
		os.Exit(1)
	}
}

func setupCommandLineArgs(L *lua.LState) {
	// Create arg table (following Lua convention)
	argTable := L.NewTable()
	
	// Get command line arguments from os.Args
	args := os.Args
	
	// arg[0] is the executable name (script name equivalent)
	if len(args) > 0 {
		argTable.RawSetInt(0, lua.LString(args[0]))
	}
	
	// arg[1], arg[2], etc. are the script arguments
	for i := 1; i < len(args); i++ {
		argTable.RawSetInt(i, lua.LString(args[i]))
	}
	
	// Set global arg table
	L.SetGlobal("arg", argTable)
}

func registerTUIFunctions(L *lua.LState) {
	// TUI module no longer supported - return error functions
	tuiModule := L.NewTable()
	
	// Create error functions for all TUI components
	errorFunc := L.NewFunction(func(L *lua.LState) int {
		L.Push(lua.LNil)
		L.Push(lua.LString("TUI module has been removed from Hype. Use simple REPL mode instead."))
		return 2
	})
	
	L.SetField(tuiModule, "newApp", errorFunc)
	L.SetField(tuiModule, "newTextView", errorFunc)
	L.SetField(tuiModule, "newInputField", errorFunc)
	L.SetField(tuiModule, "newButton", errorFunc)
	L.SetField(tuiModule, "newFlex", errorFunc)
	
	L.SetGlobal("tui", tuiModule)
}

// setupTUIMetatables removed - TUI functionality no longer supported

// appIndex removed - TUI functionality no longer supported

// All TUI functions removed - TUI functionality no longer supported
// WebSocket stub - functionality removed
func registerWebSocketStub(L *lua.LState) {
	L.PreloadModule("websocket", func(L *lua.LState) int {
		L.RaiseError("WebSocket module has been removed from Hype. Use HTTP or other alternatives.")
		return 0
	})
}

func registerKVModule(L *lua.LState) {
	L.PreloadModule("kv", func(L *lua.LState) int {
		kvModule := L.NewTable()
		L.SetField(kvModule, "open", L.NewFunction(kvOpen))
		L.Push(kvModule)
		return 1
	})
	
	// Set up database metatable
	dbMT := L.NewTypeMetatable("KVDB")
	L.SetField(dbMT, "__index", L.NewFunction(kvIndex))
	L.SetField(dbMT, "__gc", L.NewFunction(kvGC))
	
	// Set up transaction metatable
	txnMT := L.NewTypeMetatable("KVTxn")
	L.SetField(txnMT, "__index", L.NewFunction(kvTxnIndex))
	L.SetField(txnMT, "__gc", L.NewFunction(kvTxnGC))
	
	// Set up cursor metatable
	cursorMT := L.NewTypeMetatable("KVCursor")
	L.SetField(cursorMT, "__index", L.NewFunction(kvCursorIndex))
	L.SetField(cursorMT, "__gc", L.NewFunction(kvCursorGC))
}

type KVDB struct {
	db   *bbolt.DB
	path string
}

type KVTxn struct {
	tx     *bbolt.Tx
	db     *KVDB
	bucket *bbolt.Bucket
}

type KVCursor struct {
	cursor *bbolt.Cursor
	tx     *bbolt.Tx
	db     *KVDB
}

func kvOpen(L *lua.LState) int {
	path := L.CheckString(1)
	
	// Optional options table (for compatibility)
	var readonly bool = false
	
	if L.GetTop() >= 2 {
		options := L.CheckTable(2)
		if readonlyVal := L.GetField(options, "readonly"); readonlyVal != lua.LNil {
			if readonlyBool, ok := readonlyVal.(lua.LBool); ok {
				readonly = bool(readonlyBool)
			}
		}
	}
	
	options := &bbolt.Options{
		ReadOnly: readonly,
	}
	
	db, err := bbolt.Open(path, 0644, options)
	if err != nil {
		L.Push(lua.LNil)
		L.Push(lua.LString(err.Error()))
		return 2
	}
	
	kvdb := &KVDB{
		db:   db,
		path: path,
	}
	
	ud := L.NewUserData()
	ud.Value = kvdb
	L.SetMetatable(ud, L.GetTypeMetatable("KVDB"))
	L.Push(ud)
	L.Push(lua.LNil)
	return 2
}

func kvIndex(L *lua.LState) int {
	ud := L.CheckUserData(1)
	db := ud.Value.(*KVDB)
	method := L.CheckString(2)
	
	switch method {
	case "open_db":
		L.Push(L.NewFunction(func(L *lua.LState) int {
			bucketName := L.OptString(2, "default")
			
			// Create bucket if it doesn't exist
			err := db.db.Update(func(tx *bbolt.Tx) error {
				_, err := tx.CreateBucketIfNotExists([]byte(bucketName))
				return err
			})
			
			if err != nil {
				L.Push(lua.LString(err.Error()))
				return 1
			}
			
			L.Push(lua.LNil) // No error
			return 1
		}))
	case "begin_txn":
		L.Push(L.NewFunction(func(L *lua.LState) int {
			readonly := L.OptBool(2, false)
			
			var tx *bbolt.Tx
			var err error
			
			if readonly {
				tx, err = db.db.Begin(false)
			} else {
				tx, err = db.db.Begin(true)
			}
			
			if err != nil {
				L.Push(lua.LNil)
				L.Push(lua.LString(err.Error()))
				return 2
			}
			
			kvTxn := &KVTxn{tx: tx, db: db}
			
			ud := L.NewUserData()
			ud.Value = kvTxn
			L.SetMetatable(ud, L.GetTypeMetatable("KVTxn"))
			L.Push(ud)
			L.Push(lua.LNil)
			return 2
		}))
	case "get":
		L.Push(L.NewFunction(func(L *lua.LState) int {
			bucketName := L.OptString(2, "default")
			key := L.CheckString(3)
			
			var val []byte
			err := db.db.View(func(tx *bbolt.Tx) error {
				bucket := tx.Bucket([]byte(bucketName))
				if bucket == nil {
					return fmt.Errorf("bucket not found: %s", bucketName)
				}
				val = bucket.Get([]byte(key))
				return nil
			})
			
			if err != nil {
				L.Push(lua.LNil)
				L.Push(lua.LString(err.Error()))
				return 2
			}
			
			if val == nil {
				L.Push(lua.LNil)
				L.Push(lua.LNil)
				return 2
			}
			
			L.Push(lua.LString(string(val)))
			L.Push(lua.LNil)
			return 2
		}))
	case "put":
		L.Push(L.NewFunction(func(L *lua.LState) int {
			bucketName := L.OptString(2, "default")
			key := L.CheckString(3)
			value := L.CheckString(4)
			
			err := db.db.Update(func(tx *bbolt.Tx) error {
				bucket := tx.Bucket([]byte(bucketName))
				if bucket == nil {
					return fmt.Errorf("bucket not found: %s", bucketName)
				}
				return bucket.Put([]byte(key), []byte(value))
			})
			
			if err != nil {
				L.Push(lua.LString(err.Error()))
				return 1
			}
			
			L.Push(lua.LNil)
			return 1
		}))
	case "delete":
		L.Push(L.NewFunction(func(L *lua.LState) int {
			bucketName := L.OptString(2, "default")
			key := L.CheckString(3)
			
			err := db.db.Update(func(tx *bbolt.Tx) error {
				bucket := tx.Bucket([]byte(bucketName))
				if bucket == nil {
					return fmt.Errorf("bucket not found: %s", bucketName)
				}
				return bucket.Delete([]byte(key))
			})
			
			if err != nil {
				L.Push(lua.LString(err.Error()))
				return 1
			}
			
			L.Push(lua.LNil)
			return 1
		}))
	case "keys":
		L.Push(L.NewFunction(func(L *lua.LState) int {
			bucketName := L.OptString(2, "default")
			prefix := L.OptString(3, "")
			
			var keys []string
			err := db.db.View(func(tx *bbolt.Tx) error {
				bucket := tx.Bucket([]byte(bucketName))
				if bucket == nil {
					return fmt.Errorf("bucket not found: %s", bucketName)
				}
				
				c := bucket.Cursor()
				if prefix != "" {
					// Prefix search
					prefixBytes := []byte(prefix)
					for k, _ := c.Seek(prefixBytes); k != nil && bytes.HasPrefix(k, prefixBytes); k, _ = c.Next() {
						keys = append(keys, string(k))
					}
				} else {
					// All keys
					for k, _ := c.First(); k != nil; k, _ = c.Next() {
						keys = append(keys, string(k))
					}
				}
				return nil
			})
			
			if err != nil {
				L.Push(lua.LNil)
				L.Push(lua.LString(err.Error()))
				return 2
			}
			
			// Create Lua table with keys
			table := L.NewTable()
			for i, key := range keys {
				table.RawSetInt(i+1, lua.LString(key))
			}
			L.Push(table)
			L.Push(lua.LNil)
			return 2
		}))
	case "foreach":
		L.Push(L.NewFunction(func(L *lua.LState) int {
			bucketName := L.OptString(2, "default")
			callback := L.CheckFunction(3)
			
			err := db.db.View(func(tx *bbolt.Tx) error {
				bucket := tx.Bucket([]byte(bucketName))
				if bucket == nil {
					return fmt.Errorf("bucket not found: %s", bucketName)
				}
				
				c := bucket.Cursor()
				for k, v := c.First(); k != nil; k, v = c.Next() {
					L.Push(callback)
					L.Push(lua.LString(string(k)))
					L.Push(lua.LString(string(v)))
					if err := L.PCall(2, 1, nil); err != nil {
						return fmt.Errorf("callback error: %v", err)
					}
					
					// Check if callback returned false to break
					result := L.Get(-1)
					L.Pop(1)
					if result == lua.LFalse {
						break
					}
				}
				return nil
			})
			
			if err != nil {
				L.Push(lua.LString(err.Error()))
				return 1
			}
			
			L.Push(lua.LNil)
			return 1
		}))
	case "close":
		L.Push(L.NewFunction(func(L *lua.LState) int {
			db.db.Close()
			return 0
		}))
	}
	return 1
}

func kvGC(L *lua.LState) int {
	ud := L.CheckUserData(1)
	if db, ok := ud.Value.(*KVDB); ok {
		db.db.Close()
	}
	return 0
}

func kvTxnIndex(L *lua.LState) int {
	ud := L.CheckUserData(1)
	txn := ud.Value.(*KVTxn)
	method := L.CheckString(2)
	
	switch method {
	case "get":
		L.Push(L.NewFunction(func(L *lua.LState) int {
			bucketName := L.OptString(2, "default")
			key := L.CheckString(3)
			
			bucket := txn.tx.Bucket([]byte(bucketName))
			if bucket == nil {
				L.Push(lua.LNil)
				L.Push(lua.LString("bucket not found"))
				return 2
			}
			
			val := bucket.Get([]byte(key))
			if val == nil {
				L.Push(lua.LNil)
				L.Push(lua.LNil)
				return 2
			}
			
			L.Push(lua.LString(string(val)))
			L.Push(lua.LNil)
			return 2
		}))
	case "put":
		L.Push(L.NewFunction(func(L *lua.LState) int {
			bucketName := L.OptString(2, "default")
			key := L.CheckString(3)
			value := L.CheckString(4)
			
			bucket := txn.tx.Bucket([]byte(bucketName))
			if bucket == nil {
				L.Push(lua.LString("bucket not found"))
				return 1
			}
			
			err := bucket.Put([]byte(key), []byte(value))
			if err != nil {
				L.Push(lua.LString(err.Error()))
				return 1
			}
			
			L.Push(lua.LNil)
			return 1
		}))
	case "delete":
		L.Push(L.NewFunction(func(L *lua.LState) int {
			bucketName := L.OptString(2, "default")
			key := L.CheckString(3)
			
			bucket := txn.tx.Bucket([]byte(bucketName))
			if bucket == nil {
				L.Push(lua.LString("bucket not found"))
				return 1
			}
			
			err := bucket.Delete([]byte(key))
			if err != nil {
				L.Push(lua.LString(err.Error()))
				return 1
			}
			
			L.Push(lua.LNil)
			return 1
		}))
	case "commit":
		L.Push(L.NewFunction(func(L *lua.LState) int {
			err := txn.tx.Commit()
			if err != nil {
				L.Push(lua.LString(err.Error()))
				return 1
			}
			L.Push(lua.LNil)
			return 1
		}))
	case "abort":
		L.Push(L.NewFunction(func(L *lua.LState) int {
			txn.tx.Rollback()
			return 0
		}))
	}
	return 1
}

func kvTxnGC(L *lua.LState) int {
	ud := L.CheckUserData(1)
	if txn, ok := ud.Value.(*KVTxn); ok {
		txn.tx.Rollback()
	}
	return 0
}

func kvCursorIndex(L *lua.LState) int {
	ud := L.CheckUserData(1)
	cursor := ud.Value.(*KVCursor)
	method := L.CheckString(2)
	
	switch method {
	case "first":
		L.Push(L.NewFunction(func(L *lua.LState) int {
			var key, value []byte
			err := cursor.db.db.View(func(tx *bbolt.Tx) error {
				c := tx.Bucket([]byte("default")).Cursor() // TODO: make bucket configurable
				key, value = c.First()
				return nil
			})
			
			if err != nil {
				L.Push(lua.LNil)
				L.Push(lua.LNil)
				L.Push(lua.LString(err.Error()))
				return 3
			}
			
			if key == nil {
				L.Push(lua.LNil)
				L.Push(lua.LNil)
				L.Push(lua.LNil)
				return 3
			}
			
			L.Push(lua.LString(string(key)))
			L.Push(lua.LString(string(value)))
			L.Push(lua.LNil)
			return 3
		}))
	case "last":
		L.Push(L.NewFunction(func(L *lua.LState) int {
			var key, value []byte
			err := cursor.db.db.View(func(tx *bbolt.Tx) error {
				c := tx.Bucket([]byte("default")).Cursor()
				key, value = c.Last()
				return nil
			})
			
			if err != nil {
				L.Push(lua.LNil)
				L.Push(lua.LNil)
				L.Push(lua.LString(err.Error()))
				return 3
			}
			
			if key == nil {
				L.Push(lua.LNil)
				L.Push(lua.LNil)
				L.Push(lua.LNil)
				return 3
			}
			
			L.Push(lua.LString(string(key)))
			L.Push(lua.LString(string(value)))
			L.Push(lua.LNil)
			return 3
		}))
	case "seek":
		L.Push(L.NewFunction(func(L *lua.LState) int {
			seekKey := L.CheckString(2)
			
			var key, value []byte
			err := cursor.db.db.View(func(tx *bbolt.Tx) error {
				c := tx.Bucket([]byte("default")).Cursor()
				key, value = c.Seek([]byte(seekKey))
				return nil
			})
			
			if err != nil {
				L.Push(lua.LNil)
				L.Push(lua.LNil)
				L.Push(lua.LString(err.Error()))
				return 3
			}
			
			if key == nil {
				L.Push(lua.LNil)
				L.Push(lua.LNil)
				L.Push(lua.LNil)
				return 3
			}
			
			L.Push(lua.LString(string(key)))
			L.Push(lua.LString(string(value)))
			L.Push(lua.LNil)
			return 3
		}))
	}
	return 1
}

func kvCursorGC(L *lua.LState) int {
	// Cursors are automatically cleaned up when transaction ends
	return 0
}

// JWK represents a JSON Web Key
type JWK struct {
	Kty string ` + "`json:\"kty\"`" + `           // Key Type
	Alg string ` + "`json:\"alg,omitempty\"`" + ` // Algorithm
	Use string ` + "`json:\"use,omitempty\"`" + ` // Public Key Use
	Kid string ` + "`json:\"kid,omitempty\"`" + ` // Key ID
	
	// RSA keys
	N string ` + "`json:\"n,omitempty\"`" + ` // Modulus
	E string ` + "`json:\"e,omitempty\"`" + ` // Exponent
	D string ` + "`json:\"d,omitempty\"`" + ` // Private Exponent
	P string ` + "`json:\"p,omitempty\"`" + ` // First Prime Factor
	Q string ` + "`json:\"q,omitempty\"`" + ` // Second Prime Factor
	Dp string ` + "`json:\"dp,omitempty\"`" + ` // First Factor CRT Exponent
	Dq string ` + "`json:\"dq,omitempty\"`" + ` // Second Factor CRT Exponent
	Qi string ` + "`json:\"qi,omitempty\"`" + ` // First CRT Coefficient
	
	// ECDSA keys
	Crv string ` + "`json:\"crv,omitempty\"`" + ` // Curve
	X   string ` + "`json:\"x,omitempty\"`" + `   // X Coordinate
	Y   string ` + "`json:\"y,omitempty\"`" + `   // Y Coordinate
	
	// EdDSA keys (same as ECDSA for Ed25519)
}

// registerCryptoModule adds crypto functionality to Lua
func registerCryptoModule(L *lua.LState) {
	L.PreloadModule("crypto", func(L *lua.LState) int {
		cryptoModule := L.NewTable()
		
		L.SetField(cryptoModule, "generate_jwk", L.NewFunction(cryptoGenerateJWK))
		L.SetField(cryptoModule, "sign", L.NewFunction(cryptoSign))
		L.SetField(cryptoModule, "verify", L.NewFunction(cryptoVerify))
		L.SetField(cryptoModule, "jwk_to_public", L.NewFunction(cryptoJWKToPublic))
		L.SetField(cryptoModule, "jwk_thumbprint", L.NewFunction(cryptoJWKThumbprint))
		L.SetField(cryptoModule, "jwk_to_json", L.NewFunction(cryptoJWKToJSON))
		L.SetField(cryptoModule, "jwk_from_json", L.NewFunction(cryptoJWKFromJSON))
		
		// Hashing functions
		L.SetField(cryptoModule, "sha256", L.NewFunction(cryptoSHA256))
		L.SetField(cryptoModule, "sha384", L.NewFunction(cryptoSHA384))
		L.SetField(cryptoModule, "sha512", L.NewFunction(cryptoSHA512))
		L.SetField(cryptoModule, "hash", L.NewFunction(cryptoHash))
		L.SetField(cryptoModule, "deep_hash", L.NewFunction(cryptoDeepHash))
		
		L.Push(cryptoModule)
		return 1
	})
}

// cryptoGenerateJWK generates a new JWK keypair
func cryptoGenerateJWK(L *lua.LState) int {
	algorithm := L.ToString(1)
	if algorithm == "" {
		algorithm = "RS256" // default
	}
	
	// Optional second parameter for key size (RSA only)
	keySize := 0
	if L.GetTop() >= 2 {
		keySize = L.ToInt(2)
	}
	
	var jwk *JWK
	var err error
	
	switch algorithm {
	case "RS256", "RS384", "RS512", "PS256", "PS384", "PS512":
		jwk, err = generateRSAJWK(algorithm, keySize)
	case "ES256", "ES384", "ES512":
		jwk, err = generateECDSAJWK(algorithm)
	case "EdDSA":
		jwk, err = generateEd25519JWK()
	default:
		L.Push(lua.LNil)
		L.Push(lua.LString("unsupported algorithm: " + algorithm))
		return 2
	}
	
	if err != nil {
		L.Push(lua.LNil)
		L.Push(lua.LString("failed to generate key: " + err.Error()))
		return 2
	}
	
	// Convert JWK to Lua table
	jwkTable := jwkToLuaTable(L, jwk)
	L.Push(jwkTable)
	return 1
}

// generateRSAJWK creates an RSA JWK
func generateRSAJWK(algorithm string, requestedKeySize int) (*JWK, error) {
	keySize := 2048  // default
	
	// If a specific key size was requested, validate and use it
	if requestedKeySize > 0 {
		if requestedKeySize == 2048 || requestedKeySize == 3072 || requestedKeySize == 4096 {
			keySize = requestedKeySize
		} else {
			return nil, fmt.Errorf("unsupported RSA key size: %d (supported: 2048, 3072, 4096)", requestedKeySize)
		}
	} else {
		// Use default key sizes based on algorithm
		if algorithm == "RS384" || algorithm == "RS512" || algorithm == "PS384" || algorithm == "PS512" {
			keySize = 3072
		}
	}
	
	privateKey, err := rsa.GenerateKey(rand.Reader, keySize)
	if err != nil {
		return nil, err
	}
	
	jwk := &JWK{
		Kty: "RSA",
		Alg: algorithm,
		Use: "sig",
		Kid: fmt.Sprintf("%d", time.Now().Unix()),
		N:   base64.RawURLEncoding.EncodeToString(privateKey.N.Bytes()),
		E:   base64.RawURLEncoding.EncodeToString(big.NewInt(int64(privateKey.E)).Bytes()),
		D:   base64.RawURLEncoding.EncodeToString(privateKey.D.Bytes()),
		P:   base64.RawURLEncoding.EncodeToString(privateKey.Primes[0].Bytes()),
		Q:   base64.RawURLEncoding.EncodeToString(privateKey.Primes[1].Bytes()),
	}
	
	// Calculate CRT values
	dp := new(big.Int).Mod(privateKey.D, new(big.Int).Sub(privateKey.Primes[0], big.NewInt(1)))
	dq := new(big.Int).Mod(privateKey.D, new(big.Int).Sub(privateKey.Primes[1], big.NewInt(1)))
	qi := new(big.Int).ModInverse(privateKey.Primes[1], privateKey.Primes[0])
	
	jwk.Dp = base64.RawURLEncoding.EncodeToString(dp.Bytes())
	jwk.Dq = base64.RawURLEncoding.EncodeToString(dq.Bytes())
	jwk.Qi = base64.RawURLEncoding.EncodeToString(qi.Bytes())
	
	return jwk, nil
}

// generateECDSAJWK creates an ECDSA JWK
func generateECDSAJWK(algorithm string) (*JWK, error) {
	var curve elliptic.Curve
	var crvName string
	
	switch algorithm {
	case "ES256":
		curve = elliptic.P256()
		crvName = "P-256"
	case "ES384":
		curve = elliptic.P384()
		crvName = "P-384"
	case "ES512":
		curve = elliptic.P521()
		crvName = "P-521"
	default:
		return nil, fmt.Errorf("unsupported ECDSA algorithm: %s", algorithm)
	}
	
	privateKey, err := ecdsa.GenerateKey(curve, rand.Reader)
	if err != nil {
		return nil, err
	}
	
	// Get curve size for padding
	keySize := (curve.Params().BitSize + 7) / 8
	
	xBytes := privateKey.X.Bytes()
	yBytes := privateKey.Y.Bytes()
	dBytes := privateKey.D.Bytes()
	
	// Pad to correct length
	if len(xBytes) < keySize {
		padded := make([]byte, keySize)
		copy(padded[keySize-len(xBytes):], xBytes)
		xBytes = padded
	}
	if len(yBytes) < keySize {
		padded := make([]byte, keySize)
		copy(padded[keySize-len(yBytes):], yBytes)
		yBytes = padded
	}
	if len(dBytes) < keySize {
		padded := make([]byte, keySize)
		copy(padded[keySize-len(dBytes):], dBytes)
		dBytes = padded
	}
	
	jwk := &JWK{
		Kty: "EC",
		Alg: algorithm,
		Use: "sig",
		Kid: fmt.Sprintf("%d", time.Now().Unix()),
		Crv: crvName,
		X:   base64.RawURLEncoding.EncodeToString(xBytes),
		Y:   base64.RawURLEncoding.EncodeToString(yBytes),
		D:   base64.RawURLEncoding.EncodeToString(dBytes),
	}
	
	return jwk, nil
}

// generateEd25519JWK creates an Ed25519 JWK
func generateEd25519JWK() (*JWK, error) {
	publicKey, privateKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return nil, err
	}
	
	jwk := &JWK{
		Kty: "OKP",
		Alg: "EdDSA",
		Use: "sig",
		Kid: fmt.Sprintf("%d", time.Now().Unix()),
		Crv: "Ed25519",
		X:   base64.RawURLEncoding.EncodeToString(publicKey),
		D:   base64.RawURLEncoding.EncodeToString(privateKey[:32]), // Ed25519 private key is 32 bytes
	}
	
	return jwk, nil
}

// cryptoSign signs data with a JWK
func cryptoSign(L *lua.LState) int {
	jwkTable := L.ToTable(1)
	data := L.ToString(2)
	
	if jwkTable == nil || data == "" {
		L.Push(lua.LNil)
		L.Push(lua.LString("missing jwk or data"))
		return 2
	}
	
	jwk, err := luaTableToJWK(jwkTable)
	if err != nil {
		L.Push(lua.LNil)
		L.Push(lua.LString("invalid jwk: " + err.Error()))
		return 2
	}
	
	signature, err := signWithJWK(jwk, []byte(data))
	if err != nil {
		L.Push(lua.LNil)
		L.Push(lua.LString("signing failed: " + err.Error()))
		return 2
	}
	
	L.Push(lua.LString(base64.RawURLEncoding.EncodeToString(signature)))
	return 1
}

// cryptoVerify verifies a signature with a JWK
func cryptoVerify(L *lua.LState) int {
	jwkTable := L.ToTable(1)
	data := L.ToString(2)
	signatureB64 := L.ToString(3)
	
	if jwkTable == nil || data == "" || signatureB64 == "" {
		L.Push(lua.LBool(false))
		L.Push(lua.LString("missing jwk, data, or signature"))
		return 2
	}
	
	jwk, err := luaTableToJWK(jwkTable)
	if err != nil {
		L.Push(lua.LBool(false))
		L.Push(lua.LString("invalid jwk: " + err.Error()))
		return 2
	}
	
	signature, err := base64.RawURLEncoding.DecodeString(signatureB64)
	if err != nil {
		L.Push(lua.LBool(false))
		L.Push(lua.LString("invalid signature encoding: " + err.Error()))
		return 2
	}
	
	valid, err := verifyWithJWK(jwk, []byte(data), signature)
	if err != nil {
		L.Push(lua.LBool(false))
		L.Push(lua.LString("verification failed: " + err.Error()))
		return 2
	}
	
	L.Push(lua.LBool(valid))
	return 1
}

// signWithJWK signs data using a JWK
func signWithJWK(jwk *JWK, data []byte) ([]byte, error) {
	switch jwk.Kty {
	case "RSA":
		return signRSA(jwk, data)
	case "EC":
		return signECDSA(jwk, data)
	case "OKP":
		if jwk.Crv == "Ed25519" {
			return signEd25519(jwk, data)
		}
		return nil, fmt.Errorf("unsupported OKP curve: %s", jwk.Crv)
	default:
		return nil, fmt.Errorf("unsupported key type: %s", jwk.Kty)
	}
}

// verifyWithJWK verifies a signature using a JWK
func verifyWithJWK(jwk *JWK, data []byte, signature []byte) (bool, error) {
	switch jwk.Kty {
	case "RSA":
		return verifyRSA(jwk, data, signature)
	case "EC":
		return verifyECDSA(jwk, data, signature)
	case "OKP":
		if jwk.Crv == "Ed25519" {
			return verifyEd25519(jwk, data, signature)
		}
		return false, fmt.Errorf("unsupported OKP curve: %s", jwk.Crv)
	default:
		return false, fmt.Errorf("unsupported key type: %s", jwk.Kty)
	}
}

// signRSA signs data with RSA
func signRSA(jwk *JWK, data []byte) ([]byte, error) {
	privateKey, err := jwkToRSAPrivateKey(jwk)
	if err != nil {
		return nil, err
	}
	
	var hash crypto.Hash
	switch jwk.Alg {
	case "RS256", "PS256":
		hash = crypto.SHA256
	case "RS384", "PS384":
		hash = crypto.SHA384
	case "RS512", "PS512":
		hash = crypto.SHA512
	default:
		return nil, fmt.Errorf("unsupported RSA algorithm: %s", jwk.Alg)
	}
	
	hasher := hash.New()
	hasher.Write(data)
	hashed := hasher.Sum(nil)
	
	// Use PSS for PS algorithms
	switch jwk.Alg {
	case "PS256", "PS384", "PS512":
		opts := &rsa.PSSOptions{
			SaltLength: rsa.PSSSaltLengthEqualsHash,
			Hash:       hash,
		}
		return rsa.SignPSS(rand.Reader, privateKey, hash, hashed, opts)
	default:
		return rsa.SignPKCS1v15(rand.Reader, privateKey, hash, hashed)
	}
}

// verifyRSA verifies an RSA signature
func verifyRSA(jwk *JWK, data []byte, signature []byte) (bool, error) {
	publicKey, err := jwkToRSAPublicKey(jwk)
	if err != nil {
		return false, err
	}
	
	var hash crypto.Hash
	switch jwk.Alg {
	case "RS256", "PS256":
		hash = crypto.SHA256
	case "RS384", "PS384":
		hash = crypto.SHA384
	case "RS512", "PS512":
		hash = crypto.SHA512
	default:
		return false, fmt.Errorf("unsupported RSA algorithm: %s", jwk.Alg)
	}
	
	hasher := hash.New()
	hasher.Write(data)
	hashed := hasher.Sum(nil)
	
	// Use PSS for PS algorithms
	switch jwk.Alg {
	case "PS256", "PS384", "PS512":
		opts := &rsa.PSSOptions{
			SaltLength: rsa.PSSSaltLengthEqualsHash,
			Hash:       hash,
		}
		err = rsa.VerifyPSS(publicKey, hash, hashed, signature, opts)
	default:
		err = rsa.VerifyPKCS1v15(publicKey, hash, hashed, signature)
	}
	return err == nil, nil
}

// signECDSA signs data with ECDSA
func signECDSA(jwk *JWK, data []byte) ([]byte, error) {
	privateKey, err := jwkToECDSAPrivateKey(jwk)
	if err != nil {
		return nil, err
	}
	
	var hasher crypto.Hash
	switch jwk.Alg {
	case "ES256":
		hasher = crypto.SHA256
	case "ES384":
		hasher = crypto.SHA384
	case "ES512":
		hasher = crypto.SHA512
	default:
		return nil, fmt.Errorf("unsupported ECDSA algorithm: %s", jwk.Alg)
	}
	
	hash := hasher.New()
	hash.Write(data)
	hashed := hash.Sum(nil)
	
	r, s, err := ecdsa.Sign(rand.Reader, privateKey, hashed)
	if err != nil {
		return nil, err
	}
	
	// Get curve size for signature formatting
	keySize := (privateKey.Curve.Params().BitSize + 7) / 8
	
	rBytes := r.Bytes()
	sBytes := s.Bytes()
	
	// Pad to correct length
	signature := make([]byte, 2*keySize)
	copy(signature[keySize-len(rBytes):keySize], rBytes)
	copy(signature[2*keySize-len(sBytes):], sBytes)
	
	return signature, nil
}

// verifyECDSA verifies an ECDSA signature
func verifyECDSA(jwk *JWK, data []byte, signature []byte) (bool, error) {
	publicKey, err := jwkToECDSAPublicKey(jwk)
	if err != nil {
		return false, err
	}
	
	var hasher crypto.Hash
	switch jwk.Alg {
	case "ES256":
		hasher = crypto.SHA256
	case "ES384":
		hasher = crypto.SHA384
	case "ES512":
		hasher = crypto.SHA512
	default:
		return false, fmt.Errorf("unsupported ECDSA algorithm: %s", jwk.Alg)
	}
	
	hash := hasher.New()
	hash.Write(data)
	hashed := hash.Sum(nil)
	
	// Parse signature (r, s values)
	keySize := len(signature) / 2
	r := new(big.Int).SetBytes(signature[:keySize])
	s := new(big.Int).SetBytes(signature[keySize:])
	
	return ecdsa.Verify(publicKey, hashed, r, s), nil
}

// signEd25519 signs data with Ed25519
func signEd25519(jwk *JWK, data []byte) ([]byte, error) {
	privateKey, err := jwkToEd25519PrivateKey(jwk)
	if err != nil {
		return nil, err
	}
	
	return ed25519.Sign(privateKey, data), nil
}

// verifyEd25519 verifies an Ed25519 signature
func verifyEd25519(jwk *JWK, data []byte, signature []byte) (bool, error) {
	publicKey, err := jwkToEd25519PublicKey(jwk)
	if err != nil {
		return false, err
	}
	
	return ed25519.Verify(publicKey, data, signature), nil
}

// Helper functions for JWK conversion
func jwkToRSAPrivateKey(jwk *JWK) (*rsa.PrivateKey, error) {
	nBytes, err := base64.RawURLEncoding.DecodeString(jwk.N)
	if err != nil {
		return nil, err
	}
	eBytes, err := base64.RawURLEncoding.DecodeString(jwk.E)
	if err != nil {
		return nil, err
	}
	dBytes, err := base64.RawURLEncoding.DecodeString(jwk.D)
	if err != nil {
		return nil, err
	}
	pBytes, err := base64.RawURLEncoding.DecodeString(jwk.P)
	if err != nil {
		return nil, err
	}
	qBytes, err := base64.RawURLEncoding.DecodeString(jwk.Q)
	if err != nil {
		return nil, err
	}
	
	return &rsa.PrivateKey{
		PublicKey: rsa.PublicKey{
			N: new(big.Int).SetBytes(nBytes),
			E: int(new(big.Int).SetBytes(eBytes).Int64()),
		},
		D:      new(big.Int).SetBytes(dBytes),
		Primes: []*big.Int{new(big.Int).SetBytes(pBytes), new(big.Int).SetBytes(qBytes)},
	}, nil
}

func jwkToRSAPublicKey(jwk *JWK) (*rsa.PublicKey, error) {
	nBytes, err := base64.RawURLEncoding.DecodeString(jwk.N)
	if err != nil {
		return nil, err
	}
	eBytes, err := base64.RawURLEncoding.DecodeString(jwk.E)
	if err != nil {
		return nil, err
	}
	
	return &rsa.PublicKey{
		N: new(big.Int).SetBytes(nBytes),
		E: int(new(big.Int).SetBytes(eBytes).Int64()),
	}, nil
}

func jwkToECDSAPrivateKey(jwk *JWK) (*ecdsa.PrivateKey, error) {
	var curve elliptic.Curve
	switch jwk.Crv {
	case "P-256":
		curve = elliptic.P256()
	case "P-384":
		curve = elliptic.P384()
	case "P-521":
		curve = elliptic.P521()
	default:
		return nil, fmt.Errorf("unsupported curve: %s", jwk.Crv)
	}
	
	xBytes, err := base64.RawURLEncoding.DecodeString(jwk.X)
	if err != nil {
		return nil, err
	}
	yBytes, err := base64.RawURLEncoding.DecodeString(jwk.Y)
	if err != nil {
		return nil, err
	}
	dBytes, err := base64.RawURLEncoding.DecodeString(jwk.D)
	if err != nil {
		return nil, err
	}
	
	return &ecdsa.PrivateKey{
		PublicKey: ecdsa.PublicKey{
			Curve: curve,
			X:     new(big.Int).SetBytes(xBytes),
			Y:     new(big.Int).SetBytes(yBytes),
		},
		D: new(big.Int).SetBytes(dBytes),
	}, nil
}

func jwkToECDSAPublicKey(jwk *JWK) (*ecdsa.PublicKey, error) {
	var curve elliptic.Curve
	switch jwk.Crv {
	case "P-256":
		curve = elliptic.P256()
	case "P-384":
		curve = elliptic.P384()
	case "P-521":
		curve = elliptic.P521()
	default:
		return nil, fmt.Errorf("unsupported curve: %s", jwk.Crv)
	}
	
	xBytes, err := base64.RawURLEncoding.DecodeString(jwk.X)
	if err != nil {
		return nil, err
	}
	yBytes, err := base64.RawURLEncoding.DecodeString(jwk.Y)
	if err != nil {
		return nil, err
	}
	
	return &ecdsa.PublicKey{
		Curve: curve,
		X:     new(big.Int).SetBytes(xBytes),
		Y:     new(big.Int).SetBytes(yBytes),
	}, nil
}

func jwkToEd25519PrivateKey(jwk *JWK) (ed25519.PrivateKey, error) {
	xBytes, err := base64.RawURLEncoding.DecodeString(jwk.X)
	if err != nil {
		return nil, err
	}
	dBytes, err := base64.RawURLEncoding.DecodeString(jwk.D)
	if err != nil {
		return nil, err
	}
	
	// Ed25519 private key is 64 bytes: 32 bytes private + 32 bytes public
	privateKey := make(ed25519.PrivateKey, ed25519.PrivateKeySize)
	copy(privateKey[:32], dBytes)
	copy(privateKey[32:], xBytes)
	
	return privateKey, nil
}

func jwkToEd25519PublicKey(jwk *JWK) (ed25519.PublicKey, error) {
	xBytes, err := base64.RawURLEncoding.DecodeString(jwk.X)
	if err != nil {
		return nil, err
	}
	
	return ed25519.PublicKey(xBytes), nil
}

// Helper functions for Lua table conversion
func jwkToLuaTable(L *lua.LState, jwk *JWK) *lua.LTable {
	table := L.NewTable()
	
	L.SetField(table, "kty", lua.LString(jwk.Kty))
	if jwk.Alg != "" {
		L.SetField(table, "alg", lua.LString(jwk.Alg))
	}
	if jwk.Use != "" {
		L.SetField(table, "use", lua.LString(jwk.Use))
	}
	if jwk.Kid != "" {
		L.SetField(table, "kid", lua.LString(jwk.Kid))
	}
	
	// RSA fields
	if jwk.N != "" {
		L.SetField(table, "n", lua.LString(jwk.N))
	}
	if jwk.E != "" {
		L.SetField(table, "e", lua.LString(jwk.E))
	}
	if jwk.D != "" {
		L.SetField(table, "d", lua.LString(jwk.D))
	}
	if jwk.P != "" {
		L.SetField(table, "p", lua.LString(jwk.P))
	}
	if jwk.Q != "" {
		L.SetField(table, "q", lua.LString(jwk.Q))
	}
	if jwk.Dp != "" {
		L.SetField(table, "dp", lua.LString(jwk.Dp))
	}
	if jwk.Dq != "" {
		L.SetField(table, "dq", lua.LString(jwk.Dq))
	}
	if jwk.Qi != "" {
		L.SetField(table, "qi", lua.LString(jwk.Qi))
	}
	
	// ECDSA/EdDSA fields
	if jwk.Crv != "" {
		L.SetField(table, "crv", lua.LString(jwk.Crv))
	}
	if jwk.X != "" {
		L.SetField(table, "x", lua.LString(jwk.X))
	}
	if jwk.Y != "" {
		L.SetField(table, "y", lua.LString(jwk.Y))
	}
	
	return table
}

func luaTableToJWK(table *lua.LTable) (*JWK, error) {
	jwk := &JWK{}
	
	table.ForEach(func(key, value lua.LValue) {
		keyStr := key.String()
		valueStr := value.String()
		
		switch keyStr {
		case "kty":
			jwk.Kty = valueStr
		case "alg":
			jwk.Alg = valueStr
		case "use":
			jwk.Use = valueStr
		case "kid":
			jwk.Kid = valueStr
		case "n":
			jwk.N = valueStr
		case "e":
			jwk.E = valueStr
		case "d":
			jwk.D = valueStr
		case "p":
			jwk.P = valueStr
		case "q":
			jwk.Q = valueStr
		case "dp":
			jwk.Dp = valueStr
		case "dq":
			jwk.Dq = valueStr
		case "qi":
			jwk.Qi = valueStr
		case "crv":
			jwk.Crv = valueStr
		case "x":
			jwk.X = valueStr
		case "y":
			jwk.Y = valueStr
		}
	})
	
	if jwk.Kty == "" {
		return nil, fmt.Errorf("missing required field: kty")
	}
	
	return jwk, nil
}

// cryptoJWKToPublic extracts public key from JWK
func cryptoJWKToPublic(L *lua.LState) int {
	jwkTable := L.ToTable(1)
	if jwkTable == nil {
		L.Push(lua.LNil)
		L.Push(lua.LString("missing jwk"))
		return 2
	}
	
	jwk, err := luaTableToJWK(jwkTable)
	if err != nil {
		L.Push(lua.LNil)
		L.Push(lua.LString("invalid jwk: " + err.Error()))
		return 2
	}
	
	publicJWK := &JWK{
		Kty: jwk.Kty,
		Alg: jwk.Alg,
		Use: jwk.Use,
		Kid: jwk.Kid,
		N:   jwk.N,
		E:   jwk.E,
		Crv: jwk.Crv,
		X:   jwk.X,
		Y:   jwk.Y,
	}
	
	publicTable := jwkToLuaTable(L, publicJWK)
	L.Push(publicTable)
	return 1
}

// cryptoJWKThumbprint generates a thumbprint for a JWK
func cryptoJWKThumbprint(L *lua.LState) int {
	jwkTable := L.ToTable(1)
	if jwkTable == nil {
		L.Push(lua.LNil)
		L.Push(lua.LString("missing jwk"))
		return 2
	}
	
	jwk, err := luaTableToJWK(jwkTable)
	if err != nil {
		L.Push(lua.LNil)
		L.Push(lua.LString("invalid jwk: " + err.Error()))
		return 2
	}
	
	// Create canonical JSON for thumbprint (RFC 7638)
	var canonical map[string]interface{}
	switch jwk.Kty {
	case "RSA":
		canonical = map[string]interface{}{
			"e":   jwk.E,
			"kty": jwk.Kty,
			"n":   jwk.N,
		}
	case "EC":
		canonical = map[string]interface{}{
			"crv": jwk.Crv,
			"kty": jwk.Kty,
			"x":   jwk.X,
			"y":   jwk.Y,
		}
	case "OKP":
		canonical = map[string]interface{}{
			"crv": jwk.Crv,
			"kty": jwk.Kty,
			"x":   jwk.X,
		}
	default:
		L.Push(lua.LNil)
		L.Push(lua.LString("unsupported key type for thumbprint: " + jwk.Kty))
		return 2
	}
	
	canonicalJSON, err := json.Marshal(canonical)
	if err != nil {
		L.Push(lua.LNil)
		L.Push(lua.LString("failed to create canonical JSON: " + err.Error()))
		return 2
	}
	
	hash := sha256.Sum256(canonicalJSON)
	thumbprint := base64.RawURLEncoding.EncodeToString(hash[:])
	
	L.Push(lua.LString(thumbprint))
	return 1
}

// cryptoJWKToJSON converts JWK to JSON string
func cryptoJWKToJSON(L *lua.LState) int {
	jwkTable := L.ToTable(1)
	if jwkTable == nil {
		L.Push(lua.LNil)
		L.Push(lua.LString("missing jwk"))
		return 2
	}
	
	jwk, err := luaTableToJWK(jwkTable)
	if err != nil {
		L.Push(lua.LNil)
		L.Push(lua.LString("invalid jwk: " + err.Error()))
		return 2
	}
	
	jsonData, err := json.Marshal(jwk)
	if err != nil {
		L.Push(lua.LNil)
		L.Push(lua.LString("failed to marshal JSON: " + err.Error()))
		return 2
	}
	
	L.Push(lua.LString(string(jsonData)))
	return 1
}

// cryptoJWKFromJSON parses JWK from JSON string
func cryptoJWKFromJSON(L *lua.LState) int {
	jsonStr := L.ToString(1)
	if jsonStr == "" {
		L.Push(lua.LNil)
		L.Push(lua.LString("missing json string"))
		return 2
	}
	
	var jwk JWK
	if err := json.Unmarshal([]byte(jsonStr), &jwk); err != nil {
		L.Push(lua.LNil)
		L.Push(lua.LString("invalid JSON: " + err.Error()))
		return 2
	}
	
	jwkTable := jwkToLuaTable(L, &jwk)
	L.Push(jwkTable)
	return 1
}

// cryptoSHA256 computes SHA-256 hash of input
func cryptoSHA256(L *lua.LState) int {
	data := L.ToString(1)
	hash := sha256.Sum256([]byte(data))
	L.Push(lua.LString(hex.EncodeToString(hash[:])))
	return 1
}

// cryptoSHA384 computes SHA-384 hash of input
func cryptoSHA384(L *lua.LState) int {
	data := L.ToString(1)
	hash := sha512.Sum384([]byte(data))
	L.Push(lua.LString(hex.EncodeToString(hash[:])))
	return 1
}

// cryptoSHA512 computes SHA-512 hash of input
func cryptoSHA512(L *lua.LState) int {
	data := L.ToString(1)
	hash := sha512.Sum512([]byte(data))
	L.Push(lua.LString(hex.EncodeToString(hash[:])))
	return 1
}

// cryptoHash computes hash with specified algorithm
func cryptoHash(L *lua.LState) int {
	algorithm := L.ToString(1)
	data := L.ToString(2)
	
	var result string
	switch strings.ToLower(algorithm) {
	case "sha256":
		hash := sha256.Sum256([]byte(data))
		result = hex.EncodeToString(hash[:])
	case "sha384":
		hash := sha512.Sum384([]byte(data))
		result = hex.EncodeToString(hash[:])
	case "sha512":
		hash := sha512.Sum512([]byte(data))
		result = hex.EncodeToString(hash[:])
	default:
		L.Push(lua.LNil)
		L.Push(lua.LString("unsupported algorithm: " + algorithm))
		return 2
	}
	
	L.Push(lua.LString(result))
	return 1
}

// cryptoDeepHash computes SHA-384 hash of nested data structures
func cryptoDeepHash(L *lua.LState) int {
	value := L.Get(1)
	algorithm := L.OptString(2, "sha384")
	
	// Serialize the value to a canonical string representation
	serialized, err := serializeForHashing(value)
	if err != nil {
		L.Push(lua.LNil)
		L.Push(lua.LString("failed to serialize: " + err.Error()))
		return 2
	}
	
	var result string
	switch strings.ToLower(algorithm) {
	case "sha256":
		hash := sha256.Sum256([]byte(serialized))
		result = hex.EncodeToString(hash[:])
	case "sha384":
		hash := sha512.Sum384([]byte(serialized))
		result = hex.EncodeToString(hash[:])
	case "sha512":
		hash := sha512.Sum512([]byte(serialized))
		result = hex.EncodeToString(hash[:])
	default:
		L.Push(lua.LNil)
		L.Push(lua.LString("unsupported algorithm: " + algorithm))
		return 2
	}
	
	L.Push(lua.LString(result))
	return 1
}

// serializeForHashing converts Lua value to canonical string for consistent hashing
func serializeForHashing(lv lua.LValue) (string, error) {
	switch v := lv.(type) {
	case lua.LString:
		return string(v), nil
	case lua.LNumber:
		return fmt.Sprintf("%g", float64(v)), nil
	case lua.LBool:
		if bool(v) {
			return "true", nil
		}
		return "false", nil
	case *lua.LNilType:
		return "null", nil
	case *lua.LTable:
		// Check if it's an array or object
		isArray := true
		maxIndex := 0
		count := 0
		
		v.ForEach(func(k, val lua.LValue) {
			count++
			if num, ok := k.(lua.LNumber); ok {
				if int(num) > maxIndex {
					maxIndex = int(num)
				}
			} else {
				isArray = false
			}
		})
		
		if isArray && count > 0 && maxIndex == count {
			// Serialize as array
			var items []string
			for i := 1; i <= maxIndex; i++ {
				val := v.RawGetInt(i)
				serialized, err := serializeForHashing(val)
				if err != nil {
					return "", err
				}
				items = append(items, serialized)
			}
			return "[" + strings.Join(items, ",") + "]", nil
		} else {
			// Serialize as object with sorted keys
			var pairs []string
			keys := []string{}
			
			// Collect all keys
			v.ForEach(func(k, val lua.LValue) {
				if str, ok := k.(lua.LString); ok {
					keys = append(keys, string(str))
				} else if num, ok := k.(lua.LNumber); ok {
					keys = append(keys, fmt.Sprintf("%g", float64(num)))
				}
			})
			
			// Sort keys for consistent ordering
			sort.Strings(keys)
			
			// Build key-value pairs
			for _, key := range keys {
				val := v.RawGetString(key)
				if val == lua.LNil {
					// Try as number
					if num, err := strconv.ParseFloat(key, 64); err == nil {
						val = v.RawGetInt(int(num))
					}
				}
				
				serialized, err := serializeForHashing(val)
				if err != nil {
					return "", err
				}
				pairs = append(pairs, fmt.Sprintf("\"%s\":%s", key, serialized))
			}
			
			return "{" + strings.Join(pairs, ",") + "}", nil
		}
	default:
		return "", fmt.Errorf("unsupported type: %T", lv)
	}
}

// registerHTTPSigModule adds HTTP signature functionality to Lua
func registerHTTPSigModule(L *lua.LState) {
	L.PreloadModule("httpsig", func(L *lua.LState) int {
		httpsigModule := L.NewTable()
		
		L.SetField(httpsigModule, "sign", L.NewFunction(httpsigSign))
		L.SetField(httpsigModule, "verify", L.NewFunction(httpsigVerify))
		L.SetField(httpsigModule, "create_digest", L.NewFunction(httpsigCreateDigest))
		L.SetField(httpsigModule, "verify_digest", L.NewFunction(httpsigVerifyDigest))
		
		L.Push(httpsigModule)
		return 1
	})
}

// HTTP signature types for runtime
type HTTPMessage struct {
	Type    string            // "request" or "response"
	Method  string            // HTTP method (for requests)
	Path    string            // Request path (for requests)
	Status  int               // Status code (for responses)
	Headers map[string]string // HTTP headers
	Body    string            // Message body
}

type HTTPSignatureOptions struct {
	JWK             *JWK     // Signing/verification key
	KeyID           string   // Key identifier
	Algorithm       string   // Signature algorithm
	Headers         []string // Headers to include in signature
	Created         int64    // Creation timestamp
	Expires         int64    // Expiration timestamp
	RequiredHeaders []string // Required headers for verification
	MaxAge          int64    // Maximum signature age in seconds
}

type HTTPSignatureResult struct {
	Valid     bool
	KeyID     string
	Algorithm string
	Reason    string
}

// httpsigSign signs an HTTP message
func httpsigSign(L *lua.LState) int {
	messageTable := L.ToTable(1)
	optionsTable := L.ToTable(2)
	
	if messageTable == nil {
		L.Push(lua.LNil)
		L.Push(lua.LString("missing message table"))
		return 2
	}
	
	if optionsTable == nil {
		L.Push(lua.LNil)
		L.Push(lua.LString("missing options table"))
		return 2
	}
	
	// Parse message
	message, err := luaTableToHTTPMessage(messageTable)
	if err != nil {
		L.Push(lua.LNil)
		L.Push(lua.LString("invalid message: " + err.Error()))
		return 2
	}
	
	// Parse options
	options, err := luaTableToHTTPSignatureOptions(optionsTable)
	if err != nil {
		L.Push(lua.LNil)
		L.Push(lua.LString("invalid options: " + err.Error()))
		return 2
	}
	
	// Generate signature
	signatureHeader, err := createHTTPSignature(message, options)
	if err != nil {
		L.Push(lua.LNil)
		L.Push(lua.LString("signing failed: " + err.Error()))
		return 2
	}
	
	// Return updated headers with signature
	resultTable := L.NewTable()
	
	newHeaders := L.NewTable()
	
	// Copy all headers from the message (including ones added during signing)
	for key, value := range message.Headers {
		L.SetField(newHeaders, key, lua.LString(value))
	}
	
	// Add signature header
	L.SetField(newHeaders, "signature", lua.LString(signatureHeader))
	
	L.SetField(resultTable, "headers", newHeaders)
	L.Push(resultTable)
	return 1
}

// httpsigVerify verifies an HTTP message signature
func httpsigVerify(L *lua.LState) int {
	messageTable := L.ToTable(1)
	optionsTable := L.ToTable(2)
	
	if messageTable == nil || optionsTable == nil {
		L.Push(lua.LBool(false))
		L.Push(lua.LString("missing message or options"))
		return 2
	}
	
	// Parse message
	message, err := luaTableToHTTPMessage(messageTable)
	if err != nil {
		L.Push(lua.LBool(false))
		L.Push(lua.LString("invalid message: " + err.Error()))
		return 2
	}
	
	// Parse options
	options, err := luaTableToHTTPSignatureOptions(optionsTable)
	if err != nil {
		L.Push(lua.LBool(false))
		L.Push(lua.LString("invalid options: " + err.Error()))
		return 2
	}
	
	// Verify signature
	result, err := verifyHTTPSignature(message, options)
	if err != nil {
		L.Push(lua.LBool(false))
		L.Push(lua.LString("verification failed: " + err.Error()))
		return 2
	}
	
	// Return verification result
	resultTable := L.NewTable()
	L.SetField(resultTable, "valid", lua.LBool(result.Valid))
	L.SetField(resultTable, "key_id", lua.LString(result.KeyID))
	L.SetField(resultTable, "algorithm", lua.LString(result.Algorithm))
	if result.Reason != "" {
		L.SetField(resultTable, "reason", lua.LString(result.Reason))
	}
	
	L.Push(resultTable)
	return 1
}

// httpsigCreateDigest creates a digest header for body content
func httpsigCreateDigest(L *lua.LState) int {
	content := L.ToString(1)
	algorithm := L.ToString(2)
	
	if algorithm == "" {
		algorithm = "sha256" // default
	}
	
	digest, err := createDigest(content, algorithm)
	if err != nil {
		L.Push(lua.LNil)
		L.Push(lua.LString("digest creation failed: " + err.Error()))
		return 2
	}
	
	L.Push(lua.LString(digest))
	return 1
}

// httpsigVerifyDigest verifies a digest header against content
func httpsigVerifyDigest(L *lua.LState) int {
	content := L.ToString(1)
	digestHeader := L.ToString(2)
	
	if content == "" || digestHeader == "" {
		L.Push(lua.LBool(false))
		L.Push(lua.LString("missing content or digest header"))
		return 2
	}
	
	valid, err := verifyDigest(content, digestHeader)
	if err != nil {
		L.Push(lua.LBool(false))
		L.Push(lua.LString("digest verification failed: " + err.Error()))
		return 2
	}
	
	L.Push(lua.LBool(valid))
	return 1
}

// Helper functions for HTTP signatures

func createHTTPSignature(message *HTTPMessage, options *HTTPSignatureOptions) (string, error) {
	// Add date header if not present
	if !hasHeader(message.Headers, "date") {
		message.Headers["date"] = time.Now().UTC().Format(time.RFC1123)
	}
	
	// Determine headers to sign
	headersToSign := options.Headers
	if len(headersToSign) == 0 {
		// Default headers based on message type
		if message.Type == "request" {
			headersToSign = []string{"(request-target)", "host", "date"}
		} else {
			headersToSign = []string{"(status)", "content-type", "date"}
		}
	}
	
	// Create digest header if needed
	if contains(headersToSign, "digest") && !hasHeader(message.Headers, "digest") {
		digest, err := createDigest(message.Body, "sha256")
		if err != nil {
			return "", fmt.Errorf("failed to create digest: %w", err)
		}
		message.Headers["digest"] = digest
	}
	
	// Build signing string
	signingString, err := buildSigningString(message, headersToSign, options.Created, options.Expires)
	if err != nil {
		return "", fmt.Errorf("failed to build signing string: %w", err)
	}
	
	// Sign the string
	signature, err := signWithJWK(options.JWK, []byte(signingString))
	if err != nil {
		return "", fmt.Errorf("failed to sign: %w", err)
	}
	
	// Build signature header
	signatureB64 := base64.StdEncoding.EncodeToString(signature)
	
	var parts []string
	parts = append(parts, fmt.Sprintf("keyId=\"%s\"", options.KeyID))
	parts = append(parts, fmt.Sprintf("algorithm=\"%s\"", getSignatureAlgorithm(options.JWK)))
	parts = append(parts, fmt.Sprintf("headers=\"%s\"", strings.Join(headersToSign, " ")))
	
	if options.Created > 0 {
		parts = append(parts, fmt.Sprintf("created=%d", options.Created))
	}
	if options.Expires > 0 {
		parts = append(parts, fmt.Sprintf("expires=%d", options.Expires))
	}
	
	parts = append(parts, fmt.Sprintf("signature=\"%s\"", signatureB64))
	
	return strings.Join(parts, ","), nil
}

func verifyHTTPSignature(message *HTTPMessage, options *HTTPSignatureOptions) (*HTTPSignatureResult, error) {
	result := &HTTPSignatureResult{}
	
	// Parse signature header
	signatureHeader, ok := message.Headers["signature"]
	if !ok {
		result.Reason = "missing signature header"
		return result, nil
	}
	
	sigParams, err := parseSignatureHeader(signatureHeader)
	if err != nil {
		result.Reason = "invalid signature header format"
		return result, nil
	}
	
	result.KeyID = sigParams["keyId"]
	result.Algorithm = sigParams["algorithm"]
	
	// Check required headers
	signedHeaders := strings.Fields(sigParams["headers"])
	for _, required := range options.RequiredHeaders {
		if !contains(signedHeaders, required) {
			result.Reason = fmt.Sprintf("required header '%s' not signed", required)
			return result, nil
		}
	}
	
	// Validate digest if digest header was signed
	if contains(signedHeaders, "digest") {
		digestHeader, ok := message.Headers["digest"]
		if !ok {
			result.Reason = "digest header missing but was signed"
			return result, nil
		}
		
		// Verify digest matches the body content
		digestValid, err := verifyDigest(message.Body, digestHeader)
		if err != nil {
			result.Reason = "digest verification failed: " + err.Error()
			return result, nil
		}
		
		if !digestValid {
			result.Reason = "digest mismatch - body content has been tampered"
			return result, nil
		}
	}
	
	// Build signing string
	created, _ := strconv.ParseInt(sigParams["created"], 10, 64)
	expires, _ := strconv.ParseInt(sigParams["expires"], 10, 64)
	
	signingString, err := buildSigningString(message, signedHeaders, created, expires)
	if err != nil {
		result.Reason = "failed to build signing string"
		return result, nil
	}
	
	// Verify signature
	signatureB64 := sigParams["signature"]
	signature, err := base64.StdEncoding.DecodeString(signatureB64)
	if err != nil {
		result.Reason = "invalid signature encoding"
		return result, nil
	}
	
	valid, err := verifyWithJWK(options.JWK, []byte(signingString), signature)
	if err != nil {
		result.Reason = "signature verification failed"
		return result, nil
	}
	
	result.Valid = valid
	if !valid {
		result.Reason = "signature verification failed"
	}
	
	return result, nil
}

func createDigest(content, algorithm string) (string, error) {
	var hash []byte
	var algName string
	
	switch strings.ToLower(algorithm) {
	case "sha256":
		h := sha256.Sum256([]byte(content))
		hash = h[:]
		algName = "SHA-256"
	case "sha512":
		h := sha512.Sum512([]byte(content))
		hash = h[:]
		algName = "SHA-512"
	default:
		return "", fmt.Errorf("unsupported digest algorithm: %s", algorithm)
	}
	
	return algName + "=" + base64.StdEncoding.EncodeToString(hash), nil
}

func verifyDigest(content, digestHeader string) (bool, error) {
	// Parse digest header (e.g., "SHA-256=X48E9qOokqqrvdts8nOJRJN3OWDUoyWxBf7kbu9DBPE=")
	parts := strings.SplitN(digestHeader, "=", 2)
	if len(parts) != 2 {
		return false, fmt.Errorf("invalid digest header format")
	}
	
	algorithm := strings.ToLower(strings.TrimSpace(parts[0]))
	expectedDigest := strings.TrimSpace(parts[1])
	
	// Map algorithm names
	switch algorithm {
	case "sha-256":
		algorithm = "sha256"
	case "sha-512":
		algorithm = "sha512"
	}
	
	// Create digest for content
	actualDigest, err := createDigest(content, algorithm)
	if err != nil {
		return false, err
	}
	
	// Extract just the base64 part for comparison
	actualParts := strings.SplitN(actualDigest, "=", 2)
	if len(actualParts) != 2 {
		return false, fmt.Errorf("failed to create digest")
	}
	
	return actualParts[1] == expectedDigest, nil
}

func buildSigningString(message *HTTPMessage, headers []string, created, expires int64) (string, error) {
	var lines []string
	
	for _, header := range headers {
		switch header {
		case "(request-target)":
			if message.Type != "request" {
				return "", fmt.Errorf("(request-target) can only be used with requests")
			}
			target := strings.ToLower(message.Method) + " " + message.Path
			lines = append(lines, "(request-target): "+target)
			
		case "(status)":
			if message.Type != "response" {
				return "", fmt.Errorf("(status) can only be used with responses")
			}
			lines = append(lines, "(status): "+strconv.Itoa(message.Status))
			
		case "(created)":
			if created <= 0 {
				return "", fmt.Errorf("(created) header requires created timestamp")
			}
			lines = append(lines, "(created): "+strconv.FormatInt(created, 10))
			
		case "(expires)":
			if expires <= 0 {
				return "", fmt.Errorf("(expires) header requires expires timestamp")
			}
			lines = append(lines, "(expires): "+strconv.FormatInt(expires, 10))
			
		default:
			// Regular header
			value, ok := message.Headers[strings.ToLower(header)]
			if !ok {
				return "", fmt.Errorf("header '%s' not found in message", header)
			}
			lines = append(lines, strings.ToLower(header)+": "+value)
		}
	}
	
	return strings.Join(lines, "\n"), nil
}

func parseSignatureHeader(header string) (map[string]string, error) {
	params := make(map[string]string)
	
	// Split by commas, but handle quoted strings
	parts := strings.Split(header, ",")
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		
		// Find key=value
		eqIdx := strings.Index(part, "=")
		if eqIdx == -1 {
			continue
		}
		
		key := strings.TrimSpace(part[:eqIdx])
		value := strings.TrimSpace(part[eqIdx+1:])
		
		// Remove quotes
		if len(value) >= 2 && value[0] == '"' && value[len(value)-1] == '"' {
			value = value[1 : len(value)-1]
		}
		
		params[key] = value
	}
	
	return params, nil
}

func getSignatureAlgorithm(jwk *JWK) string {
	switch jwk.Alg {
	case "RS256", "RS384", "RS512":
		return "rsa-" + strings.ToLower(jwk.Alg[2:])
	case "PS256", "PS384", "PS512":
		return "rsa-pss-" + strings.ToLower(jwk.Alg[2:])
	case "ES256", "ES384", "ES512":
		return "ecdsa-" + strings.ToLower(jwk.Alg[2:])
	case "EdDSA":
		return "ed25519"
	default:
		return jwk.Alg
	}
}

func luaTableToHTTPMessage(table *lua.LTable) (*HTTPMessage, error) {
	message := &HTTPMessage{
		Headers: make(map[string]string),
	}
	
	table.ForEach(func(key, value lua.LValue) {
		switch key.String() {
		case "type":
			message.Type = value.String()
		case "method":
			message.Method = value.String()
		case "path":
			message.Path = value.String()
		case "status":
			if num, ok := value.(lua.LNumber); ok {
				message.Status = int(num)
			}
		case "body":
			message.Body = value.String()
		case "headers":
			if headersTable, ok := value.(*lua.LTable); ok {
				headersTable.ForEach(func(hkey, hvalue lua.LValue) {
					message.Headers[strings.ToLower(hkey.String())] = hvalue.String()
				})
			}
		}
	})
	
	if message.Type == "" {
		return nil, fmt.Errorf("missing message type")
	}
	
	return message, nil
}

func luaTableToHTTPSignatureOptions(table *lua.LTable) (*HTTPSignatureOptions, error) {
	options := &HTTPSignatureOptions{}
	
	table.ForEach(func(key, value lua.LValue) {
		switch key.String() {
		case "jwk":
			if jwkTable, ok := value.(*lua.LTable); ok {
				jwk, err := luaTableToJWK(jwkTable)
				if err == nil {
					options.JWK = jwk
				}
			}
		case "key_id":
			options.KeyID = value.String()
		case "algorithm":
			options.Algorithm = value.String()
		case "headers":
			if headersTable, ok := value.(*lua.LTable); ok {
				var headers []string
				for i := 1; ; i++ {
					val := headersTable.RawGetInt(i)
					if val == lua.LNil {
						break
					}
					headers = append(headers, val.String())
				}
				options.Headers = headers
			}
		case "created":
			if num, ok := value.(lua.LNumber); ok {
				options.Created = int64(num)
			}
		case "expires":
			if num, ok := value.(lua.LNumber); ok {
				options.Expires = int64(num)
			}
		case "required_headers":
			if headersTable, ok := value.(*lua.LTable); ok {
				var headers []string
				for i := 1; ; i++ {
					val := headersTable.RawGetInt(i)
					if val == lua.LNil {
						break
					}
					headers = append(headers, val.String())
				}
				options.RequiredHeaders = headers
			}
		case "max_age":
			if num, ok := value.(lua.LNumber); ok {
				options.MaxAge = int64(num)
			}
		}
	})
	
	if options.JWK == nil {
		return nil, fmt.Errorf("missing JWK")
	}
	
	return options, nil
}

// Utility functions
func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}

func hasHeader(headers map[string]string, name string) bool {
	_, exists := headers[strings.ToLower(name)]
	return exists
}

`

	tmpl, err := template.New("runtime").Parse(runtimeTemplate)
	if err != nil {
		return err
	}

	mainFile := filepath.Join(tempDir, "main.go")
	f, err := os.Create(mainFile)
	if err != nil {
		return err
	}
	defer f.Close()

	return tmpl.Execute(f, config)
}

func buildExecutableFromRuntime(tempDir string, config *BuildConfig) error {
	goModContent := `module luax-runtime

go 1.21

require (
	github.com/yuin/gopher-lua v1.1.1
	go.etcd.io/bbolt v1.4.1
)
`

	goModPath := filepath.Join(tempDir, "go.mod")
	if err := os.WriteFile(goModPath, []byte(goModContent), 0644); err != nil {
		return err
	}

	// Run go mod tidy to download dependencies
	tidyCmd := exec.Command("go", "mod", "tidy")
	tidyCmd.Dir = tempDir
	if output, err := tidyCmd.CombinedOutput(); err != nil {
		return fmt.Errorf("go mod tidy failed: %w\nOutput: %s", err, output)
	}

	outputPath := config.OutputName
	if config.Target == "windows" {
		outputPath += ".exe"
	}

	cmd := exec.Command("go", "build", "-o", outputPath, ".")
	cmd.Dir = tempDir
	
	// Use environment variables if set, otherwise use current architecture
	targetGOOS := os.Getenv("GOOS")
	targetGOARCH := os.Getenv("GOARCH")
	
	if targetGOOS == "" {
		targetGOOS = config.Target
	}
	
	if targetGOARCH == "" {
		targetGOARCH = runtime.GOARCH
		if config.Target != runtime.GOOS {
			// For cross-compilation to different OS, default to amd64
			targetGOARCH = "amd64"
		}
	}
	
	cmd.Env = append(os.Environ(),
		"GOOS="+targetGOOS,
		"GOARCH="+targetGOARCH,
	)

	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("go build failed: %w\nOutput: %s", err, output)
	}

	builtExecutable := filepath.Join(tempDir, filepath.Base(outputPath))
	if _, err := os.Stat(builtExecutable); err == nil {
		return os.Rename(builtExecutable, outputPath)
	}

	return nil
}

// HTTP Signatures implementation for builder.go
// This is a complete copy of httpsig_functions.go to maintain build/run parity

// HTTPSignatureOptions represents options for HTTP signature creation/verification
type HTTPSignatureOptions struct {
	JWK             *JWK
	KeyID           string
	Algorithm       string
	Headers         []string
	Created         int64
	Expires         int64
	RequiredHeaders []string
	MaxAge          int64
}

// HTTPMessage represents an HTTP request or response for signing
type HTTPMessage struct {
	Type    string            // "request" or "response"
	Method  string            // HTTP method (for requests)
	Path    string            // URL path (for requests)
	Status  int               // Status code (for responses)
	Headers map[string]string // HTTP headers
	Body    string            // Message body
}

// registerHTTPSigModule adds HTTP signature functionality to Lua
func registerHTTPSigModule(L *lua.LState) {
	L.PreloadModule("httpsig", func(L *lua.LState) int {
		httpsigModule := L.NewTable()
		
		L.SetField(httpsigModule, "sign", L.NewFunction(httpsigSign))
		L.SetField(httpsigModule, "verify", L.NewFunction(httpsigVerify))
		L.SetField(httpsigModule, "create_digest", L.NewFunction(httpsigCreateDigest))
		L.SetField(httpsigModule, "verify_digest", L.NewFunction(httpsigVerifyDigest))
		
		L.Push(httpsigModule)
		return 1
	})
}

// httpsigSign signs an HTTP message
func httpsigSign(L *lua.LState) int {
	messageTable := L.ToTable(1)
	optionsTable := L.ToTable(2)
	
	if messageTable == nil {
		L.Push(lua.LNil)
		L.Push(lua.LString("missing message table"))
		return 2
	}
	
	if optionsTable == nil {
		L.Push(lua.LNil)
		L.Push(lua.LString("missing options table"))
		return 2
	}
	
	// Parse message
	message, err := luaTableToHTTPMessage(messageTable)
	if err != nil {
		L.Push(lua.LNil)
		L.Push(lua.LString("invalid message: " + err.Error()))
		return 2
	}
	
	// Parse options
	options, err := luaTableToHTTPSignatureOptions(optionsTable)
	if err != nil {
		L.Push(lua.LNil)
		L.Push(lua.LString("invalid options: " + err.Error()))
		return 2
	}
	
	// Generate signature
	signatureHeader, err := createHTTPSignature(message, options)
	if err != nil {
		L.Push(lua.LNil)
		L.Push(lua.LString("signing failed: " + err.Error()))
		return 2
	}
	
	// Return updated headers with signature
	resultTable := L.NewTable()
	
	newHeaders := L.NewTable()
	
	// Copy all headers from the message (including ones added during signing)
	for key, value := range message.Headers {
		L.SetField(newHeaders, key, lua.LString(value))
	}
	
	// Add signature header
	L.SetField(newHeaders, "signature", lua.LString(signatureHeader))
	
	L.SetField(resultTable, "headers", newHeaders)
	L.Push(resultTable)
	return 1
}

// httpsigVerify verifies an HTTP message signature
func httpsigVerify(L *lua.LState) int {
	messageTable := L.ToTable(1)
	optionsTable := L.ToTable(2)
	
	if messageTable == nil || optionsTable == nil {
		L.Push(lua.LBool(false))
		L.Push(lua.LString("missing message or options"))
		return 2
	}
	
	// Parse message
	message, err := luaTableToHTTPMessage(messageTable)
	if err != nil {
		L.Push(lua.LBool(false))
		L.Push(lua.LString("invalid message: " + err.Error()))
		return 2
	}
	
	// Parse options
	options, err := luaTableToHTTPSignatureOptions(optionsTable)
	if err != nil {
		L.Push(lua.LBool(false))
		L.Push(lua.LString("invalid options: " + err.Error()))
		return 2
	}
	
	// Verify signature
	result, err := verifyHTTPSignature(message, options)
	if err != nil {
		L.Push(lua.LBool(false))
		L.Push(lua.LString("verification failed: " + err.Error()))
		return 2
	}
	
	// Return verification result
	resultTable := L.NewTable()
	L.SetField(resultTable, "valid", lua.LBool(result.Valid))
	L.SetField(resultTable, "key_id", lua.LString(result.KeyID))
	L.SetField(resultTable, "algorithm", lua.LString(result.Algorithm))
	if result.Reason != "" {
		L.SetField(resultTable, "reason", lua.LString(result.Reason))
	}
	
	L.Push(resultTable)
	return 1
}

// httpsigCreateDigest creates a digest header for body content
func httpsigCreateDigest(L *lua.LState) int {
	content := L.ToString(1)
	algorithm := L.ToString(2)
	
	if algorithm == "" {
		algorithm = "sha256" // default
	}
	
	digest, err := createDigest(content, algorithm)
	if err != nil {
		L.Push(lua.LNil)
		L.Push(lua.LString("digest creation failed: " + err.Error()))
		return 2
	}
	
	L.Push(lua.LString(digest))
	return 1
}

// httpsigVerifyDigest verifies a digest header against content
func httpsigVerifyDigest(L *lua.LState) int {
	content := L.ToString(1)
	digestHeader := L.ToString(2)
	
	if content == "" || digestHeader == "" {
		L.Push(lua.LBool(false))
		L.Push(lua.LString("missing content or digest header"))
		return 2
	}
	
	valid, err := verifyDigest(content, digestHeader)
	if err != nil {
		L.Push(lua.LBool(false))
		L.Push(lua.LString("digest verification failed: " + err.Error()))
		return 2
	}
	
	L.Push(lua.LBool(valid))
	return 1
}

// HTTP Signature implementation functions

// HTTPSignatureResult represents the result of signature verification
type HTTPSignatureResult struct {
	Valid     bool
	KeyID     string
	Algorithm string
	Reason    string
}

// createHTTPSignature creates an HTTP signature header
func createHTTPSignature(message *HTTPMessage, options *HTTPSignatureOptions) (string, error) {
	// Add date header if not present
	if !hasHeader(message.Headers, "date") {
		message.Headers["date"] = time.Now().UTC().Format(time.RFC1123)
	}
	
	// Determine headers to sign
	headersToSign := options.Headers
	if len(headersToSign) == 0 {
		// Default headers based on message type
		if message.Type == "request" {
			headersToSign = []string{"(request-target)", "host", "date"}
		} else {
			headersToSign = []string{"(status)", "content-type", "date"}
		}
	}
	
	// Create digest header if needed
	if contains(headersToSign, "digest") && !hasHeader(message.Headers, "digest") {
		if message.Body == "" {
			// Empty body gets empty digest
			digest, err := createDigest("", "sha256")
			if err != nil {
				return "", fmt.Errorf("failed to create digest: %w", err)
			}
			message.Headers["digest"] = digest
		} else {
			digest, err := createDigest(message.Body, "sha256")
			if err != nil {
				return "", fmt.Errorf("failed to create digest: %w", err)
			}
			message.Headers["digest"] = digest
		}
	}
	
	// Add digest header if body is present and not already included in signing
	if message.Body != "" && !contains(headersToSign, "digest") {
		headersToSign = append(headersToSign, "digest")
		
		// Create digest if not present
		if !hasHeader(message.Headers, "digest") {
			digest, err := createDigest(message.Body, "sha256")
			if err != nil {
				return "", fmt.Errorf("failed to create digest: %w", err)
			}
			message.Headers["digest"] = digest
		}
	}
	
	// Build signing string
	signingString, err := buildSigningString(message, headersToSign, options.Created, options.Expires)
	if err != nil {
		return "", fmt.Errorf("failed to build signing string: %w", err)
	}
	
	// Sign the string
	signature, err := signWithJWK(options.JWK, []byte(signingString))
	if err != nil {
		return "", fmt.Errorf("failed to sign: %w", err)
	}
	
	// Build signature header
	signatureB64 := base64.StdEncoding.EncodeToString(signature)
	
	var parts []string
	parts = append(parts, fmt.Sprintf(`keyId="%s"`, options.KeyID))
	parts = append(parts, fmt.Sprintf(`algorithm="%s"`, getSignatureAlgorithm(options.JWK)))
	parts = append(parts, fmt.Sprintf(`headers="%s"`, strings.Join(headersToSign, " ")))
	
	if options.Created > 0 {
		parts = append(parts, fmt.Sprintf(`created=%d`, options.Created))
	}
	if options.Expires > 0 {
		parts = append(parts, fmt.Sprintf(`expires=%d`, options.Expires))
	}
	
	parts = append(parts, fmt.Sprintf(`signature="%s"`, signatureB64))
	
	return strings.Join(parts, ","), nil
}

// verifyHTTPSignature verifies an HTTP signature
func verifyHTTPSignature(message *HTTPMessage, options *HTTPSignatureOptions) (*HTTPSignatureResult, error) {
	result := &HTTPSignatureResult{}
	
	// Parse signature header
	signatureHeader, ok := message.Headers["signature"]
	if !ok {
		result.Reason = "missing signature header"
		return result, nil
	}
	
	sigParams, err := parseSignatureHeader(signatureHeader)
	if err != nil {
		result.Reason = "invalid signature header format"
		return result, nil
	}
	
	result.KeyID = sigParams["keyId"]
	result.Algorithm = sigParams["algorithm"]
	
	// Check required headers
	signedHeaders := strings.Fields(sigParams["headers"])
	for _, required := range options.RequiredHeaders {
		if !contains(signedHeaders, required) {
			result.Reason = fmt.Sprintf("required header '%s' not signed", required)
			return result, nil
		}
	}
	
	// Validate digest if digest header was signed
	if contains(signedHeaders, "digest") {
		digestHeader, ok := message.Headers["digest"]
		if !ok {
			result.Reason = "digest header missing but was signed"
			return result, nil
		}
		
		// Verify digest matches the body content
		digestValid, err := verifyDigest(message.Body, digestHeader)
		if err != nil {
			result.Reason = "digest verification failed: " + err.Error()
			return result, nil
		}
		
		if !digestValid {
			result.Reason = "digest mismatch - body content has been tampered"
			return result, nil
		}
	}
	
	// Check expiration
	if created, ok := sigParams["created"]; ok {
		createdTime, err := strconv.ParseInt(created, 10, 64)
		if err != nil {
			result.Reason = "invalid created timestamp"
			return result, nil
		}
		
		if options.MaxAge > 0 && time.Now().Unix()-createdTime > options.MaxAge {
			result.Reason = "signature expired"
			return result, nil
		}
	}
	
	if expires, ok := sigParams["expires"]; ok {
		expiresTime, err := strconv.ParseInt(expires, 10, 64)
		if err != nil {
			result.Reason = "invalid expires timestamp"
			return result, nil
		}
		
		if time.Now().Unix() > expiresTime {
			result.Reason = "signature expired"
			return result, nil
		}
	}
	
	// Build signing string
	created, _ := strconv.ParseInt(sigParams["created"], 10, 64)
	expires, _ := strconv.ParseInt(sigParams["expires"], 10, 64)
	
	signingString, err := buildSigningString(message, signedHeaders, created, expires)
	if err != nil {
		result.Reason = "failed to build signing string"
		return result, nil
	}
	
	// Verify signature
	signatureB64 := sigParams["signature"]
	signature, err := base64.StdEncoding.DecodeString(signatureB64)
	if err != nil {
		result.Reason = "invalid signature encoding"
		return result, nil
	}
	
	valid, err := verifyWithJWK(options.JWK, []byte(signingString), signature)
	if err != nil {
		result.Reason = "signature verification failed"
		return result, nil
	}
	
	result.Valid = valid
	if !valid {
		result.Reason = "signature verification failed"
	}
	
	return result, nil
}

// buildSigningString constructs the string to be signed
func buildSigningString(message *HTTPMessage, headers []string, created, expires int64) (string, error) {
	var lines []string
	
	for _, header := range headers {
		switch header {
		case "(request-target)":
			if message.Type != "request" {
				return "", fmt.Errorf("(request-target) can only be used with requests")
			}
			target := strings.ToLower(message.Method) + " " + message.Path
			lines = append(lines, "(request-target): "+target)
			
		case "(status)":
			if message.Type != "response" {
				return "", fmt.Errorf("(status) can only be used with responses")
			}
			lines = append(lines, "(status): "+strconv.Itoa(message.Status))
			
		case "(created)":
			if created <= 0 {
				return "", fmt.Errorf("(created) header requires created timestamp")
			}
			lines = append(lines, "(created): "+strconv.FormatInt(created, 10))
			
		case "(expires)":
			if expires <= 0 {
				return "", fmt.Errorf("(expires) header requires expires timestamp")
			}
			lines = append(lines, "(expires): "+strconv.FormatInt(expires, 10))
			
		default:
			// Regular header
			value, ok := message.Headers[strings.ToLower(header)]
			if !ok {
				return "", fmt.Errorf("header '%s' not found in message", header)
			}
			lines = append(lines, strings.ToLower(header)+": "+value)
		}
	}
	
	return strings.Join(lines, "\n"), nil
}

// parseSignatureHeader parses HTTP signature header into components
func parseSignatureHeader(header string) (map[string]string, error) {
	params := make(map[string]string)
	
	// Split by commas, but handle quoted strings
	parts := strings.Split(header, ",")
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		
		// Find key=value
		eqIdx := strings.Index(part, "=")
		if eqIdx == -1 {
			continue
		}
		
		key := strings.TrimSpace(part[:eqIdx])
		value := strings.TrimSpace(part[eqIdx+1:])
		
		// Remove quotes
		if len(value) >= 2 && value[0] == '"' && value[len(value)-1] == '"' {
			value = value[1 : len(value)-1]
		}
		
		params[key] = value
	}
	
	return params, nil
}

// createDigest creates a digest header for content
func createDigest(content, algorithm string) (string, error) {
	var hash []byte
	var algName string
	
	switch strings.ToLower(algorithm) {
	case "sha256":
		h := sha256.Sum256([]byte(content))
		hash = h[:]
		algName = "SHA-256"
	case "sha512":
		h := sha512.Sum512([]byte(content))
		hash = h[:]
		algName = "SHA-512"
	default:
		return "", fmt.Errorf("unsupported digest algorithm: %s", algorithm)
	}
	
	return algName + "=" + base64.StdEncoding.EncodeToString(hash), nil
}

// verifyDigest verifies a digest header against content
func verifyDigest(content, digestHeader string) (bool, error) {
	// Parse digest header (e.g., "SHA-256=X48E9qOokqqrvdts8nOJRJN3OWDUoyWxBf7kbu9DBPE=")
	parts := strings.SplitN(digestHeader, "=", 2)
	if len(parts) != 2 {
		return false, fmt.Errorf("invalid digest header format")
	}
	
	algorithm := strings.ToLower(strings.TrimSpace(parts[0]))
	expectedDigest := strings.TrimSpace(parts[1])
	
	// Map algorithm names
	switch algorithm {
	case "sha-256":
		algorithm = "sha256"
	case "sha-512":
		algorithm = "sha512"
	}
	
	// Create digest for content
	actualDigest, err := createDigest(content, algorithm)
	if err != nil {
		return false, err
	}
	
	// Extract just the base64 part for comparison
	actualParts := strings.SplitN(actualDigest, "=", 2)
	if len(actualParts) != 2 {
		return false, fmt.Errorf("failed to create digest")
	}
	
	return actualParts[1] == expectedDigest, nil
}

// getSignatureAlgorithm maps JWK algorithm to HTTP signature algorithm
func getSignatureAlgorithm(jwk *JWK) string {
	switch jwk.Alg {
	case "RS256", "RS384", "RS512":
		return "rsa-" + strings.ToLower(jwk.Alg[2:])
	case "PS256", "PS384", "PS512":
		return "rsa-pss-" + strings.ToLower(jwk.Alg[2:])
	case "ES256", "ES384", "ES512":
		return "ecdsa-" + strings.ToLower(jwk.Alg[2:])
	case "EdDSA":
		return "ed25519"
	default:
		return jwk.Alg
	}
}

// Helper functions for Lua table conversion

func luaTableToHTTPMessage(table *lua.LTable) (*HTTPMessage, error) {
	message := &HTTPMessage{
		Headers: make(map[string]string),
	}
	
	table.ForEach(func(key, value lua.LValue) {
		switch key.String() {
		case "type":
			message.Type = value.String()
		case "method":
			message.Method = value.String()
		case "path":
			message.Path = value.String()
		case "status":
			if num, ok := value.(lua.LNumber); ok {
				message.Status = int(num)
			}
		case "body":
			message.Body = value.String()
		case "headers":
			if headersTable, ok := value.(*lua.LTable); ok {
				headersTable.ForEach(func(hkey, hvalue lua.LValue) {
					message.Headers[strings.ToLower(hkey.String())] = hvalue.String()
				})
			}
		}
	})
	
	if message.Type == "" {
		return nil, fmt.Errorf("missing message type")
	}
	
	return message, nil
}

func luaTableToHTTPSignatureOptions(table *lua.LTable) (*HTTPSignatureOptions, error) {
	options := &HTTPSignatureOptions{}
	
	table.ForEach(func(key, value lua.LValue) {
		switch key.String() {
		case "jwk":
			if jwkTable, ok := value.(*lua.LTable); ok {
				jwk, err := luaTableToJWK(jwkTable)
				if err == nil {
					options.JWK = jwk
				}
			}
		case "key_id":
			options.KeyID = value.String()
		case "algorithm":
			options.Algorithm = value.String()
		case "headers":
			if headersTable, ok := value.(*lua.LTable); ok {
				var headers []string
				for i := 1; ; i++ {
					val := headersTable.RawGetInt(i)
					if val == lua.LNil {
						break
					}
					headers = append(headers, val.String())
				}
				options.Headers = headers
			}
		case "created":
			if num, ok := value.(lua.LNumber); ok {
				options.Created = int64(num)
			}
		case "expires":
			if num, ok := value.(lua.LNumber); ok {
				options.Expires = int64(num)
			}
		case "required_headers":
			if headersTable, ok := value.(*lua.LTable); ok {
				var headers []string
				for i := 1; ; i++ {
					val := headersTable.RawGetInt(i)
					if val == lua.LNil {
						break
					}
					headers = append(headers, val.String())
				}
				options.RequiredHeaders = headers
			}
		case "max_age":
			if num, ok := value.(lua.LNumber); ok {
				options.MaxAge = int64(num)
			}
		}
	})
	
	if options.JWK == nil {
		return nil, fmt.Errorf("missing JWK")
	}
	// Key ID is optional for verification - can be extracted from signature header
	
	return options, nil
}

// Utility functions
func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}

func hasHeader(headers map[string]string, name string) bool {
	_, exists := headers[strings.ToLower(name)]
	return exists
}