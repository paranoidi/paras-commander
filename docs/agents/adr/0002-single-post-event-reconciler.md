# Derived UI invariants live in one post-event reconciler

`App.reconcileAfterEvent()` runs once at the end of every event-loop iteration and holds every derived-state hook (selections-strip focus, selection-size scans, latched sync, preview/quick-view refresh, …); each hook must be idempotent. New event sources inherit consistency for free instead of sprinkling fix-ups across dispatch branches. Cost: hook order matters, and tests that bypass `handleKey` must call the reconciler themselves.
