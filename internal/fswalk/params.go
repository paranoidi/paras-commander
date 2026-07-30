package fswalk

// Params configures adaptive filesystem walk concurrency.
type Params struct {
	InitialWorkers  int
	MaxWorkers      int
	AdaptIntervalMS int
}
