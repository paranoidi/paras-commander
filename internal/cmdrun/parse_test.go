package cmdrun

import (
	"reflect"
	"testing"
)

func TestParseCommandArgvSimple(t *testing.T) {
	got, err := ParseCommandArgv(`gzip -9`)
	if err != nil {
		t.Fatalf("ParseCommandArgv: %v", err)
	}
	want := []string{"gzip", "-9"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %#v want %#v", got, want)
	}
}

func TestParseCommandArgvQuotedSpaces(t *testing.T) {
	got, err := ParseCommandArgv(`echo "hello world"`)
	if err != nil {
		t.Fatalf("ParseCommandArgv: %v", err)
	}
	want := []string{"echo", "hello world"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %#v want %#v", got, want)
	}
}

func TestParseCommandArgvAppendPath(t *testing.T) {
	prefix, err := ParseCommandArgv(`ls -la`)
	if err != nil {
		t.Fatal(err)
	}
	path := `/tmp/foo bar/file.txt`
	argv := append(append([]string(nil), prefix...), path)
	want := []string{"ls", "-la", `/tmp/foo bar/file.txt`}
	if !reflect.DeepEqual(argv, want) {
		t.Fatalf("got %#v want %#v", argv, want)
	}
}

func TestParseCommandArgvEmpty(t *testing.T) {
	got, err := ParseCommandArgv(`   `)
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if len(got) != 0 {
		t.Fatalf("got %#v, want empty slice", got)
	}
}
