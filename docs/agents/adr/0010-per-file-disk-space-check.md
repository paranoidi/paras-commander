# Free space is checked per file at run time, not upfront

Streamed copies start before totals are known, so there is no upfront free-space check; each file is checked as it runs and a shortage becomes a blocker. This follows mc and keeps enqueue off the UI thread. Cost: a copy can start and then stop midway on a full volume.
