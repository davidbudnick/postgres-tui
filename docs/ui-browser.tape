# Capture primary workspace screens (needs docker postgres seeded as demo)
Output "docs/browser.png"

Require ./bin/postgres-tui

Set Shell "zsh"
Set FontSize 15
Set FontFamily "SF Mono"
Set Theme "Dracula"
Set Width 1600
Set Height 1000
Set TypingSpeed 0.015s
Set Padding 10
Set Framerate 20

Hide
Type "export CLICOLOR_FORCE=1 COLORTERM=truecolor TERM=xterm-256color DEMO_HOME=$(mktemp -d) && mkdir -p $DEMO_HOME/.config/postgres-tui"
Enter
Sleep 0.3s
Type "HOME=$DEMO_HOME ./bin/postgres-tui --host localhost --port 5432 --user postgres --password postgres --database demo --sslmode disable --name 'Local Demo'"
Enter
Sleep 3s
Show

# Workspace browser — jump to a table
Sleep 1.5s
Type "/"
Sleep 0.25s
Type "order_items"
Sleep 0.9s
Down
Sleep 0.5s
Screenshot docs/browser.png
Sleep 0.25s

# Table data
Enter
Sleep 2.5s
Screenshot docs/table-data.png
Sleep 0.25s

# Structure
Type "D"
Sleep 2s
Screenshot docs/table-detail.png
Sleep 0.25s

# Back to tree
Escape
Sleep 0.4s
Escape
Sleep 0.5s

# Object search
Type "/"
Sleep 0.25s
Type "user"
Sleep 1.0s
Screenshot docs/object-search.png
Sleep 0.25s
Escape
Sleep 0.35s

# SQL query with syntax colors + autocomplete
Type ";"
Sleep 1.5s
# clear prefilled if any
Backspace 100
Sleep 0.15s
Type "SELECT id, email FROM us"
Sleep 0.9s
Screenshot docs/query.png
Sleep 0.25s
Tab
Sleep 0.35s
Type " WHERE id < 30 LIMIT 15;"
Sleep 0.3s
Ctrl+Enter
Sleep 2.5s
Screenshot docs/query-results.png
Sleep 0.25s
Escape
Sleep 0.4s

# ERD
Type "E"
Sleep 2.5s
Screenshot docs/erd.png
Sleep 0.25s
Escape
Sleep 0.35s

# Activity
Type "A"
Sleep 1.8s
Screenshot docs/activity.png
Sleep 0.25s
Escape
Sleep 0.35s

# Server
Type "i"
Sleep 1.6s
Screenshot docs/server-info.png
Sleep 0.25s
Escape
Sleep 0.35s

# Databases switcher
Type "G"
Sleep 0.25s
Enter
Sleep 1.6s
Screenshot docs/databases.png
Sleep 0.25s
Escape
Sleep 0.35s

# Help
Type "?"
Sleep 1.2s
Screenshot docs/help.png
Sleep 0.3s
