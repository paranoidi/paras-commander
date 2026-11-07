package panel

import "testing"

func TestMetaAlertsLevel(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name  string
		warn  []string
		value string
		want  bool
	}{
		{"float above", []string{">1.5"}, "1.6", true},
		{"float at limit", []string{">1.5"}, "1.5", false},
		{"float at or above", []string{">=0.25"}, "0.25", true},
		{"float below", []string{"<0.5"}, "0.49", true},
		{"float at or below", []string{"<=0.5"}, "0.51", false},
		{"size value", []string{">1G"}, "1.5G", true},
		{"size below", []string{">1G"}, "800M", false},
		{"non-numeric cell", []string{">1"}, "lantern", false},
		{"nan cell", []string{">1"}, "nan", false},
		{"equal number", []string{"=1.50"}, "1.5", true},
		{"equal text", []string{"=Missing"}, "missing", true},
		{"equal is whole value", []string{"=miss"}, "missing", false},
		{"not equal number", []string{"!=2"}, "2.0", false},
		{"not equal text", []string{"!=indexed"}, "changed", true},
		{"not equal text ignores case", []string{"!=indexed"}, "INDEXED", false},
		{"not equal against text cell", []string{"!=5"}, "lantern", true},
		{"substring", []string{"err"}, "read error", true},
		{"substring ignores case", []string{"ERROR"}, "Read error", true},
		{"bare number is a substring", []string{"2"}, "12", true},
		{"bang without equals is a substring", []string{"!x"}, "wow!x", true},
		{"any listed rule", []string{"meadow", ">5"}, "7", true},
		{"operand spacing", []string{"> 1.5"}, "2", true},
		{"value spacing", []string{">1"}, "  2\n", true},
		{"blank cell", []string{"!=5"}, "  ", false},
		{"multi-field cell", []string{"harbor"}, "harbor\tmeadow", false},
	}
	for _, c := range cases {
		a, err := ParseMetaAlerts(c.warn, nil)
		if err != nil {
			t.Fatalf("%s: %v", c.name, err)
		}
		want := MetaAlertNone
		if c.want {
			want = MetaAlertWarn
		}
		if got := a.Level(c.value); got != want {
			t.Errorf("%s: Level(%q) = %d, want %d", c.name, c.value, got, want)
		}
	}
}

func TestMetaAlertsCriticalWinsOverWarn(t *testing.T) {
	t.Parallel()
	a := MustMetaAlerts([]string{">1"}, []string{">2"})
	for value, want := range map[string]MetaAlertLevel{"1": MetaAlertNone, "2": MetaAlertWarn, "3": MetaAlertCritical} {
		if got := a.Level(value); got != want {
			t.Errorf("Level(%q) = %d, want %d", value, got, want)
		}
	}
}

func TestMetaAlertsEmpty(t *testing.T) {
	t.Parallel()
	if !(MetaAlerts{}).Empty() {
		t.Fatal("zero value should be empty")
	}
	if (MetaAlerts{}).Level("anything") != MetaAlertNone {
		t.Fatal("zero value should never alert")
	}
	if MustMetaAlerts(nil, []string{"x"}).Empty() {
		t.Fatal("critical-only rules are not empty")
	}
}

func TestParseMetaAlertsRejectsInvalidRules(t *testing.T) {
	t.Parallel()
	for _, rule := range []string{"", "  ", ">", ">=", "=", "!=", ">lantern", "<=meadow", ">nan"} {
		if _, err := ParseMetaAlerts([]string{rule}, nil); err == nil {
			t.Errorf("warn rule %q: want error", rule)
		}
		if _, err := ParseMetaAlerts(nil, []string{rule}); err == nil {
			t.Errorf("critical rule %q: want error", rule)
		}
	}
}
