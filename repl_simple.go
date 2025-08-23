package main

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/yuin/gopher-lua"
	"github.com/spf13/cobra"
)

var replSimpleCmd = &cobra.Command{
	Use:   "repl-simple",
	Short: "Start a simple interactive Lua REPL",
	Long:  `Start a simple interactive Lua Read-Eval-Print Loop.`,
	Run: func(cmd *cobra.Command, args []string) {
		if err := runSimpleREPLDirect(); err != nil {
			fmt.Fprintf(os.Stderr, "Error running REPL: %v\n", err)
			os.Exit(1)
		}
	},
}

func runSimpleREPLDirect() error {
	// Create Lua state with all modules
	L := lua.NewState()
	defer L.Close()

	// Open standard libraries
	lua.OpenBase(L)
	lua.OpenPackage(L)
	lua.OpenCoroutine(L)
	lua.OpenTable(L)
	lua.OpenIo(L)
	lua.OpenOs(L)
	lua.OpenString(L)
	lua.OpenMath(L)
	lua.OpenDebug(L)

	// Register all modules
	RegisterHTTPModule(L)
	registerKVModule(L)
	registerTUIFunctions(L)
	registerCryptoModule(L)
	registerHTTPSigModule(L)
	registerWebSocketStub(L)

	fmt.Println("🚀 Hype Lua REPL v1.8.0")
	fmt.Println("========================")
	fmt.Println()
	fmt.Println("Available modules: tui, http, kv, crypto, ws")
	fmt.Println("Type expressions and press Enter. Use Ctrl+C to exit.")
	fmt.Println("Multiline: Use '\\' at end of line or let incomplete statements continue")
	fmt.Println()

	// Simple line-by-line REPL
	scanner := bufio.NewScanner(os.Stdin)
	fmt.Print("hype> ")
	
	for scanner.Scan() {
		line := scanner.Text()
		
		if line == "" {
			fmt.Print("hype> ")
			continue
		}
		
		// Save the current stack size
		oldTop := L.GetTop()
		
		// Try to execute as expression first (for return values)
		err := L.DoString("return " + line)
		hasReturnValue := false
		
		if err != nil {
			// If that fails, try as statement
			L.SetTop(oldTop) // Restore stack
			err = L.DoString(line)
		} else {
			hasReturnValue = true
		}
		
		if err != nil {
			fmt.Println("Error:", err)
		} else if hasReturnValue {
			// Print any NEW return values (after oldTop)
			n := L.GetTop()
			if n > oldTop {
				results := []string{}
				for i := oldTop + 1; i <= n; i++ {
					lv := L.Get(i)
					var result string
					switch lv.Type() {
					case lua.LTNil:
						result = "nil"
					case lua.LTBool:
						result = fmt.Sprintf("%v", lua.LVAsBool(lv))
					case lua.LTNumber:
						result = fmt.Sprintf("%v", lua.LVAsNumber(lv))
					case lua.LTString:
						result = lua.LVAsString(lv)
					case lua.LTTable:
						result = "<table>"
					case lua.LTFunction:
						result = "<function>"
					default:
						result = fmt.Sprintf("<%s>", lv.Type().String())
					}
					results = append(results, result)
				}
				fmt.Println(strings.Join(results, "\t"))
			}
			L.SetTop(oldTop) // Restore stack
		}
		
		fmt.Print("hype> ")
	}
	
	return scanner.Err()
}