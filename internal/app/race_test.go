//go:build race

package app

// raceSlowdown scales wall-clock budgets in perf-guard tests; the race detector runs code
// roughly 5-10x slower.
const raceSlowdown = 10
