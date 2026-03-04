# Interactive Mode Commands (DO NOT RUN)

The following commands enter interactive mode that waits for user input, causing timeout. **Never execute these.**

## Claude CLI

- `claude` (without args) - Starts interactive session
- `claude skills` - Skills management (list, install, etc.)
- `claude mcp` - MCP server management
- `claude config` - Configuration changes
- `-r` / `--resume` (without value) - Session picker

## Git

- `git rebase -i` - Interactive rebase
- `git add -i` / `git add --interactive` - Interactive staging
- `git add -p` / `git add --patch` - Patch mode
- `git checkout -p` - Patch checkout
- `git reset -p` - Patch reset
- `git stash -p` - Patch stash
- `git commit` (without message) - Opens editor

## Editors/Viewers

- `vim`, `vi`, `nvim`, `nano`, `pico`, `emacs`
- `less`, `more`, `man`

## Others

- `ssh` (when password required)
- `sudo` (when password required)
- `npm init` (question mode)
- `npx create-*` (most ask questions)
- `python` / `node` (REPL, when run without args)

## Alternatives

- Check skills list: `ls ~/.claude/skills/`
- Git commit: `git commit -m "message"`
- View files: Use `cat`, `head`, `tail`
- Run Python/Node: `python -c "code"` / `node -e "code"`
