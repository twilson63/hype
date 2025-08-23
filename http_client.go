package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/yuin/gopher-lua"
)

// RegisterHTTPModule registers the HTTP client module
func RegisterHTTPModule(L *lua.LState) {
	L.PreloadModule("http", func(L *lua.LState) int {
		httpModule := L.NewTable()

		// Client methods
		L.SetField(httpModule, "get", L.NewFunction(httpGet))
		L.SetField(httpModule, "post", L.NewFunction(httpPost))
		L.SetField(httpModule, "put", L.NewFunction(httpPut))
		L.SetField(httpModule, "delete", L.NewFunction(httpDelete))
		L.SetField(httpModule, "head", L.NewFunction(httpHead))
		L.SetField(httpModule, "patch", L.NewFunction(httpPatch))
		L.SetField(httpModule, "request", L.NewFunction(httpRequest))

		L.Push(httpModule)
		return 1
	})
}

// httpRequest is the generic HTTP request function
func httpRequest(L *lua.LState) int {
	method := L.CheckString(1)
	url := L.CheckString(2)

	var body io.Reader
	var contentType string
	headers := make(map[string]string)
	timeout := 30 * time.Second

	// Parse options (3rd parameter)
	if L.GetTop() >= 3 && L.Get(3) != lua.LNil {
		switch v := L.Get(3).(type) {
		case lua.LString:
			// If 3rd param is string, it's the body
			body = strings.NewReader(string(v))
		case *lua.LTable:
			// If 3rd param is table, it could be body (for JSON) or options
			if jsonBody := L.GetField(v, "_json"); jsonBody != lua.LNil {
				// Special case: table should be converted to JSON
				jsonBytes, err := tableToJSON(L, v)
				if err != nil {
					L.Push(lua.LNil)
					L.Push(lua.LString(fmt.Sprintf("failed to encode JSON: %v", err)))
					return 2
				}
				body = bytes.NewReader(jsonBytes)
				contentType = "application/json"
			}
		}
	}

	// Parse options (4th parameter or 3rd if body was string)
	optionsIndex := 4
	if body != nil && L.GetTop() >= 3 {
		optionsIndex = 4
	} else if body == nil && L.GetTop() >= 3 {
		optionsIndex = 3
	}

	if L.GetTop() >= optionsIndex {
		if options, ok := L.Get(optionsIndex).(*lua.LTable); ok {
			// Parse timeout
			if timeoutVal := L.GetField(options, "timeout"); timeoutVal != lua.LNil {
				if timeoutNum, ok := timeoutVal.(lua.LNumber); ok {
					timeout = time.Duration(float64(timeoutNum)) * time.Second
				}
			}

			// Parse headers
			if headersVal := L.GetField(options, "headers"); headersVal != lua.LNil {
				if headersTable, ok := headersVal.(*lua.LTable); ok {
					headersTable.ForEach(func(k, v lua.LValue) {
						if key, ok := k.(lua.LString); ok {
							if value, ok := v.(lua.LString); ok {
								headers[string(key)] = string(value)
							}
						}
					})
				}
			}

			// Parse body if not already set
			if body == nil {
				if bodyVal := L.GetField(options, "body"); bodyVal != lua.LNil {
					if bodyStr, ok := bodyVal.(lua.LString); ok {
						body = strings.NewReader(string(bodyStr))
					}
				}
			}
		}
	}

	// Create HTTP client
	client := &http.Client{
		Timeout: timeout,
	}

	// Create request
	req, err := http.NewRequest(method, url, body)
	if err != nil {
		L.Push(lua.LNil)
		L.Push(lua.LString(err.Error()))
		return 2
	}

	// Set headers
	for key, value := range headers {
		req.Header.Set(key, value)
	}

	// Set Content-Type if we have one
	if contentType != "" && req.Header.Get("Content-Type") == "" {
		req.Header.Set("Content-Type", contentType)
	}

	// Set default User-Agent if not provided
	if req.Header.Get("User-Agent") == "" {
		req.Header.Set("User-Agent", "Hype/1.0")
	}

	// Make request
	resp, err := client.Do(req)
	if err != nil {
		L.Push(lua.LNil)
		L.Push(lua.LString(err.Error()))
		return 2
	}
	defer resp.Body.Close()

	// Read response body
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		L.Push(lua.LNil)
		L.Push(lua.LString(fmt.Sprintf("failed to read response: %v", err)))
		return 2
	}

	// Create response table
	responseTable := L.NewTable()
	L.SetField(responseTable, "status", lua.LNumber(resp.StatusCode))
	L.SetField(responseTable, "status_code", lua.LNumber(resp.StatusCode))
	L.SetField(responseTable, "body", lua.LString(string(respBody)))

	// Add headers
	headersTable := L.NewTable()
	for key, values := range resp.Header {
		if len(values) == 1 {
			L.SetField(headersTable, key, lua.LString(values[0]))
		} else if len(values) > 1 {
			// Multiple values - create array
			valuesTable := L.NewTable()
			for i, v := range values {
				valuesTable.RawSetInt(i+1, lua.LString(v))
			}
			L.SetField(headersTable, key, valuesTable)
		}
	}
	L.SetField(responseTable, "headers", headersTable)

	// Add JSON decode helper
	L.SetField(responseTable, "json", L.NewFunction(func(L *lua.LState) int {
		// Try to parse body as JSON
		var result interface{}
		if err := json.Unmarshal(respBody, &result); err != nil {
			L.Push(lua.LNil)
			L.Push(lua.LString(fmt.Sprintf("invalid JSON: %v", err)))
			return 2
		}

		// Convert to Lua value
		luaValue := goToLua(L, result)
		L.Push(luaValue)
		return 1
	}))

	L.Push(responseTable)
	L.Push(lua.LNil)
	return 2
}

// Convenience methods for common HTTP verbs
func httpGet(L *lua.LState) int {
	args := []lua.LValue{lua.LString("GET"), L.Get(1)}
	if L.GetTop() >= 2 {
		args = append(args, L.Get(2)) // options
	}
	L.SetTop(0)
	for _, arg := range args {
		L.Push(arg)
	}
	return httpRequest(L)
}

func httpPost(L *lua.LState) int {
	args := []lua.LValue{lua.LString("POST"), L.Get(1)}
	if L.GetTop() >= 2 {
		args = append(args, L.Get(2)) // body
	}
	if L.GetTop() >= 3 {
		args = append(args, L.Get(3)) // options
	}
	L.SetTop(0)
	for _, arg := range args {
		L.Push(arg)
	}
	return httpRequest(L)
}

func httpPut(L *lua.LState) int {
	args := []lua.LValue{lua.LString("PUT"), L.Get(1)}
	if L.GetTop() >= 2 {
		args = append(args, L.Get(2)) // body
	}
	if L.GetTop() >= 3 {
		args = append(args, L.Get(3)) // options
	}
	L.SetTop(0)
	for _, arg := range args {
		L.Push(arg)
	}
	return httpRequest(L)
}

func httpDelete(L *lua.LState) int {
	args := []lua.LValue{lua.LString("DELETE"), L.Get(1)}
	if L.GetTop() >= 2 {
		args = append(args, L.Get(2)) // options
	}
	L.SetTop(0)
	for _, arg := range args {
		L.Push(arg)
	}
	return httpRequest(L)
}

func httpHead(L *lua.LState) int {
	args := []lua.LValue{lua.LString("HEAD"), L.Get(1)}
	if L.GetTop() >= 2 {
		args = append(args, L.Get(2)) // options
	}
	L.SetTop(0)
	for _, arg := range args {
		L.Push(arg)
	}
	return httpRequest(L)
}

func httpPatch(L *lua.LState) int {
	args := []lua.LValue{lua.LString("PATCH"), L.Get(1)}
	if L.GetTop() >= 2 {
		args = append(args, L.Get(2)) // body
	}
	if L.GetTop() >= 3 {
		args = append(args, L.Get(3)) // options
	}
	L.SetTop(0)
	for _, arg := range args {
		L.Push(arg)
	}
	return httpRequest(L)
}

// Helper functions for JSON conversion
func tableToJSON(L *lua.LState, table *lua.LTable) ([]byte, error) {
	result := luaTableToGo(L, table)
	return json.Marshal(result)
}

func luaTableToGo(L *lua.LState, table *lua.LTable) interface{} {
	// Check if it's an array
	maxn := table.MaxN()
	if maxn > 0 {
		// It's an array
		arr := make([]interface{}, maxn)
		for i := 1; i <= maxn; i++ {
			val := table.RawGetInt(i)
			arr[i-1] = luaValueToGo(L, val)
		}

		// Check if there are non-numeric keys
		hasNonNumeric := false
		table.ForEach(func(k, v lua.LValue) {
			if _, ok := k.(lua.LNumber); !ok {
				hasNonNumeric = true
			}
		})

		if !hasNonNumeric {
			return arr
		}
	}

	// It's a map
	m := make(map[string]interface{})
	table.ForEach(func(k, v lua.LValue) {
		key := k.String()
		m[key] = luaValueToGo(L, v)
	})
	return m
}

func luaValueToGo(L *lua.LState, value lua.LValue) interface{} {
	switch v := value.(type) {
	case lua.LString:
		return string(v)
	case lua.LNumber:
		return float64(v)
	case lua.LBool:
		return bool(v)
	case *lua.LTable:
		return luaTableToGo(L, v)
	case *lua.LNilType:
		return nil
	default:
		return v.String()
	}
}

func goToLua(L *lua.LState, value interface{}) lua.LValue {
	switch v := value.(type) {
	case nil:
		return lua.LNil
	case bool:
		return lua.LBool(v)
	case float64:
		return lua.LNumber(v)
	case string:
		return lua.LString(v)
	case []interface{}:
		table := L.NewTable()
		for i, item := range v {
			table.RawSetInt(i+1, goToLua(L, item))
		}
		return table
	case map[string]interface{}:
		table := L.NewTable()
		for key, val := range v {
			L.SetField(table, key, goToLua(L, val))
		}
		return table
	default:
		return lua.LString(fmt.Sprintf("%v", v))
	}
}
