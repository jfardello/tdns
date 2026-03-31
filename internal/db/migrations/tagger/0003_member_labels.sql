CREATE TABLE IF NOT EXISTS member_labels (
	member_address TEXT NOT NULL REFERENCES members(address) ON DELETE CASCADE,
	label_name TEXT NOT NULL REFERENCES labels(name) ON DELETE CASCADE,
	PRIMARY KEY (member_address, label_name)
);
