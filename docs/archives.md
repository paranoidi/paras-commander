# Archives

Paras Commander looks inside archives without extracting them, and extracts them as background jobs. Both rely on standard command-line tools being on `PATH`; nothing is bundled.

## Archive preview

Highlight an archive and press `F3`, or use [quick view](quick-view-preview.md) or the carousel child column. The contents appear as a fully expanded tree with the same connector style as the file list's tree view: directories are bold and listed first, then files, each group alphabetical. Very long listings end with `[listing truncated]`.

Listable formats: `.zip` / `.jar`, `.tar`, `.tar.gz` / `.tgz`, `.tar.bz2` / `.tbz2`, `.tar.xz`, `.tar.zst`, `.rar` and `.7z`. Single-stream `.gz`, `.bz2`, `.xz` and `.Z` files are not listed (they hold one file); they are treated as ordinary files.

Listing uses `tar`, `unzip`, `unrar` and `7z` (`7za` / `7zz`). If the needed tool is missing the preview names the binary to install.

## Extract

| | |
|---|---|
| Key | `Ctrl+X` (action `file.extract`, rebindable in `keybindings.toml`) |
| Menu | File -> Extract (`x`) |
| Leader | Esc, `x` |

Extract works on the highlighted archive or on all selected archives. The dialog asks for a destination, which defaults to the other panel's directory; the shared path shortcuts for the active and inactive panel and the [path picker](navigation.md#bookmarks-and-the-path-picker) work in the field. Confirming queues a job in the [job queue](jobs.md).

- Existing files are kept: the tools run with their "do not overwrite" options (`unzip -n`, `7z -aos`, `unrar -o-`, `tar -k`). Stream files (`.gz`, `.bz2`, `.xz`, `.Z`) decompress to `<destination>/<name without suffix>` and, if that file exists, go through the normal job conflict prompt ([Jobs](jobs.md)).
- Items the installed tools cannot handle are skipped with a toast naming the reason.
- The destination may be a remote `sftp://` path ([SFTP](sftp.md)).

Extraction tools are looked up once per run: `tar` (or `gtar` / `bsdtar`), `unzip`, `7z` / `7za` / `7zz`, `unrar`, `gzip`, `bzip2`, `xz` and `uncompress`.
