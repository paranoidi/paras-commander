// Package gitignore applies Git work-tree ignore rules for panel listings and Find.
//
// Rules are loaded from .git/info/exclude and nested .gitignore files from the
// repository root through the listing directory. Global excludes from git config
// (core.excludesFile) and .gitignore outside a work tree are not applied.
//
// Work-tree detection (WorkTreeRoot, ValidWorkTreeRoot) is cached process-wide:
// once a repository root is known, descendant paths reuse it without walking
// parents. Entries invalidate when .git or HEAD mtimes change.
package gitignore
