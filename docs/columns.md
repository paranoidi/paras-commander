# Columns and column profiles

Every panel shows the file name plus the columns you choose. Choose them in one dialog, save a choice as a named profile, and switch between profiles with one key. Each panel has its own columns.

## The Columns dialog

`M-,` (leader `,`, Left/Right menu, Columns...) opens the dialog for the active panel. Tick the columns you want and press OK; Cancel leaves the panel as it was.

The file columns come first and are always shown:

| Column | Shows |
|---|---|
| Size | size of a file, or of a directory once scanned ([Disk usage](disk-usage.md)) |
| Modified | modification time |
| Changed | inode change time (ctime): also changes on a rename, `chmod` or `chown` |
| Accessed | last access time; many filesystems update it only coarsely (`relatime`) |
| Created | birth time, where the filesystem records one |
| Permissions | mode string, such as `-rw-r--r--` |
| Owner | owner's name, or the numeric id when the name is not known |
| Group | group's name, or the numeric id when the name is not known |
| Links | number of hard links |

**Meta** lists the [meta columns](meta-columns.md) from the `meta.toml` that applies to the panel's directory. It is hidden when there are none.

**File Index** lists the [fileindex columns](fileindex.md#columns). It is hidden unless the index database exists and opens.

The checkboxes sit in three aligned columns, or fewer on a narrow terminal. A meta column's description shows on the line below them while the checkbox has focus.

`F9` opens `meta.toml` in `$EDITOR` and reloads the list. A `meta.toml` that is skipped as untrusted or fails to load is reported as a message.

Not every source has every fact, and a column with nothing to show stays blank:

| Column | Local | SFTP | `fileindex://` |
|---|---|---|---|
| Owner, Group | yes | numeric id | blank |
| Links, Changed | yes | blank | blank |
| Accessed | yes | yes | blank |
| Created | yes (Linux) | blank | blank |

Created takes one extra system call per entry, so listings only fetch it while a panel shows that column; switching it on reloads the panel. Owner and group names come from `/etc/passwd` and `/etc/group`, so accounts known only through LDAP or SSSD show as numeric ids.

The columns are drawn in a fixed order: name, meta columns, then file columns in the table order above. Size and Modified sort when you click their titles ([Mouse](mouse.md)). When a panel runs out of room it drops meta columns first, then file columns, each rightmost first, then shows names only ([Layout](layout.md)).

## Profiles

A profile is a saved set of ticked columns, both file and meta.

| Profile | Columns |
|---|---|
| `modified` | Size, Modified |
| `permissions` | Size, Permissions |
| `brief` | Size |
| `index` | Size, the fileindex Tags column |

These four are built in. They always exist and can't be overwritten or deleted.

In the Columns dialog:

- `F5` saves the ticked columns: enter a name and an optional description. Saving under an existing saved name replaces it, and `F5` again in the prompt lists saved profiles to overwrite. A built-in name is refused.
- `F2` opens the profile list; type to filter. `Enter` fills the checkboxes from the selected profile, then OK applies them.
- In that list, `F6` makes the selected profile the startup default, and `F8` deletes it.

A profile keeps the names of meta columns the current directory doesn't offer, so saving in one directory doesn't lose a column from another directory's local `meta.toml`. Those columns appear again in directories that offer them.

The keys can be rebound under `[dialog.columns]` in `keybindings.toml`.

## Cycling

`M-t` moves the active panel to the next profile and shows its name in a message: `modified`, `permissions`, `brief`, then your saved profiles in file order, and back to the start. `index` is left out of the cycle. After you choose columns that match no profile, `M-t` starts again from the first profile.

A profile replaces both file and meta columns, so cycling to a profile with no meta columns switches meta columns off.

In [carousel](quick-view-preview.md) view both the dialog and `M-t` work. Meta columns change at once on the centre column, and file columns take effect when you leave the carousel.

## Defaults and files

- `default_column_profile` under [`[panels]`](config.md#panels) picks the profile both panels start with, including its meta columns. `F6` in the profile list sets it. An unknown name shows a warning and uses `brief`.
- `default_profile` under [`[fileindex]`](config.md#fileindex) is applied when a panel with no meta columns enters an index view, and the panel's previous columns come back when it leaves, unless you changed the columns in between. See [Fileindex](fileindex.md#columns).
- Saved profiles live in `column-profiles.toml` in the config directory ([`[columns] file`](config.md#columns)). Editing that file by hand takes effect the next time the Columns dialog opens.

```toml
[[profiles]]
name = "audit"
description = "who owns what"
columns = ["size", "permissions", "owner", "group"]
meta = []
```

The names in `columns` are the File column titles in lower case. The names in `meta` are `meta.toml` entry names or fileindex column names such as `fileindex.tags`.
