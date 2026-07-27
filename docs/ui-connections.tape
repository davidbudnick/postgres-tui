# Capture: home / connections screen
Output "docs/connections.png"

Require ./bin/postgres-tui

Set Shell "zsh"
Set FontSize 16
Set FontFamily "SF Mono"
Set Theme "Dracula"
Set Width 1400
Set Height 900
Set TypingSpeed 0.02s
Set Padding 12
Set Framerate 20

Hide
Type "export CLICOLOR_FORCE=1 COLORTERM=truecolor TERM=xterm-256color DEMO_HOME=$(mktemp -d) && mkdir -p $DEMO_HOME/.config/postgres-tui && cp docs/demo-config.json $DEMO_HOME/.config/postgres-tui/config.json"
Enter
Sleep 0.3s
Type "HOME=$DEMO_HOME ./bin/postgres-tui"
Enter
Sleep 1.8s
Show

Sleep 2s
Screenshot docs/connections.png
Sleep 0.3s
