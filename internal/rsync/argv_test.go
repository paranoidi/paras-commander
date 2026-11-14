package rsync

import (
	"reflect"
	"strings"
	"testing"
)

func base() Options {
	return Options{Sources: []string{"/srv/harbor/orchard"}, Dest: "/srv/meadow", Archive: true}
}

func TestBuildArgv(t *testing.T) {
	tests := []struct {
		name string
		mod  func(*Options)
		want []string
	}{
		{"update", func(*Options) {}, []string{"rsync", "-a", "/srv/harbor/orchard", "/srv/meadow/"}},
		{"no archive recurses", func(o *Options) { o.Archive = false }, []string{"rsync", "-r", "/srv/harbor/orchard", "/srv/meadow/"}},
		{"mirror", func(o *Options) { o.Mode = ModeMirror }, []string{"rsync", "-a", "--delete", "/srv/harbor/orchard", "/srv/meadow/"}},
		{"move", func(o *Options) { o.Mode = ModeMove }, []string{"rsync", "-a", "--remove-source-files", "/srv/harbor/orchard", "/srv/meadow/"}},
		{"flags", func(o *Options) {
			o.Checksum, o.OnlyNewer, o.SkipExisting, o.Compress, o.DryRun = true, true, true, true, true
		}, []string{"rsync", "-a", "-c", "-u", "--ignore-existing", "-z", "-n", "/srv/harbor/orchard", "/srv/meadow/"}},
		{"contents only", func(o *Options) {
			o.ContentsOnly = true
			o.Sources = []string{"/srv/harbor/orchard", "/srv/harbor/willow/"}
		}, []string{"rsync", "-a", "/srv/harbor/orchard/", "/srv/harbor/willow/", "/srv/meadow/"}},
		{"dest slash kept", func(o *Options) { o.Dest = "/srv/meadow/" }, []string{"rsync", "-a", "/srv/harbor/orchard", "/srv/meadow/"}},
		{"excludes", func(o *Options) { o.Exclude = []string{"*.tmp", "lantern"} },
			[]string{"rsync", "-a", "--exclude=*.tmp", "--exclude=lantern", "/srv/harbor/orchard", "/srv/meadow/"}},
		{"bwlimit", func(o *Options) { o.BwLimit = 512 }, []string{"rsync", "-a", "--bwlimit=512", "/srv/harbor/orchard", "/srv/meadow/"}},
		{"extra before paths", func(o *Options) { o.Extra = []string{"--hard-links", "--chmod=D755"} },
			[]string{"rsync", "-a", "--hard-links", "--chmod=D755", "/srv/harbor/orchard", "/srv/meadow/"}},
		{"remote", func(o *Options) { o.SSH = "ssh -p 2222"; o.Dest = "ann@willow:/srv/meadow" },
			[]string{"rsync", "-a", "-e", "ssh -p 2222", "/srv/harbor/orchard", "ann@willow:/srv/meadow/"}},
		{"report progress2", func(o *Options) { o.Report, o.Progress2 = true, true },
			[]string{"rsync", "-a", "--info=progress2", "--no-inc-recursive", "--out-format=%n", "/srv/harbor/orchard", "/srv/meadow/"}},
		{"report old rsync", func(o *Options) { o.Report = true },
			[]string{"rsync", "-a", "--out-format=%n", "/srv/harbor/orchard", "/srv/meadow/"}},
		{"report dry run itemizes", func(o *Options) { o.Report, o.Progress2, o.DryRun = true, true, true },
			[]string{"rsync", "-a", "-n", "--out-format=%i %n%L", "/srv/harbor/orchard", "/srv/meadow/"}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			o := base()
			tc.mod(&o)
			if got := BuildArgv(o); !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("argv = %q\nwant   %q", got, tc.want)
			}
		})
	}
}

func TestDisplayQuotesOnlyWhatNeedsIt(t *testing.T) {
	got := Display([]string{"rsync", "-a", "--exclude=*.tmp", "/srv/old harbor/", "/srv/meadow/"})
	want := `rsync -a '--exclude=*.tmp' '/srv/old harbor/' /srv/meadow/`
	if got != want {
		t.Fatalf("Display = %s, want %s", got, want)
	}
	if !strings.Contains(Display([]string{"rsync", "-e", "ssh -p 2222"}), `'ssh -p 2222'`) {
		t.Fatal("the ssh command must be quoted")
	}
}
