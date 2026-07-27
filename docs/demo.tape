# postgres-tui full README demo GIF — lean into query + cool features
Output "docs/main.gif"

Require ./bin/postgres-tui

Set Shell "zsh"
Set FontSize 16
Set FontFamily "SF Mono"
Set Theme "Dracula"
Set Width 1600
Set Height 1000
Set TypingSpeed 0.025s
Set Padding 14
Set Framerate 20
Set PlaybackSpeed 1.0
Set LoopOffset 5%

Hide
Type "export CLICOLOR_FORCE=1 COLORTERM=truecolor TERM=xterm-256color DEMO_HOME=$(mktemp -d) && mkdir -p $DEMO_HOME/.config/postgres-tui && cp docs/demo-config.json $DEMO_HOME/.config/postgres-tui/config.json"
Enter
Sleep 0.3s
Type "HOME=$DEMO_HOME ./bin/postgres-tui"
Enter
Sleep 1.6s
Show

# Home cards
Sleep 2.0s

Hide
Type "q"
Enter
Sleep 0.35s
Type "HOME=$DEMO_HOME ./bin/postgres-tui --host localhost --port 5432 --user postgres --password postgres --database demo --sslmode disable --name 'Local Demo'"
Enter
Sleep 2.6s
Show

# Browser + search
Sleep 1.4s
Type "/"
Sleep 0.2s
Type "order"
Sleep 0.9s
Down
Sleep 0.5s

# Table data (blue band + value colors)
Enter
Sleep 2.4s
Type "j"
Sleep 0.12s
Type "j"
Sleep 0.12s
Type "l"
Sleep 0.15s
Type "l"
Sleep 0.35s

# Structure (typed columns)
Type "D"
Sleep 2.0s
Type "l"
Sleep 0.45s
Type "l"
Sleep 0.55s

Escape
Sleep 0.35s
Escape
Sleep 0.45s

# ── SQL is the star: highlight + smart complete + run ──
Type ";"
Sleep 1.6s
Backspace 100
Sleep 0.15s
Type "SELECT id, email FROM us"
Sleep 1.0s
Tab
Sleep 0.45s
Type " WHERE id < 25"
Sleep 0.35s
Type " LIMIT 12;"
Sleep 0.4s
Ctrl+Enter
Sleep 2.6s

# Tweak query — show column complete in WHERE-ish edit
Left 12
Sleep 0.2s
# leave results visible after another run
Ctrl+Enter
Sleep 1.8s

Escape
Sleep 0.45s

# ERD (cool graph)
Type "E"
Sleep 2.8s
Escape
Sleep 0.4s

# Object search flash
Type "/"
Sleep 0.2s
Type "product"
Sleep 1.0s
Escape
Sleep 0.5s

# Land on browser preview
Sleep 1.2s
