package usermenu

import "testing"

func TestCommandRequiresIteratedF(t *testing.T) {
	tests := []struct {
		cmd  string
		want bool
	}{
		{`gzip -9 %f`, true},
		{`gzip -9`, false},
		{`echo %%f`, false},
		{`echo %f %d`, true},
	}
	for _, tc := range tests {
		if got := CommandRequiresIteratedF(tc.cmd); got != tc.want {
			t.Errorf("CommandRequiresIteratedF(%q) = %v, want %v", tc.cmd, got, tc.want)
		}
	}
}
