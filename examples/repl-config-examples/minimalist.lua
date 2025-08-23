-- Minimalist prompt
-- Copy this to ~/.config/hype/repl.lua to use

function prompt(ctx)
    return ctx.multiline and "... " or "» "
end