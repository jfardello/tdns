CREATE TABLE browser_session_csrf_tokens (
	session_hash BLOB NOT NULL CHECK (length(session_hash) = 32),
	csrf_hash BLOB NOT NULL CHECK (length(csrf_hash) = 32),
	created_at INTEGER NOT NULL,
	PRIMARY KEY (session_hash, csrf_hash),
	FOREIGN KEY (session_hash) REFERENCES browser_sessions(session_hash) ON DELETE CASCADE
);

CREATE INDEX browser_session_csrf_tokens_created_at_idx
	ON browser_session_csrf_tokens (session_hash, created_at);

INSERT INTO browser_session_csrf_tokens (session_hash, csrf_hash, created_at)
	SELECT session_hash, csrf_hash, created_at
	FROM browser_sessions;
