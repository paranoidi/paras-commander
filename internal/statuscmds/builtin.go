package statuscmds

// Builtins returns the built-in status commands, available without any status.toml entry.
// Their output carries no separator: the menu bar draws the theme's separator glyph.
func Builtins() []Def {
	return []Def{
		{
			Name:       "loadavg",
			Label:      "Load average",
			Command:    `awk '{print $1, $2, $3}' /proc/loadavg`,
			IntervalMS: 3000,
			MaxWidth:   17,
		},
		{
			Name:       "memory",
			Label:      "Memory used",
			Command:    `awk '/^MemTotal:/{t=$2} /^MemAvailable:/{a=$2} END{if (t) printf "%.1fG/%.0fG\n", (t-a)/1048576, t/1048576}' /proc/meminfo`,
			IntervalMS: 5000,
			MaxWidth:   11,
		},
		{
			// Empty output (no battery) hides the segment.
			Name:       "battery",
			Label:      "Battery",
			Command:    `cat /sys/class/power_supply/BAT*/capacity 2>/dev/null | head -n 1 | sed 's/$/%/'`,
			IntervalMS: 30000,
			MaxWidth:   4,
		},
	}
}
