package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/chzyer/readline"
	"github.com/spf13/cobra"
	"github.com/yuin/gopher-lua"
)

var replCmd = &cobra.Command{
	Use:   "repl",
	Short: "Start an interactive Lua REPL with command history and custom prompts",
	Long: `Start an interactive Lua Read-Eval-Print Loop (REPL) with enhanced features.

Features:
- Interactive Lua evaluation with command history
- Arrow key navigation (↑/↓) through command history
- Tab completion for Lua keywords and Hype modules
- Custom prompt support via ~/.config/hype/repl.lua
- Persistent session state across expressions
- Access to all Hype modules (http, kv, crypto, ws)
- Multiline expression support
- Persistent history storage (~/.hype_history)
- Error handling and return value display

Controls:
- Enter: Execute the current expression
- ↑/↓: Navigate command history
- Tab: Auto-complete commands
- Ctrl+C: Exit the REPL (or clear current multiline input)
- Use backslash (\) at end of line for multiline expressions

Custom Prompts:
Use 'hype repl-config init [example]' to set up a custom prompt.
Available examples: basic-colored, minimalist, rich-context, arrow-style`,
	Run: func(cmd *cobra.Command, args []string) {
		if err := runSimpleREPL(); err != nil {
			fmt.Fprintf(os.Stderr, "Error running REPL: %v\n", err)
			os.Exit(1)
		}
	},
}

var replConfigCmd = &cobra.Command{
	Use:   "repl-config",
	Short: "Manage REPL configuration and custom prompts",
	Long: `Manage REPL configuration including custom prompt setup.

Subcommands:
  init [example]  - Initialize REPL config with optional example prompt
  list           - List available example prompts
  path           - Show the config file path`,
}

var replConfigInitCmd = &cobra.Command{
	Use:   "init [example]",
	Short: "Initialize REPL config directory and optionally copy an example prompt",
	Long: `Initialize the REPL configuration directory at ~/.config/hype/

Available examples:
  basic-colored  - Colored prompt with line numbers (default)
  minimalist     - Clean, minimal prompt
  rich-context   - Rich prompt with timestamp and status
  arrow-style    - Modern arrow-style prompt`,
	Run: func(cmd *cobra.Command, args []string) {
		example := "basic-colored"
		if len(args) > 0 {
			example = args[0]
		}

		if err := initREPLConfig(example); err != nil {
			fmt.Fprintf(os.Stderr, "Error initializing REPL config: %v\n", err)
			os.Exit(1)
		}
	},
}

var replConfigListCmd = &cobra.Command{
	Use:   "list",
	Short: "List available example prompt configurations",
	Run: func(cmd *cobra.Command, args []string) {
		listPromptExamples()
	},
}

var replConfigPathCmd = &cobra.Command{
	Use:   "path",
	Short: "Show the path to the REPL configuration file",
	Run: func(cmd *cobra.Command, args []string) {
		homeDir, err := os.UserHomeDir()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error getting home directory: %v\n", err)
			os.Exit(1)
		}

		configPath := filepath.Join(homeDir, ".config", "hype", "repl.lua")
		fmt.Println(configPath)
	},
}

func init() {
	// Add subcommands to repl-config
	replConfigCmd.AddCommand(replConfigInitCmd)
	replConfigCmd.AddCommand(replConfigListCmd)
	replConfigCmd.AddCommand(replConfigPathCmd)
}

// PromptContext contains information available to custom prompt functions
type PromptContext struct {
	IsMultiline   bool
	LineCount     int
	HistoryLength int
	LastCommand   string
}

// loadCustomPromptConfig attempts to load a custom prompt function from ~/.config/hype/repl.lua
func loadCustomPromptConfig() (*lua.LFunction, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return nil, err
	}

	configPath := filepath.Join(homeDir, ".config", "hype", "repl.lua")

	// Check if config file exists
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		return nil, nil // No custom config, use default
	}

	// Create new Lua state for loading config
	L := lua.NewState()
	defer L.Close()

	// Load the config file
	if err := L.DoFile(configPath); err != nil {
		return nil, fmt.Errorf("error loading repl config: %w", err)
	}

	// Look for a "prompt" function
	promptFunc := L.GetGlobal("prompt")
	if promptFunc == lua.LNil {
		return nil, nil // No prompt function defined
	}

	if promptFunc.Type() != lua.LTFunction {
		return nil, fmt.Errorf("prompt must be a function")
	}

	return promptFunc.(*lua.LFunction), nil
}

// evaluateCustomPrompt evaluates a custom prompt function with the given context
func evaluateCustomPrompt(L *lua.LState, promptFunc *lua.LFunction, ctx PromptContext) (string, error) {
	// Create context table for the prompt function
	contextTable := L.NewTable()
	L.SetField(contextTable, "multiline", lua.LBool(ctx.IsMultiline))
	L.SetField(contextTable, "line_count", lua.LNumber(ctx.LineCount))
	L.SetField(contextTable, "history_length", lua.LNumber(ctx.HistoryLength))
	L.SetField(contextTable, "last_command", lua.LString(ctx.LastCommand))

	// Call the prompt function
	L.Push(promptFunc)
	L.Push(contextTable)

	err := L.PCall(1, 1, nil)
	if err != nil {
		return "", fmt.Errorf("error calling prompt function: %w", err)
	}

	// Get the result
	result := L.Get(-1)
	L.Pop(1)

	if result.Type() != lua.LTString {
		return "", fmt.Errorf("prompt function must return a string")
	}

	return lua.LVAsString(result), nil
}

// ensureConfigDir creates the config directory if it doesn't exist
func ensureConfigDir() error {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return err
	}

	configDir := filepath.Join(homeDir, ".config", "hype")
	return os.MkdirAll(configDir, 0755)
}

// initREPLConfig initializes the REPL config directory and optionally copies an example
func initREPLConfig(example string) error {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("failed to get home directory: %w", err)
	}

	configDir := filepath.Join(homeDir, ".config", "hype")
	configFile := filepath.Join(configDir, "repl.lua")

	// Create config directory
	if err := os.MkdirAll(configDir, 0755); err != nil {
		return fmt.Errorf("failed to create config directory: %w", err)
	}

	// Check if config file already exists
	if _, err := os.Stat(configFile); err == nil {
		fmt.Printf("Config file already exists at: %s\n", configFile)
		fmt.Println("Use 'hype repl-config path' to see the current location.")
		return nil
	}

	// Get example content based on the specified example
	var exampleContent string
	switch example {
	case "basic-colored":
		exampleContent = `-- Basic colored prompt with line numbers
function prompt(ctx)
    if ctx.multiline then
        return "\27[33m....>\27[0m "  -- Yellow continuation prompt
    else
        return string.format("\27[36mhype[%d]>\27[0m ", ctx.line_count)  -- Cyan with line numbers
    end
end`

	case "minimalist":
		exampleContent = `-- Minimalist prompt
function prompt(ctx)
    return ctx.multiline and "... " or "» "
end`

	case "rich-context":
		exampleContent = `-- Rich prompt with timestamp and context information
function prompt(ctx)
    local time = os.date("%H:%M")
    local base_color = ctx.multiline and "\27[33m" or "\27[36m"  -- Yellow or Cyan
    local reset = "\27[0m"
    
    if ctx.multiline then
        return string.format("%s[%s] ...%s ", base_color, time, reset)
    else
        local last_cmd_indicator = (ctx.last_command and ctx.last_command ~= "") and "✓" or "○"
        return string.format("%s[%s] %s hype%s ", base_color, time, last_cmd_indicator, reset)
    end
end`

	case "arrow-style":
		exampleContent = `-- Arrow-style prompt with tree continuation
function prompt(ctx)
    if ctx.multiline then
        return "\27[33m  ├─\27[0m "  -- Yellow tree continuation
    else
        return "\27[32m❯\27[0m "      -- Green arrow
    end
end`

	default:
		return fmt.Errorf("unknown example: %s", example)
	}

	// Write the example content to the config file
	if err := os.WriteFile(configFile, []byte(exampleContent), 0644); err != nil {
		return fmt.Errorf("failed to write config file: %w", err)
	}

	fmt.Printf("✓ Initialized REPL config with '%s' example\n", example)
	fmt.Printf("Config file created at: %s\n", configFile)
	fmt.Println("Run 'hype repl' to use your custom prompt!")

	return nil
}

// supportsColor checks if the terminal supports ANSI colors
func supportsColor() bool {
	term := os.Getenv("TERM")
	colorterm := os.Getenv("COLORTERM")

	// Check for explicitly unsupported terminals
	if term == "dumb" || term == "" {
		return false
	}

	// Check for NO_COLOR environment variable (standard)
	if os.Getenv("NO_COLOR") != "" {
		return false
	}

	// If COLORTERM is set, assume color support
	if colorterm != "" {
		return true
	}

	// List of terminals known to support colors
	colorTerms := []string{"xterm", "screen", "tmux", "rxvt", "ansi", "color"}
	for _, ct := range colorTerms {
		if strings.Contains(term, ct) {
			return true
		}
	}

	// Default to color support for most modern terminals
	return true
}

// printColorfulSplash displays an expressive, colorful splash screen for the REPL
func printColorfulSplash(hasCustomPrompt bool) {
	useColors := supportsColor()

	if !useColors {
		printPlainSplash(hasCustomPrompt)
		return
	}
	// ANSI color codes
	const (
		reset = "\033[0m"
		bold  = "\033[1m"
		dim   = "\033[2m"

		// Colors
		red     = "\033[31m"
		green   = "\033[32m"
		yellow  = "\033[33m"
		blue    = "\033[34m"
		magenta = "\033[35m"
		cyan    = "\033[36m"
		white   = "\033[37m"

		// Bright colors
		brightRed     = "\033[91m"
		brightGreen   = "\033[92m"
		brightYellow  = "\033[93m"
		brightBlue    = "\033[94m"
		brightMagenta = "\033[95m"
		brightCyan    = "\033[96m"

		// Background colors
		bgBlue = "\033[44m"
		bgCyan = "\033[46m"
	)

	fmt.Println()

	// Main logo/header - left border design
	fmt.Printf("  %s%s╔═══════════════════════════════════════════════════════════%s\n", brightCyan, bold, reset)
	fmt.Printf("  %s%s║%s   %s%s🚀 HYPE%s %s%sLua REPL%s %s%sv1.9.0%s\n",
		brightCyan, bold, reset,
		brightMagenta, bold, reset,
		brightYellow, bold, reset,
		brightGreen, bold, reset)
	fmt.Printf("  %s%s║%s   %s⚡ Enhanced Mode with History & Custom Prompts ⚡%s\n",
		brightCyan, bold, reset,
		brightYellow,
		reset)
	fmt.Printf("  %s%s╚═══════════════════════════════════════════════════════════%s\n", brightCyan, bold, reset)

	fmt.Println()

	// Custom prompt status
	if hasCustomPrompt {
		fmt.Printf("  %s%s✨ Custom prompt loaded%s %sfrom ~/.config/hype/repl.lua%s\n",
			brightGreen, bold, reset, dim, reset)
	} else {
		fmt.Printf("  %s💡 Tip:%s Use %s%shype repl-config init%s to customize your prompt\n",
			brightYellow, reset, brightCyan, bold, reset)
	}

	fmt.Println()

	// Feature highlights with icons and colors
	fmt.Printf("  %s%s🎯 Features:%s\n", brightBlue, bold, reset)
	fmt.Printf("    %s▶%s  Interactive Lua evaluation with rich context\n", brightGreen, reset)
	fmt.Printf("    %s⬆⬇%s  Arrow key navigation through command history\n", brightYellow, reset)
	fmt.Printf("    %s⇥%s  Tab completion for keywords and modules\n", brightMagenta, reset)
	fmt.Printf("    %s💾%s  Persistent history storage (~/.hype_history)\n", brightCyan, reset)
	fmt.Printf("    %s🎨%s  Custom prompt themes and styling\n", brightRed, reset)

	fmt.Println()

	// Available modules with styled presentation
	fmt.Printf("  %s%s📦 Available Modules:%s\n", brightBlue, bold, reset)
	modules := []struct{ name, color, desc string }{
		{"http", brightGreen, "HTTP client operations"},
		{"kv", brightYellow, "Key-value storage"},
		{"crypto", brightRed, "Cryptographic functions"},
	}

	for _, mod := range modules {
		fmt.Printf("    %s%s●%s %s%s%-6s%s %s%s%s\n",
			mod.color, bold, reset,
			mod.color, bold, mod.name, reset,
			dim, mod.desc, reset)
	}

	fmt.Println()

	// Controls section with enhanced styling
	fmt.Printf("  %s%s🎮 Controls:%s\n", brightBlue, bold, reset)
	fmt.Printf("    %s%s⏎%s Enter    %sExecute expression%s\n", brightGreen, bold, reset, dim, reset)
	fmt.Printf("    %s%s↑↓%s Arrows   %sNavigate command history%s\n", brightYellow, bold, reset, dim, reset)
	fmt.Printf("    %s%s⇥%s Tab      %sAuto-complete commands%s\n", brightMagenta, bold, reset, dim, reset)
	fmt.Printf("    %s%s^C%s Ctrl+C   %sExit REPL (or clear multiline)%s\n", brightRed, bold, reset, dim, reset)
	fmt.Printf("    %s%s\\%s  Backslash %sContinue on next line%s\n", brightCyan, bold, reset, dim, reset)

	fmt.Println()

	// Fun motivational message with gradient-like effect
	messages := []string{
		"🌟 Ready to explore the Lua universe!",
		"🔥 Let's build something amazing!",
		"⭐ Time to unleash your creativity!",
		"🎉 Welcome to interactive Lua magic!",
		"🚀 Prepare for an awesome coding session!",
	}

	// Use a simple hash of current time to pick message consistently during session
	msgIndex := len(messages) - 1 // Default to last message for consistency
	if len(messages) > 0 {
		// Simple deterministic selection based on session
		msgIndex = 0 // For now, always use first message for consistency
	}

	fmt.Printf("  %s%s%s%s\n",
		brightYellow, bold, messages[msgIndex], reset)

	fmt.Println()

	// Separator line
	fmt.Printf("  %s%s────────────────────────────────────────────────────────────%s\n",
		dim, "─", reset)

	fmt.Println()
}

// printPlainSplash displays a plain splash screen for terminals without color support
func printPlainSplash(hasCustomPrompt bool) {
	fmt.Println()
	fmt.Println("  ╔═══════════════════════════════════════════════════════════")
	fmt.Println("  ║   🚀 HYPE Lua REPL v1.9.0")
	fmt.Println("  ║   ⚡ Enhanced Mode with History & Custom Prompts ⚡")
	fmt.Println("  ╚═══════════════════════════════════════════════════════════")
	fmt.Println()

	if hasCustomPrompt {
		fmt.Println("  ✨ Custom prompt loaded from ~/.config/hype/repl.lua")
	} else {
		fmt.Println("  💡 Tip: Use 'hype repl-config init' to customize your prompt")
	}

	fmt.Println()
	fmt.Println("  🎯 Features:")
	fmt.Println("    ▶  Interactive Lua evaluation with rich context")
	fmt.Println("    ⬆⬇  Arrow key navigation through command history")
	fmt.Println("    ⇥  Tab completion for keywords and modules")
	fmt.Println("    💾  Persistent history storage (~/.hype_history)")
	fmt.Println("    🎨  Custom prompt themes and styling")
	fmt.Println()

	fmt.Println("  📦 Available Modules:")
	fmt.Println("    ● http   - HTTP client operations")
	fmt.Println("    ● kv     - Key-value storage")
	fmt.Println("    ● crypto - Cryptographic functions")
	fmt.Println()

	fmt.Println("  🎮 Controls:")
	fmt.Println("    ⏎ Enter    - Execute expression")
	fmt.Println("    ↑↓ Arrows   - Navigate command history")
	fmt.Println("    ⇥ Tab      - Auto-complete commands")
	fmt.Println("    ^C Ctrl+C   - Exit REPL (or clear multiline)")
	fmt.Println("    \\ Backslash - Continue on next line")
	fmt.Println()

	fmt.Println("  🌟 Ready to explore the Lua universe!")
	fmt.Println()
	fmt.Println("  ─────────────────────────────────────────────────────────────")
	fmt.Println()
}

// listPromptExamples lists the available example prompt configurations
func listPromptExamples() {
	fmt.Println("Available REPL prompt examples:")
	fmt.Println()

	examples := []struct {
		Name        string
		Description string
		Preview     string
	}{
		{
			Name:        "basic-colored",
			Description: "Colored prompt with line numbers",
			Preview:     "hype[1]> print('hello')",
		},
		{
			Name:        "minimalist",
			Description: "Clean, minimal prompt",
			Preview:     "» print('hello')",
		},
		{
			Name:        "rich-context",
			Description: "Rich prompt with timestamp and status",
			Preview:     "[14:30] ○ hype print('hello')",
		},
		{
			Name:        "arrow-style",
			Description: "Modern arrow-style prompt",
			Preview:     "❯ print('hello')",
		},
	}

	for _, ex := range examples {
		fmt.Printf("  %-15s %s\n", ex.Name, ex.Description)
		fmt.Printf("  %-15s Preview: %s\n", "", ex.Preview)
		fmt.Println()
	}

	fmt.Println("Usage:")
	fmt.Println("  hype repl-config init [example]  # Initialize with an example")
	fmt.Println("  hype repl-config init            # Initialize with basic-colored (default)")
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

	return runEnhancedREPLWithState(L)
}

// TUI REPL removed - TUI functionality no longer supported

// Enhanced REPL implementation with command history and arrow key navigation
func runEnhancedREPLWithState(L *lua.LState) error {
	// Set up readline with history
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("failed to get home directory: %w", err)
	}

	historyFile := filepath.Join(homeDir, ".hype_history")

	// Try to load custom prompt function
	customPromptFunc, err := loadCustomPromptConfig()
	hasCustomPrompt := err == nil && customPromptFunc != nil
	if err != nil {
		fmt.Printf("Warning: Failed to load custom prompt config: %v\n", err)
	}

	rl, err := readline.NewEx(&readline.Config{
		Prompt:      "hype> ",
		HistoryFile: historyFile,
		AutoComplete: readline.NewPrefixCompleter(
			readline.PcItem("print"),
			readline.PcItem("local"),
			readline.PcItem("function"),
			readline.PcItem("if"),
			readline.PcItem("then"),
			readline.PcItem("else"),
			readline.PcItem("end"),
			readline.PcItem("for"),
			readline.PcItem("while"),
			readline.PcItem("do"),
			readline.PcItem("return"),
			readline.PcItem("require"),
			readline.PcItem("http.get"),
			readline.PcItem("http.post"),
			readline.PcItem("kv.set"),
			readline.PcItem("kv.get"),
			readline.PcItem("crypto.hash"),
			readline.PcItem("crypto.random"),
		),
		InterruptPrompt: "^C",
		EOFPrompt:       "exit",
	})
	if err != nil {
		return fmt.Errorf("failed to initialize readline: %w", err)
	}
	defer rl.Close()

	printColorfulSplash(hasCustomPrompt)

	var buffer string
	var lineCount int
	var lastCommand string

	for {
		lineCount++

		// Determine if we're in multiline mode
		isMultiline := buffer != ""

		// Get history length (approximate)
		historyLength := 0 // readline doesn't expose this easily, we'll estimate

		// Create prompt context
		ctx := PromptContext{
			IsMultiline:   isMultiline,
			LineCount:     lineCount,
			HistoryLength: historyLength,
			LastCommand:   lastCommand,
		}

		// Set prompt based on custom function or default
		var prompt string
		if hasCustomPrompt {
			customPrompt, err := evaluateCustomPrompt(L, customPromptFunc, ctx)
			if err != nil {
				fmt.Printf("Warning: Custom prompt error: %v\n", err)
				// Fall back to default prompt
				if isMultiline {
					prompt = "....> "
				} else {
					prompt = "hype> "
				}
			} else {
				prompt = customPrompt
			}
		} else {
			// Default prompt behavior
			if isMultiline {
				prompt = "....> "
			} else {
				prompt = "hype> "
			}
		}

		rl.SetPrompt(prompt)

		line, err := rl.Readline()
		if err != nil {
			if err == readline.ErrInterrupt {
				if buffer != "" {
					buffer = ""
					continue
				}
				fmt.Println("Bye!")
				break
			}
			return err
		}

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

		// Skip empty commands
		if strings.TrimSpace(buffer) == "" {
			buffer = ""
			continue
		}

		// Check if the statement is complete
		_, err = L.LoadString(buffer)
		if err != nil && strings.Contains(err.Error(), "<eof>") {
			// Incomplete statement, continue collecting lines
			buffer += "\n"
			continue
		}

		// Statement is complete (or has a different error), execute it
		code := buffer
		buffer = "" // Reset buffer
		lastCommand = strings.TrimSpace(code)

		// Save to history
		rl.SaveHistory(lastCommand)

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
						// Create a simple colorized table representation
						table := lv.(*lua.LTable)
						if supportsColor() {
							result = "\033[96m{\033[0m" // cyan braces

							// Show a few elements
							count := 0
							table.ForEach(func(k, v lua.LValue) {
								if count > 0 {
									result += "\033[37m, \033[0m" // white comma
								}
								if count < 3 {
									// Key
									if k.Type() == lua.LTString {
										keyStr := lua.LVAsString(k)
										result += "\033[92m" + keyStr + "\033[0m = " // green key
									} else {
										result += "\033[94m[" + fmt.Sprintf("%v", k) + "]\033[0m = " // blue bracket
									}

									// Value (simplified)
									switch v.Type() {
									case lua.LTString:
										result += "\033[93m\"" + lua.LVAsString(v) + "\"\033[0m" // yellow string
									case lua.LTNumber:
										result += "\033[95m" + lua.LVAsNumber(v).String() + "\033[0m" // magenta number
									case lua.LTBool:
										boolVal := "false"
										if lua.LVAsBool(v) {
											boolVal = "true"
										}
										result += "\033[91m" + boolVal + "\033[0m" // red boolean
									case lua.LTNil:
										result += "\033[90mnil\033[0m" // gray nil
									case lua.LTTable:
										result += "\033[96m{...}\033[0m" // cyan nested table
									default:
										result += "\033[37m<" + v.Type().String() + ">\033[0m" // white other
									}
								} else if count == 3 {
									result += "\033[37m...\033[0m" // gray ellipsis
								}
								count++
							})

							result += "\033[96m}\033[0m" // cyan closing brace

							// Add table length info
							if table.Len() > 0 {
								result += " \033[90m[" + fmt.Sprintf("%d", table.Len()) + "]\033[0m" // gray length
							}
						} else {
							// Plain text version
							result = "{"
							count := 0
							table.ForEach(func(k, v lua.LValue) {
								if count > 0 {
									result += ", "
								}
								if count < 3 {
									result += fmt.Sprintf("%v=%v", k, v)
								} else if count == 3 {
									result += "..."
								}
								count++
							})
							result += "}"
							if table.Len() > 0 {
								result += fmt.Sprintf("[%d]", table.Len())
							}
						}
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

	return nil
}
