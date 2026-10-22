Source docs/tapes/settings.tape
Output "/tmp/pc-docs-demo/overview.webm"

Hide
Type "clear && HOME=$PC_DEMO_HOME XDG_CONFIG_HOME=$PC_DEMO_HOME/.config ./pc $PC_DEMO_HOME/media/trips/2025-07-lapland-hike $PC_DEMO_HOME/media/aerial"
Enter
Wait+Screen /QuitNow/
Sleep 3s
Show
Down 2
Sleep 500ms
Screenshot docs/assets/overview.png
Sleep 500ms
Hide
Type ":"
Type "q"
Sleep 500ms
Enter
