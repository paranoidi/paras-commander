# The find dialog keeps an event-fed mirror of the scan index

The UI thread never queries the scan coordinator; it holds a mirror updated only through `scan.Event` payloads, and matching runs on a snapshot copy. This keeps the UI lock-free while indexing runs, at the cost of duplicated state and eventual consistency.
