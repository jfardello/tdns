CREATE TABLE IF NOT EXISTS config_overrides (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	kind INTEGER NOT NULL,
	op INTEGER NOT NULL,
	target TEXT NOT NULL DEFAULT '',
	value TEXT NOT NULL DEFAULT '',
	created_at INTEGER NOT NULL,
	updated_at INTEGER NOT NULL,
	UNIQUE(kind, target)
);

CREATE INDEX IF NOT EXISTS idx_config_overrides_kind ON config_overrides(kind);
