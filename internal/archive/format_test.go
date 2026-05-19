package archive

import "testing"

func TestFormatForName(t *testing.T) {
	tests := []struct {
		name   string
		want   Format
		wantOK bool
	}{
		{"foo.tar.gz", FormatTarGz, true},
		{"FOO.TGZ", FormatTgz, true},
		{"a.tar.bz2", FormatTarBz2, true},
		{"b.tar.xz", FormatTarXz, true},
		{"c.tar.zst", FormatTarZst, true},
		{"d.tbz2", FormatTbz2, true},
		{"e.tar", FormatTar, true},
		{"f.zip", FormatZip, true},
		{"g.jar", FormatJar, true},
		{"h.rar", FormatRar, true},
		{"i.7z", FormatSevenZ, true},
		{"j.gz", FormatGz, true},
		{"k.bz2", FormatBz2, true},
		{"l.xz", FormatXz, true},
		{"m.Z", FormatZ, true},
		{"readme.txt", FormatUnknown, false},
		{"/path/to/archive.tar.gz", FormatTarGz, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := FormatForName(tt.name)
			if ok != tt.wantOK || got != tt.want {
				t.Fatalf("FormatForName(%q) = (%v, %v), want (%v, %v)", tt.name, got, ok, tt.want, tt.wantOK)
			}
		})
	}
}

func TestFormatForNameLongestSuffix(t *testing.T) {
	got, ok := FormatForName("file.tar.gz")
	if !ok || got != FormatTarGz {
		t.Fatalf("got (%v, %v), want (FormatTarGz, true)", got, ok)
	}
	got, ok = FormatForName("file.gz")
	if !ok || got != FormatGz {
		t.Fatalf("got (%v, %v), want (FormatGz, true)", got, ok)
	}
}

func TestOutputBasename(t *testing.T) {
	if got := OutputBasename("bar.gz", FormatGz); got != "bar" {
		t.Fatalf("OutputBasename gz = %q, want bar", got)
	}
	if got := OutputBasename("data.tar.bz2", FormatBz2); got != "data.tar" {
		t.Fatalf("OutputBasename bz2 = %q, want data.tar", got)
	}
}

func TestNeedsStdoutSink(t *testing.T) {
	if !FormatGz.NeedsStdoutSink() {
		t.Fatal("FormatGz should need stdout sink")
	}
	if FormatZip.NeedsStdoutSink() {
		t.Fatal("FormatZip should not need stdout sink")
	}
}
