package main

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/yuin/gopher-lua"
	"go.etcd.io/bbolt"
)

func runScript(scriptPath string, scriptArgs []string) error {
	return runScriptWithPlugins(scriptPath, scriptArgs, []PluginSpec{})
}

func runScriptWithPlugins(scriptPath string, scriptArgs []string, pluginSpecs []PluginSpec) error {
	scriptContent, err := os.ReadFile(scriptPath)
	if err != nil {
		return fmt.Errorf("failed to read script file: %w", err)
	}

	// Load plugins
	registry := NewPluginRegistry()
	if len(pluginSpecs) > 0 {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
		defer cancel()
		
		if err := registry.LoadPlugins(ctx, pluginSpecs); err != nil {
			return fmt.Errorf("failed to load plugins: %w", err)
		}
		defer registry.Close()
	}

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
	setupCommandLineArgs(L, scriptPath, scriptArgs)
	
	// Register built-in modules
	RegisterHTTPModule(L)
	registerKVModule(L)
	registerTUIFunctions(L)
	registerCryptoModule(L)
	registerHTTPSigModule(L)
	registerWebSocketStub(L)

	// Register plugin modules
	if err := registry.RegisterAll(L); err != nil {
		return fmt.Errorf("failed to register plugins: %w", err)
	}

	if err := L.DoString(string(scriptContent)); err != nil {
		return fmt.Errorf("lua runtime error: %w", err)
	}

	return nil
}

func setupCommandLineArgs(L *lua.LState, scriptPath string, scriptArgs []string) {
	// Create arg table (following Lua convention)
	argTable := L.NewTable()
	
	// arg[0] is the script name (standard Lua convention)
	argTable.RawSetInt(0, lua.LString(scriptPath))
	
	// arg[1], arg[2], etc. are the script arguments
	for i, arg := range scriptArgs {
		argTable.RawSetInt(i+1, lua.LString(arg))
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

// TUI Constructor Functions removed - TUI functionality no longer supported





// TUI Method Handlers removed - TUI functionality no longer supported

// textViewIndex removed - TUI functionality no longer supported

// inputFieldIndex removed - TUI functionality no longer supported

// buttonIndex removed - TUI functionality no longer supported

// flexIndex removed - TUI functionality no longer supported

// eventIndex removed - TUI functionality no longer supported

// HTTP Module

// KV Database Module
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
	
	// Set up transaction metatable
	txnMT := L.NewTypeMetatable("KVTxn")
	L.SetField(txnMT, "__index", L.NewFunction(kvTxnIndex))
	
	// Set up cursor metatable
	cursorMT := L.NewTypeMetatable("KVCursor")
	L.SetField(cursorMT, "__index", L.NewFunction(kvCursorIndex))
}

type KVDB struct {
	db *bbolt.DB
}

type KVTxn struct {
	txn *bbolt.Tx
}

type KVCursor struct {
	cursor *bbolt.Cursor
	bucket string
}

func kvOpen(L *lua.LState) int {
	path := L.CheckString(1)
	
	db, err := bbolt.Open(path, 0600, nil)
	if err != nil {
		L.Push(lua.LNil)
		L.Push(lua.LString(err.Error()))
		return 2
	}
	
	kvdb := &KVDB{db: db}
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
			bucketName := L.CheckString(2)
			
			err := db.db.Update(func(tx *bbolt.Tx) error {
				_, err := tx.CreateBucketIfNotExists([]byte(bucketName))
				return err
			})
			
			if err != nil {
				L.Push(lua.LString(err.Error()))
				return 1
			}
			return 0
		}))
	case "put":
		L.Push(L.NewFunction(func(L *lua.LState) int {
			bucketName := L.CheckString(2)
			key := L.CheckString(3)
			value := L.CheckString(4)
			
			err := db.db.Update(func(tx *bbolt.Tx) error {
				bucket := tx.Bucket([]byte(bucketName))
				if bucket == nil {
					return fmt.Errorf("bucket %s does not exist", bucketName)
				}
				return bucket.Put([]byte(key), []byte(value))
			})
			
			if err != nil {
				L.Push(lua.LString(err.Error()))
				return 1
			}
			return 0
		}))
	case "get":
		L.Push(L.NewFunction(func(L *lua.LState) int {
			bucketName := L.CheckString(2)
			key := L.CheckString(3)
			
			var value []byte
			err := db.db.View(func(tx *bbolt.Tx) error {
				bucket := tx.Bucket([]byte(bucketName))
				if bucket == nil {
					return fmt.Errorf("bucket %s does not exist", bucketName)
				}
				value = bucket.Get([]byte(key))
				return nil
			})
			
			if err != nil {
				L.Push(lua.LNil)
				L.Push(lua.LString(err.Error()))
				return 2
			}
			
			if value == nil {
				L.Push(lua.LNil)
				L.Push(lua.LNil)
			} else {
				L.Push(lua.LString(string(value)))
				L.Push(lua.LNil)
			}
			return 2
		}))
	case "delete":
		L.Push(L.NewFunction(func(L *lua.LState) int {
			bucketName := L.CheckString(2)
			key := L.CheckString(3)
			
			err := db.db.Update(func(tx *bbolt.Tx) error {
				bucket := tx.Bucket([]byte(bucketName))
				if bucket == nil {
					return fmt.Errorf("bucket %s does not exist", bucketName)
				}
				return bucket.Delete([]byte(key))
			})
			
			if err != nil {
				L.Push(lua.LString(err.Error()))
				return 1
			}
			return 0
		}))
	case "begin_txn":
		L.Push(L.NewFunction(func(L *lua.LState) int {
			writable := !L.OptBool(2, false)
			
			tx, err := db.db.Begin(writable)
			if err != nil {
				L.Push(lua.LNil)
				L.Push(lua.LString(err.Error()))
				return 2
			}
			
			kvtxn := &KVTxn{txn: tx}
			ud := L.NewUserData()
			ud.Value = kvtxn
			L.SetMetatable(ud, L.GetTypeMetatable("KVTxn"))
			L.Push(ud)
			L.Push(lua.LNil)
			return 2
		}))
	case "keys":
		L.Push(L.NewFunction(func(L *lua.LState) int {
			bucketName := L.CheckString(2)
			prefix := L.OptString(3, "")
			
			var keys []string
			err := db.db.View(func(tx *bbolt.Tx) error {
				bucket := tx.Bucket([]byte(bucketName))
				if bucket == nil {
					return fmt.Errorf("bucket %s does not exist", bucketName)
				}
				
				cursor := bucket.Cursor()
				for k, _ := cursor.First(); k != nil; k, _ = cursor.Next() {
					keyStr := string(k)
					if prefix == "" || strings.HasPrefix(keyStr, prefix) {
						keys = append(keys, keyStr)
					}
				}
				return nil
			})
			
			if err != nil {
				L.Push(lua.LNil)
				L.Push(lua.LString(err.Error()))
				return 2
			}
			
			// Convert to Lua table
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
			bucketName := L.CheckString(2)
			callback := L.CheckFunction(3)
			
			err := db.db.View(func(tx *bbolt.Tx) error {
				bucket := tx.Bucket([]byte(bucketName))
				if bucket == nil {
					return fmt.Errorf("bucket %s does not exist", bucketName)
				}
				
				cursor := bucket.Cursor()
				for k, v := cursor.First(); k != nil; k, v = cursor.Next() {
					L.Push(callback)
					L.Push(lua.LString(string(k)))
					L.Push(lua.LString(string(v)))
					L.Call(2, 1)
					
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
			return 0
		}))
	case "close":
		L.Push(L.NewFunction(func(L *lua.LState) int {
			if db.db != nil {
				db.db.Close()
				db.db = nil
			}
			return 0
		}))
	}
	
	return 1
}

func kvTxnIndex(L *lua.LState) int {
	ud := L.CheckUserData(1)
	txn := ud.Value.(*KVTxn)
	method := L.CheckString(2)
	
	switch method {
	case "put":
		L.Push(L.NewFunction(func(L *lua.LState) int {
			bucketName := L.CheckString(2)
			key := L.CheckString(3)
			value := L.CheckString(4)
			
			bucket := txn.txn.Bucket([]byte(bucketName))
			if bucket == nil {
				L.Push(lua.LString(fmt.Sprintf("bucket %s does not exist", bucketName)))
				return 1
			}
			
			err := bucket.Put([]byte(key), []byte(value))
			if err != nil {
				L.Push(lua.LString(err.Error()))
				return 1
			}
			return 0
		}))
	case "get":
		L.Push(L.NewFunction(func(L *lua.LState) int {
			bucketName := L.CheckString(2)
			key := L.CheckString(3)
			
			bucket := txn.txn.Bucket([]byte(bucketName))
			if bucket == nil {
				L.Push(lua.LNil)
				L.Push(lua.LString(fmt.Sprintf("bucket %s does not exist", bucketName)))
				return 2
			}
			
			value := bucket.Get([]byte(key))
			if value == nil {
				L.Push(lua.LNil)
				L.Push(lua.LNil)
			} else {
				L.Push(lua.LString(string(value)))
				L.Push(lua.LNil)
			}
			return 2
		}))
	case "delete":
		L.Push(L.NewFunction(func(L *lua.LState) int {
			bucketName := L.CheckString(2)
			key := L.CheckString(3)
			
			bucket := txn.txn.Bucket([]byte(bucketName))
			if bucket == nil {
				L.Push(lua.LString(fmt.Sprintf("bucket %s does not exist", bucketName)))
				return 1
			}
			
			err := bucket.Delete([]byte(key))
			if err != nil {
				L.Push(lua.LString(err.Error()))
				return 1
			}
			return 0
		}))
	case "commit":
		L.Push(L.NewFunction(func(L *lua.LState) int {
			err := txn.txn.Commit()
			if err != nil {
				L.Push(lua.LString(err.Error()))
				return 1
			}
			return 0
		}))
	case "abort":
		L.Push(L.NewFunction(func(L *lua.LState) int {
			err := txn.txn.Rollback()
			if err != nil {
				L.Push(lua.LString(err.Error()))
				return 1
			}
			return 0
		}))
	}
	
	return 1
}

func kvCursorIndex(L *lua.LState) int {
	ud := L.CheckUserData(1)
	cursor := ud.Value.(*KVCursor)
	method := L.CheckString(2)
	
	switch method {
	case "first":
		L.Push(L.NewFunction(func(L *lua.LState) int {
			k, v := cursor.cursor.First()
			if k == nil {
				L.Push(lua.LNil)
				L.Push(lua.LNil)
			} else {
				L.Push(lua.LString(string(k)))
				L.Push(lua.LString(string(v)))
			}
			return 2
		}))
	case "last":
		L.Push(L.NewFunction(func(L *lua.LState) int {
			k, v := cursor.cursor.Last()
			if k == nil {
				L.Push(lua.LNil)
				L.Push(lua.LNil)
			} else {
				L.Push(lua.LString(string(k)))
				L.Push(lua.LString(string(v)))
			}
			return 2
		}))
	case "seek":
		L.Push(L.NewFunction(func(L *lua.LState) int {
			seek := L.CheckString(2)
			k, v := cursor.cursor.Seek([]byte(seek))
			if k == nil {
				L.Push(lua.LNil)
				L.Push(lua.LNil)
			} else {
				L.Push(lua.LString(string(k)))
				L.Push(lua.LString(string(v)))
			}
			return 2
		}))
	}
	
	return 1
}

// WebSocket stub - functionality removed
func registerWebSocketStub(L *lua.LState) {
	L.PreloadModule("websocket", func(L *lua.LState) int {
		L.RaiseError("WebSocket module has been removed from Hype. Use HTTP or other alternatives.")
		return 0
	})
}

