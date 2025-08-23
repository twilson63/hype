-- Basic colored prompt with line numbers
-- Copy this to ~/.config/hype/repl.lua to use

function prompt(ctx)
    if ctx.multiline then
        return "\27[33m....>\27[0m "  -- Yellow continuation prompt
    else
        return string.format("\27[36mhype[%d]>\27[0m ", ctx.line_count)  -- Cyan with line numbers
    end
end