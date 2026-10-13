# Background readers yield to an active job's volume only at dispatch

Git status, periodic refresh, automatic disk-usage scans and preview prefetch each check for device overlap with an unfinished job before starting new work and skip if it contends; explicit user-requested scans are never gated. Mid-walk preemption was rejected because `jobs` imports `diskusage` (no reverse dependency) and it would need callbacks through each engine. SFTP-only jobs are not covered.
