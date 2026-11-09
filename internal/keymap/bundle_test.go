package keymap

import (
	"reflect"
	"testing"
)

func TestBundleReplaceFromKeepsMapIdentity(t *testing.T) {
	dst, err := DefaultBundle()
	if err != nil {
		t.Fatal(err)
	}
	src, err := buildBundle(mergeBindings(DefaultActionKeys(), map[string][]string{ActionUIReloadConfig: {"M-F12"}}),
		defaultOverlayLayers(), defaultLetterMenuKeys())
	if err != nil {
		t.Fatal(err)
	}
	global := dst.Global
	_ = global.ActionBindings() // populate the cache ReplaceWith must reset
	dst.ReplaceFrom(src)
	if dst.Global != global {
		t.Fatal("Global pointer replaced")
	}
	ev, _ := EventForKey("M-F12")
	if id, ok := global.Lookup(ev); !ok || id != ActionUIReloadConfig {
		t.Fatalf("Lookup = %q, %v; want %s", id, ok, ActionUIReloadConfig)
	}
	found := false
	for _, b := range global.ActionBindings() {
		if b.ActionID == ActionUIReloadConfig {
			found = true
		}
	}
	if !found {
		t.Fatal("ActionBindings() cache not rebuilt after ReplaceFrom")
	}
}

func TestBundleMapFieldsCoverEveryMap(t *testing.T) {
	var b Bundle
	want := 0
	mapType := reflect.TypeOf((*Map)(nil))
	for i := 0; i < reflect.TypeOf(b).NumField(); i++ {
		if reflect.TypeOf(b).Field(i).Type == mapType {
			want++
		}
	}
	if got := len(b.mapFields()); got != want {
		t.Fatalf("mapFields covers %d maps, Bundle has %d", got, want)
	}
}
