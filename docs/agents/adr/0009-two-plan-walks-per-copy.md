# Copy jobs enumerate sources twice: delivery walk and counting walk

The delivery walk feeds the executor and blocks while one item copies, so totals tallied there would freeze; a separate counting walk produces running totals (`TotalsComplete` vs `PlanComplete`). The source tree is enumerated twice in exchange for live, honest progress.
