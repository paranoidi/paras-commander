# `when` predicates

A `when` field decides which rows a rule applies to. Four kinds of rules use the same predicate
language:

| Where | What it decides | Pattern default |
|---|---|---|
| `meta.toml` `[[entry]]` | which rows a meta column runs its `file`/`dirs` command for | glob |
| `config.toml` `[[preview.commands]]` | which files/directories a preview command handles | regex |
| `config.toml` `[[panels.execute_rules]]` | which executables Enter runs in the background | glob |
| user menu (`menu.toml`) entries | which menu entries are visible | glob |

## Syntax

A predicate is a letter, a space, and an argument:

| Predicate | Matches |
|---|---|
| `f <pattern>` | the entry's name (basename only, never the full path) |
| `d <pattern>` | the directory the entry is listed in (full path, no trailing slash) |
| `t <letters>` | the entry's type; any letter may match |

Type letters: `r` regular file, `d` directory, `l` symlink, `n` anything but a directory,
`x` executable (any `x` bit), `c` char device, `b` block device, `f` FIFO, `s` socket.
`t rl` means "regular file or symlink".

Predicates combine with `!` (not), `&` (and), `|` (or) and parentheses. `!` binds tightest, then
`&`, then `|`.

```toml
when = "t d & ! d /tmp"          # directories, except in /tmp
when = "(f *.jpg | f *.png) & ! t l"
```

A bare pattern with no predicate letter is shorthand for `f`: `"*.py"` is `"f *.py"`.

### Lists

`when` can also be a list. The rule matches when **any** item matches (OR). An empty or missing
`when` matches everything.

```toml
when = ["*.mkv", "*.mp4", "t d"]  # video files, or any directory
```

`meta.toml` and the user menu accept a single string or a list; `[[preview.commands]]` and
`[[panels.execute_rules]]` require a list.

### Glob vs regex

`shell_patterns` picks how `f`/`d` patterns are read:

- `true`: glob (`filepath.Match`). The pattern must match the **whole** value. `*` does not cross
  `/`, and there is no `{a,b}` brace expansion or `**`. Matching is case-sensitive;
  `*.[mM][kK][vV]` covers both cases.
- `false`: Go regex (RE2). Unanchored unless you add `^`/`$`, so `d movies` matches any path
  containing `movies`. `(?i)` makes it case-insensitive.

Set it per `[[entry]]` or at the top of `meta.toml` / the user menu file, under `[preview]` for
preview commands, and under `[panels]` for execute rules.

Write regexes as TOML literal strings (`'...'`) so backslashes survive: `'f \.tar\.(gz|xz)$'`.

### Gotcha: `|` inside a regex

`&` and `|` end a predicate's argument unless they are inside parentheses. A regex alternation
must be wrapped in a group:

```toml
when = 'd ^/media/(movies|films)$'          # works
when = 'd ^/media/movies$|^/media/films$'   # error: unknown predicate '^'
```

## Multiple directories

Yes, `when` can match several directories. There are three ways to write it, all equivalent:

```toml
# 1. List items (OR).
when = ["t d & d /media/movies", "t d & d /mnt/archive/movies"]

# 2. | inside one expression.
when = "t d & (d /media/movies | d /mnt/archive/movies)"

# 3. One regex alternation (shell_patterns = false).
when = 't d & d ^(/media|/mnt/archive)/movies$'
```

## Examples

### Any folder whose name starts with "movies"

Rows listed inside `/foo/movies`, `/bar/movies-bd`, and so on, at any depth:

```toml
shell_patterns = false
when = 'd /movies[^/]*$'
```

`d` checks the full directory path, so the regex pins "movies" to the start of the last path
component (`/movies`) and allows anything but another `/` after it. It does not match
`/foo/movies/extras` or `/foo/old-movies`. A glob can't express "at any depth" because `*` never
crosses `/`; `d /*/movies*` only works exactly one level below `/`.

To match the folders themselves (rows named `movies…`) rather than what is inside them, match the
row name instead:

```toml
when = "t d & f movies*"
```

### Movie info only under the movie libraries, not series

The meta column runs on each movie directory in two movie libraries. The series library right
next to them never gets the command.

```toml
# meta.toml
[[entry]]
name = "movie-info"
shell_patterns = false
when = 't d & d ^(/media/movies|/mnt/archive/movies)$'
cache = true
dirs = "movie-info %f"
```

### Episode count in any season folder

Glob `*` matches one path component, so `d /media/series/*` means "directly inside a show
folder": the rows there are the season folders.

```toml
[[entry]]
name = "episodes"
when = "t d & d /media/series/*"
dirs = "find %f -maxdepth 1 -name '*.mkv' | wc -l"
```

### Video duration for video files anywhere except the downloads scratch area

```toml
[[entry]]
name = "duration"
shell_patterns = false
when = 'f (?i)\.(mkv|mp4|webm)$ & ! d ^/home/[^/]+/Downloads'
file = "ffprobe -v error -show_entries format=duration -of csv=p=0 %f"
```

The `f` is required here. The bare-pattern shorthand only applies when the whole expression is
one pattern, and an expression starting with `(` is read as a group, not a regex.

### One column for both files and directories

A bare glob filters out directory rows too, so add `t d` to keep them:

```toml
[[entry]]
name = "disk-size"
when = ["*.iso", "*.img", "t d"]
file = "du -h %f | cut -f1"
dirs = "du -sh %f | cut -f1"
```

### Line counts for source files in your projects only

```toml
[[entry]]
name = "lines"
shell_patterns = false
when = 'd ^/home/[^/]+/projects/ & f \.(go|py)$'
file = "wc -l < %f"
```

The unanchored end makes this match at any depth below `projects/`. Inside an expression every
pattern needs its letter: `d ... & *.go` is an error, `d ... & f *.go` is not.

### Preview: tree view for project directories, plain listing elsewhere

A directory rule applies to quick view and the carousel child column; when every matching rule
declines (non-zero exit), the plain directory listing is shown.

```toml
# config.toml
[preview]
shell_patterns = false

[[preview.commands]]
when = ['t d & d ^/home/[^/]+/projects$']
command = "eza --tree --level=2 --git-ignore %f"

[[preview.commands]]
when = ['f (?i)\.(epub|mobi)$']
command = "ebook-meta %f"
```

### Background-launch GUI programs from two tool folders

```toml
# config.toml
[[panels.execute_rules]]
when = ["t x & (d /opt/apps | d /home/*/Applications)", "*.AppImage"]
background = true
```

## User menu extras

The user menu evaluates `when` against the cursor instead of a single row, which adds:

| Predicate | Matches |
|---|---|
| `F <pattern>` | the name under the cursor in the **other** panel |
| `D <pattern>` | the other panel's directory |
| `T <letters>` | the type of the other panel's cursor entry |
| `t t` | the active panel has tagged (selected) entries |

`d` here is the active panel's directory. In meta, preview and execute rules, `F`/`D`/`T` are
always false and `t t` never matches.

```toml
[archive_tagged]
title = "Archive tagged files"
when = "t t"
command = "tar czf tagged.tar.gz %t"
```
