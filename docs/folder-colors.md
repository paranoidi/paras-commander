# Folder colors

pc shows the folder colors you set in Linux Mint's Nemo (the *folder-color-switcher* extension) and can set or clear them itself. Nemo sees the change, and the other way round.

## How it looks

A colored folder has its name and icon drawn in the color Nemo uses for that folder. The cursor row and selected rows keep their normal highlight colors. Folders on SFTP panels are never colored. Local folders are colored wherever they live: your home directory, the rest of the system, and drives that are currently mounted. Folders on network or device backends (MTP phones, Google Drive, network shares, `admin:`) are not.

## Setting a color

`Alt+Shift+C` (`panel.folder-color-menu`, also Left/Right menu, Folder color...) opens a strip with the colors of your current icon theme style plus **Default**, which clears the color. The choice applies to the selected directories, or to the directory under the cursor when nothing is selected.

## Requirements

- The color palette from folder-color-switcher (`/usr/share/folder-color-switcher/colors.d/`). Without it nothing is colored and the menu reports that no palette was found.
- `gio` (GLib), used to write the color so the GVfs metadata daemon stays in charge of its database. Without it, setting a color fails with a message.

## How it stays current

The colors are read from `~/.local/share/gvfs-metadata` (the databases for your home directory, the rest of the system, and each mounted drive that has a filesystem UUID or label) once at startup, in the background, and checked again every 30 seconds; plugging or unplugging a drive counts as a change. A color changed in Nemo shows up in pc within 30 seconds without flicker (colors set from pc show immediately); painting never touches the disk. A folder shown in the very first frame at startup can get its color a moment after the listing appears.

Folders carrying a Nautilus-style icon name (`folder-red`) are colored too, matched by color name against the palette.
