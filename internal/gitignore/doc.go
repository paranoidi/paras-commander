// Package gitignore applies Git work-tree ignore rules for panel listings and Find.
//
// Rules are loaded from .git/info/exclude and nested .gitignore files from the
// repository root through the listing directory. Global excludes from git config
// (core.excludesFile) and .gitignore outside a work tree are not applied.
package gitignore
