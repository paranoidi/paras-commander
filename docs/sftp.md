# SFTP

A panel can browse a remote host over SFTP and transfer files between local and remote panels. Authentication follows OpenSSH conventions, and hosts defined in your `~/.ssh/config` are offered for quick access.

## Connecting

`Alt+R` (`remote.sftp-link`, rebindable in `keybindings.toml`) or Left/Right -> SFTP ... (`T`) opens the connect dialog for that panel. It lists the `Host` entries of your SSH config with a fuzzy filter, and has a location field for typing a URL by hand. Enter on a host fills in `sftp://<alias>/~`. URLs look like `sftp://user@host/path`; `~` stands for the remote home directory and a leading `/` is the server's filesystem root.

The SSH config is read from `[sftp].ssh_config_file`, else `$SSH_CONFIG`, else `~/.ssh/config`, including `Include` files. `HostName`, `User`, `Port` and `IdentityFile` are applied; `Host` and `Match host` patterns with wildcards and `!` negations are honored. Panels keep the alias in their path; it is resolved when connecting.

### Authentication

In order, like OpenSSH: the stanza's `IdentityFile`, keys offered by ssh-agent (`SSH_AUTH_SOCK`, the stanza's `IdentityAgent`, and common socket locations), then the default `~/.ssh/id_*` keys when the stanza has no `IdentityFile` and not `IdentitiesOnly yes`. If those fail, a password dialog appears. Passphrase-protected key files on disk are skipped; load them into the agent instead.

The first connection to an unknown host shows a host-key dialog: Accept (this session), Save (writes to `known_hosts`, `[sftp].known_hosts_file` or `~/.ssh/known_hosts`) or Reject.

Idle connections are closed after `[sftp].idle_timeout_secs` (default 60). Connecting and listing run in the background with `dial_timeout_secs` and `list_timeout_secs` limits, and the two panels connect independently.

## What works

- Browsing, entering and leaving directories, refresh, sorting, filtering.
- Copy and move (`F5` / `F6`) between local and remote, or within the same host, as background [jobs](jobs.md). Extract can target a remote destination.
- Create directories, rename and delete remotely.

## What does not

- Disk usage, volume free-space, gitignore filtering and git status are disabled on remote panels.
- chmod, chown, symlink and hardlink creation are blocked remotely.
- Symlinks inside a remote tree are skipped when copying.
- Files are never executed locally from a remote panel, and local-only features that need a real path (copy-on-write cloning, disk-space checks) do not apply to remote paths.
- History does not check whether remote directories still exist.

## Options

[`[sftp]`](config.md#sftp).
