-- Arrow-style prompt with tree continuation
-- Copy this to ~/.config/hype/repl.lua to use

function prompt(ctx)
    if ctx.multiline then
        return "\27[33m  ├─\27[0m "  -- Yellow tree continuation
    else
        return "\27[32m❯\27[0m "      -- Green arrow
    end
end