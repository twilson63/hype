package main

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// bundleScript bundles a Lua script with its dependencies into a single file
func bundleScript(scriptPath, outputFile string) error {
	// Generate default output filename if not provided
	if outputFile == "" {
		ext := filepath.Ext(scriptPath)
		name := strings.TrimSuffix(filepath.Base(scriptPath), ext)
		outputFile = name + "-bundled.lua"
	}
	
	bundledScript, err := resolveDependencies(scriptPath, make(map[string]bool))
	if err != nil {
		return fmt.Errorf("failed to resolve dependencies: %v", err)
	}
	
	// Write bundled script to output file
	if err := ioutil.WriteFile(outputFile, []byte(bundledScript), 0644); err != nil {
		return fmt.Errorf("failed to write bundled script: %v", err)
	}
	
	fmt.Printf("Successfully bundled %s -> %s\n", scriptPath, outputFile)
	return nil
}


// resolveDependencies recursively resolves and bundles Lua dependencies
func resolveDependencies(scriptPath string, visited map[string]bool) (string, error) {
	return resolveDependenciesWithModules(scriptPath, visited, make(map[string]bool))
}

func resolveDependenciesWithModules(scriptPath string, visited map[string]bool, availableModules map[string]bool) (string, error) {
	// Convert to absolute path for tracking
	absPath, err := filepath.Abs(scriptPath)
	if err != nil {
		return "", fmt.Errorf("failed to get absolute path for %s: %v", scriptPath, err)
	}
	
	// Check for circular dependencies
	if visited[absPath] {
		return "", fmt.Errorf("circular dependency detected: %s", absPath)
	}
	visited[absPath] = true
	
	// Read the script file
	content, err := ioutil.ReadFile(scriptPath)
	if err != nil {
		return "", fmt.Errorf("failed to read %s: %v", scriptPath, err)
	}
	
	scriptContent := string(content)
	scriptDir := filepath.Dir(scriptPath)
	
	// Find all require statements
	requirePattern := regexp.MustCompile(`require\s*\(\s*['"]([^'"]+)['"]\s*\)`)
	matches := requirePattern.FindAllStringSubmatch(scriptContent, -1)
	
	// Process each require statement
	for _, match := range matches {
		if len(match) < 2 {
			continue
		}
		
		moduleName := match[1]
		
		// Skip built-in modules and plugin modules
		if isBuiltinModule(moduleName) || availableModules[moduleName] {
			continue
		}
		
		// Resolve module path
		modulePath, err := resolveModulePath(moduleName, scriptDir)
		if err != nil {
			return "", fmt.Errorf("failed to resolve module %s: %v", moduleName, err)
		}
		
		// Recursively resolve dependencies
		moduleContent, err := resolveDependenciesWithModules(modulePath, visited, availableModules)
		if err != nil {
			return "", fmt.Errorf("failed to resolve dependencies for %s: %v", modulePath, err)
		}
		
		// Replace require statement with module content
		requireStatement := match[0]
		
		// Wrap module content in an immediately invoked function expression
		// This ensures the module is properly loaded and returned
		wrappedContent := fmt.Sprintf("(function()\n%s\nend)()", moduleContent)
		
		scriptContent = strings.Replace(scriptContent, requireStatement, wrappedContent, 1)
	}
	
	return scriptContent, nil
}

// isBuiltinModule checks if a module is a built-in Hype module
func isBuiltinModule(moduleName string) bool {
	builtins := []string{"http", "kv", "tui", "crypto", "httpsig"}
	for _, builtin := range builtins {
		if moduleName == builtin {
			return true
		}
	}
	return false
}

// resolveModulePath resolves a module name to a file path
func resolveModulePath(moduleName, scriptDir string) (string, error) {
	// Handle relative paths
	if strings.HasPrefix(moduleName, "./") || strings.HasPrefix(moduleName, "../") {
		path := filepath.Join(scriptDir, moduleName)
		if !strings.HasSuffix(path, ".lua") {
			path += ".lua"
		}
		if _, err := os.Stat(path); err == nil {
			return path, nil
		}
		return "", fmt.Errorf("module file not found: %s", path)
	}
	
	// Handle absolute paths
	if filepath.IsAbs(moduleName) {
		path := moduleName
		if !strings.HasSuffix(path, ".lua") {
			path += ".lua"
		}
		if _, err := os.Stat(path); err == nil {
			return path, nil
		}
		return "", fmt.Errorf("module file not found: %s", path)
	}
	
	// Handle module names (search in script directory)
	searchPaths := []string{
		filepath.Join(scriptDir, moduleName+".lua"),
		filepath.Join(scriptDir, moduleName, "init.lua"),
	}
	
	for _, path := range searchPaths {
		if _, err := os.Stat(path); err == nil {
			return path, nil
		}
	}
	
	return "", fmt.Errorf("module not found: %s", moduleName)
}