INSERT INTO sqlite_sequence (seq, name)
SELECT 0, 'tdnslog'
WHERE NOT EXISTS (
	SELECT 1 FROM sqlite_sequence WHERE name = 'tdnslog'
);
