package main

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/yuin/gopher-lua"
	"github.com/spf13/cobra"
)

var replCmd = &cobra.Command{
	Use:   "repl",
	Short: "Start an interactive Lua REPL",
	Long:  `Start an interactive Lua Read-Eval-Print Loop (REPL).

Provides a simple command-line REPL with:
- Interactive Lua evaluation
- Persistent session state across expressions
- Access to all Hype modules (http, kv, crypto, ws)
- Multiline expression support
- Error handling and return value display

Controls:
- Enter: Execute the current expression
- Ctrl+C: Exit the REPL
- Use backslash (\) at end of line for multiline expressions

Note: TUI mode has been removed. Use the simple command-line REPL only.`,
	Run: func(cmd *cobra.Command, args []string) {
		// TUI REPL is no longer supported, always use simple mode
		if err := runSimpleREPL(); err != nil {
			fmt.Fprintf(os.Stderr, "Error running REPL: %v\n", err)
			os.Exit(1)
		}
	},
}

func init() {
	// No flags needed - TUI mode removed
}

func runSimpleREPL() error {
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
	
	return runSimpleREPLWithState(L)
}

// TUI REPL removed - TUI functionality no longer supported

// Simple REPL implementation
func runSimpleREPLWithState(L *lua.LState) error {
	fmt.Println("Hype Lua REPL v1.9.0 (Simple Mode)")
	fmt.Println("Type expressions and press Enter. Use Ctrl+C to exit.")
	fmt.Println()
	fmt.Println("Available modules: tui, http, kv, crypto, ws")
	fmt.Println("Multiline: Use '\\' at end of line or let incomplete statements continue")
	fmt.Println()

	// Simple line-by-line REPL with multiline support
	scanner := bufio.NewScanner(os.Stdin)
	var buffer string
	var prompt string
	
	for {
		if buffer == "" {
			prompt = "hype> "
		} else {
			prompt = "....> "
		}
		fmt.Print(prompt)
		
		if !scanner.Scan() {
			break
		}
		
		line := scanner.Text()
		
		// Check for explicit line continuation
		if strings.HasSuffix(line, "\\") {
			buffer += strings.TrimSuffix(line, "\\") + "\n"
			continue
		}
		
		// Add line to buffer
		if buffer != "" {
			buffer += line + "\n"
		} else {
			buffer = line
		}
		
		// Check if the statement is complete
		_, err := L.LoadString(buffer)
		if err != nil && strings.Contains(err.Error(), "<eof>") {
			// Incomplete statement, continue collecting lines
			buffer += "\n"
			continue
		}
		
		// Statement is complete (or has a different error), execute it
		code := buffer
		buffer = "" // Reset buffer
		
		if code == "" {
			continue
		}
		
		// Save the current stack size
		oldTop := L.GetTop()
		
		// Try to execute as expression first (for return values)
		err = L.DoString("return " + code)
		hasReturnValue := false
		
		if err != nil {
			// If that fails, try as statement
			L.SetTop(oldTop) // Restore stack
			err = L.DoString(code)
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
						// For tables, just show it's a table
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
	}
	
	return scanner.Err()
}
