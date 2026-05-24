package ui

import "testing"

func TestCommandStderrDisplay(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		e    CommandRunEntry
		want string
	}{
		{
			name: "stderr wins",
			e:    CommandRunEntry{Stderr: "oops", ErrorMsg: "launch failed"},
			want: "oops",
		},
		{
			name: "error when streams empty",
			e:    CommandRunEntry{ErrorMsg: "exec format error"},
			want: "exec format error",
		},
		{
			name: "no fallback when stdout has data",
			e:    CommandRunEntry{Stdout: "hi", ErrorMsg: "ignored"},
			want: "",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := CommandStderrDisplay(tt.e); got != tt.want {
				t.Fatalf("CommandStderrDisplay() = %q, want %q", got, tt.want)
			}
		})
	}
}
