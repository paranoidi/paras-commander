# Feature handlers reach the app only through Host interfaces

Feature handlers in `internal/apphandler/*` never import `internal/app`; each takes a `Deps` struct plus a narrow `Host` interface (composed from `internal/apphandler/host` facets), and `internal/app` implements those with one-line forwarders in `*_host.go`. This avoids import cycles and keeps handlers testable without the whole app, at the cost of forwarder boilerplate and some logic duplicated across the boundary (see `technical-debt.md`).
