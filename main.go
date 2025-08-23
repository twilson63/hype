package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

// Version information - can be set at build time with -ldflags
var (
	version = "1.10.0"
	commit  = "unknown"
	date    = "unknown"
)

var rootCmd = &cobra.Command{
	Use:   "hype",
	Short: "Package Lua scripts into standalone executables",
	Long: fmt.Sprintf(`hype %s - Lua Script to Executable Packager

hype is a tool that combines a Lua runtime with your Lua scripts to create 
cross-platform executable applications.`, version),
}

var buildCmd = &cobra.Command{
	Use:   "build [lua-script]",
	Short: "Build a Lua script into an executable (auto-bundles dependencies)",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		scriptPath := args[0]
		outputName, _ := cmd.Flags().GetString("output")
		target, _ := cmd.Flags().GetString("target")
		pluginsFlag, _ := cmd.Flags().GetStringSlice("plugins")
		pluginConfig, _ := cmd.Flags().GetString("plugins-config")

		fmt.Printf("Building %s into executable %s for %s\n", scriptPath, outputName, target)

		// Load plugins
		pluginSpecs, err := loadPluginSpecs(pluginsFlag, pluginConfig)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error loading plugin specs: %v\n", err)
			os.Exit(1)
		}

		if err := buildExecutableWithPlugins(scriptPath, outputName, target, pluginSpecs); err != nil {
			fmt.Fprintf(os.Stderr, "Error building executable: %v\n", err)
			os.Exit(1)
		}
	},
}

var runCmd = &cobra.Command{
	Use:   "run [lua-script] -- [script-args...]",
	Short: "Run a Lua script directly",
	Long: `Run a Lua script directly without building an executable. Useful for development and testing.

Any arguments after '--' are passed to the Lua script as command line arguments.

Examples:
  hype run server.lua
  hype run server.lua -- --port 8080 --dir ./public
  hype run server.lua --plugins fs@1.0.0
  hype run server.lua --plugins fs,http-utils@2.1.0
  hype run server.lua --plugins myfs=./path/to/plugin@1.2.0`,
	Args: cobra.MinimumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		scriptPath := args[0]
		scriptArgs := args[1:] // Pass remaining args to script
		pluginsFlag, _ := cmd.Flags().GetStringSlice("plugins")
		pluginConfig, _ := cmd.Flags().GetString("plugins-config")

		// Load plugins
		pluginSpecs, err := loadPluginSpecs(pluginsFlag, pluginConfig)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error loading plugin specs: %v\n", err)
			os.Exit(1)
		}

		if err := runScriptWithPlugins(scriptPath, scriptArgs, pluginSpecs); err != nil {
			fmt.Fprintf(os.Stderr, "Error running script: %v\n", err)
			os.Exit(1)
		}
	},
}

var bundleCmd = &cobra.Command{
	Use:   "bundle [lua-script]",
	Short: "Bundle a Lua script with its dependencies into a single file",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		scriptPath := args[0]
		outputFile, _ := cmd.Flags().GetString("output")

		fmt.Printf("Bundling %s with dependencies...\n", scriptPath)

		if err := bundleScript(scriptPath, outputFile); err != nil {
			fmt.Fprintf(os.Stderr, "Error bundling script: %v\n", err)
			os.Exit(1)
		}
	},
}

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print the version number",
	Run: func(cmd *cobra.Command, args []string) {
		if version == "dev" {
			fmt.Printf("hype %s - Lua Script to Executable Packager\n", version)
			fmt.Printf("Commit: %s\n", commit)
			fmt.Printf("Built: %s\n", date)
		} else {
			fmt.Printf("hype %s - Lua Script to Executable Packager\n", version)
		}
	},
}

func init() {
	buildCmd.Flags().StringP("output", "o", "", "Output executable name")
	buildCmd.Flags().StringP("target", "t", "current", "Target platform (current, linux, windows, darwin)")
	buildCmd.Flags().StringSliceP("plugins", "p", []string{}, "Plugin specifications (e.g., fs@1.0.0, myalias=./path/to/plugin@2.0.0)")
	buildCmd.Flags().String("plugins-config", "", "Path to plugin configuration file")

	runCmd.Flags().StringSliceP("plugins", "p", []string{}, "Plugin specifications (e.g., fs@1.0.0, myalias=./path/to/plugin@2.0.0)")
	runCmd.Flags().String("plugins-config", "", "Path to plugin configuration file")

	bundleCmd.Flags().StringP("output", "o", "", "Output bundled script file (default: [script]-bundled.lua)")

	rootCmd.AddCommand(buildCmd)
	rootCmd.AddCommand(runCmd)
	rootCmd.AddCommand(bundleCmd)
	rootCmd.AddCommand(versionCmd)
	rootCmd.AddCommand(replCmd)
	rootCmd.AddCommand(replConfigCmd)
}

// loadPluginSpecs loads plugin specifications from CLI flags and config files
func loadPluginSpecs(pluginsFlag []string, pluginConfig string) ([]PluginSpec, error) {
	var allSpecs []PluginSpec

	// Load from CLI flags
	if len(pluginsFlag) > 0 {
		specs, err := ParsePluginSpecs(pluginsFlag)
		if err != nil {
			return nil, fmt.Errorf("failed to parse plugin specs: %w", err)
		}
		allSpecs = append(allSpecs, specs...)
	}

	// Load from config file
	if pluginConfig != "" {
		specs, err := LoadPluginConfig(pluginConfig)
		if err != nil {
			return nil, fmt.Errorf("failed to load plugin config: %w", err)
		}
		allSpecs = append(allSpecs, specs...)
	}

	return allSpecs, nil
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
