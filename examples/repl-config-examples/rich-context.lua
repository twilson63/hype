-- Rich prompt with timestamp and context information
-- Copy this to ~/.config/hype/repl.lua to use

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
end