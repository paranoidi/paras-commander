package panel

import (
	"testing"

	"github.com/paranoidi/paras-commander/internal/treeflat"
)

func tn(id string, kids ...treeflat.Node[TreeEntry]) treeflat.Node[TreeEntry] {
	n := treeflat.Node[TreeEntry]{ID: id}
	if kids != nil {
		n.Children = kids
	}
	return n
}

func TestFindTreeNodeMatchesFullWalk(t *testing.T) {
	local := []treeflat.Node[TreeEntry]{
		tn("/storage/harbor-old", tn("/storage/harbor-old/willow", tn("/storage/harbor-old/willow/lantern"))),
		tn("/storage/harbor", tn("/storage/harbor/meadow", tn("/storage/harbor/meadow/orchard"))),
		// A ghost row: a fileindex:// id under a local parent, with children in another scheme.
		tn("/storage/beacon", tn("fileindex:///storage/beacon/thistle", tn("fileindex:///storage/beacon/thistle/ember"))),
		// Non-path ids; the second duplicate looks like one reachable under a path ancestor.
		tn("plain-token", tn("/storage/odd", tn("dup"))),
		tn("/storage/last", tn("dup")),
	}
	rootSlash := []treeflat.Node[TreeEntry]{
		tn("/", tn("/etc", tn("/etc/cedar"))),
		tn("/var/", tn("/var/log")),
	}
	sftp := []treeflat.Node[TreeEntry]{
		tn("sftp://wren@falcon/", tn("sftp://wren@falcon/home", tn("sftp://wren@falcon/home/maple"))),
	}
	cases := []struct {
		name  string
		roots []treeflat.Node[TreeEntry]
		id    string
		want  bool
	}{
		{"root", local, "/storage/harbor", true},
		{"deep", local, "/storage/harbor/meadow/orchard", true},
		{"prefix sibling", local, "/storage/harbor-old/willow/lantern", true},
		{"prefix sibling not under", local, "/storage/harbor-ol", false},
		{"ghost", local, "fileindex:///storage/beacon/thistle", true},
		{"under ghost", local, "fileindex:///storage/beacon/thistle/ember", true},
		{"absent", local, "/storage/harbor/missing", false},
		{"non-path id", local, "plain-token", true},
		{"duplicate non-path id", local, "dup", true},
		{"under slash root", rootSlash, "/etc/cedar", true},
		{"slash root itself", rootSlash, "/", true},
		{"trailing slash root child", rootSlash, "/var/log", true},
		{"sftp", sftp, "sftp://wren@falcon/home/maple", true},
		{"sftp root", sftp, "sftp://wren@falcon/", true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := findTreeNode(c.roots, c.id)
			if want := findTreeNodeWalk(c.roots, c.id); got != want {
				t.Fatalf("findTreeNode = %p, full walk = %p", got, want)
			}
			if (got != nil) != c.want {
				t.Fatalf("found = %v, want %v", got != nil, c.want)
			}
		})
	}
}
