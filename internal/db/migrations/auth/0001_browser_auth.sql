CREATE TABLE browser_sessions (
	session_hash BLOB PRIMARY KEY NOT NULL CHECK (length(session_hash) = 32),
	subject TEXT NOT NULL CHECK (length(subject) > 0),
	scope TEXT NOT NULL CHECK (length(scope) > 0),
	csrf_hash BLOB NOT NULL CHECK (length(csrf_hash) = 32),
	created_at INTEGER NOT NULL,
	last_used_at INTEGER NOT NULL,
	expires_at INTEGER NOT NULL,
	CHECK (expires_at > created_at)
);

CREATE INDEX browser_sessions_expires_at_idx
	ON browser_sessions (expires_at);

CREATE TABLE consumed_browser_codes (
	code_hash BLOB PRIMARY KEY NOT NULL CHECK (length(code_hash) = 32),
	consumed_at INTEGER NOT NULL,
	expires_at INTEGER NOT NULL
);

CREATE INDEX consumed_browser_codes_expires_at_idx
	ON consumed_browser_codes (expires_at);
