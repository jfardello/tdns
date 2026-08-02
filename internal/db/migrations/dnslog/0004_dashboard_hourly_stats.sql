CREATE TABLE IF NOT EXISTS dashboard_hourly_stats (
	hour_bucket INTEGER NOT NULL PRIMARY KEY,
	total_queries INTEGER NOT NULL CHECK (total_queries >= 0),
	blocked_queries INTEGER NOT NULL CHECK (blocked_queries >= 0),
	allowed_queries INTEGER NOT NULL CHECK (allowed_queries >= 0),
	computed_at INTEGER NOT NULL,
	CHECK (blocked_queries + allowed_queries = total_queries)
);
