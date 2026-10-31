# Change directory on exit

A program cannot change the directory of the shell that started it. To land in the active panel's directory after quitting `pc`, start it through a small shell function that reads the directory back.

```sh
pc --cwd-file=PATH
```

On quit, `pc` writes the active panel's directory (absolute, with a trailing newline) to `PATH`. On an SFTP panel nothing is written; on a `fileindex://` panel the nearest directory that still exists on disk is written.

## bash / zsh

Add to `~/.bashrc` or `~/.zshrc`:

```sh
pcd() {
    local tmp dir
    tmp="$(mktemp)"
    pc --cwd-file="$tmp" "$@"
    dir="$(cat "$tmp")"
    rm -f "$tmp"
    [ -n "$dir" ] && [ "$dir" != "$PWD" ] && cd "$dir"
}
```

## fish

Save as `~/.config/fish/functions/pcd.fish`:

```fish
function pcd
    set -l tmp (mktemp)
    pc --cwd-file=$tmp $argv
    set -l dir (cat $tmp)
    rm -f $tmp
    if test -n "$dir"; and test "$dir" != "$PWD"
        cd $dir
    end
end
```

Name the function `pc` instead (and call `command pc` inside it) if you always want this behaviour.
