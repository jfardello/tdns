ALTER TABLE browser_sessions
	ADD COLUMN authentication_method TEXT NOT NULL DEFAULT 'browser_code'
	CHECK (authentication_method IN ('browser_code', 'password'));

CREATE INDEX browser_sessions_authentication_method_idx
	ON browser_sessions (authentication_method);

CREATE TABLE local_administrator (
	singleton INTEGER PRIMARY KEY NOT NULL CHECK (singleton = 1),
	username TEXT NOT NULL CHECK (length(username) BETWEEN 1 AND 64),
	password_hash BLOB NOT NULL CHECK (length(password_hash) > 0),
	subject TEXT NOT NULL CHECK (length(subject) > 0),
	scope TEXT NOT NULL CHECK (scope = 'tdns.kubewire.net:rw'),
	enabled INTEGER NOT NULL CHECK (enabled IN (0, 1)),
	created_at INTEGER NOT NULL,
	updated_at INTEGER NOT NULL
);
