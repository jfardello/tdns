ALTER TABLE browser_sessions
	ADD COLUMN persistent INTEGER NOT NULL DEFAULT 0 CHECK (persistent IN (0, 1));
