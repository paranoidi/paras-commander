package cmdrun

import "testing"

func TestStripJobControlNoise(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want string
	}{
		{"empty", "", ""},
		{"no noise untouched", "warning: file exists\n", "warning: file exists\n"},
		{
			"bash -ic startup noise around real output",
			"bash: cannot set terminal process group (-1): Inappropriate ioctl for device\n" +
				"bash: no job control in this shell\n" +
				"real stderr line\n",
			"real stderr line\n",
		},
		{"only noise becomes empty (no toast)", "bash: no job control in this shell\n", ""},
	}
	for _, c := range cases {
		if got := string(stripJobControlNoise([]byte(c.in))); got != c.want {
			t.Errorf("%s: got %q, want %q", c.name, got, c.want)
		}
	}
}
