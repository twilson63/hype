# Hype REPL Custom Prompt Examples

This directory contains example custom prompt configurations for the Hype REPL.

## How to Use

1. Choose one of the example configurations below
2. Copy the file to `~/.config/hype/repl.lua`
3. Start the Hype REPL: `hype repl`

## Available Examples

### `basic-colored.lua`
A simple colored prompt with line numbers:
```
hype[1]> print("hello")
hype[2]> local x = \
....> 42
```

### `minimalist.lua` 
A clean, minimalist prompt:
```
» print("hello")
» local x = \
... 42
```

### `rich-context.lua`
A rich prompt with timestamp and command status:
```
[14:30] ○ hype print("hello")
[14:30] ✓ hype local x = \
[14:30] ... 42
```

### `arrow-style.lua`
Modern arrow-style prompt with tree continuation:
```
❯ print("hello")
❯ local x = \
  ├─ 42
```

## Context Variables

Your custom prompt function receives a context table with these fields:

- `multiline` (boolean): Whether we're continuing a multiline input
- `line_count` (number): Current line number in the session
- `history_length` (number): Number of commands in history (approximate)
- `last_command` (string): The last successfully executed command

## Color Codes

You can use ANSI color codes in your prompts:
- `\27[31m` - Red
- `\27[32m` - Green
- `\27[33m` - Yellow
- `\27[34m` - Blue
- `\27[35m` - Magenta
- `\27[36m` - Cyan
- `\27[37m` - White
- `\27[0m` - Reset to default

## Example Custom Function

```lua
function prompt(ctx)
    local color = ctx.multiline and "\27[33m" or "\27[32m"
    local symbol = ctx.multiline and "..." or ">"
    return string.format("%s%s\27[0m ", color, symbol)
end
```