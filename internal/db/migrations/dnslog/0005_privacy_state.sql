CREATE TABLE IF NOT EXISTS dnslog_privacy_state (
	singleton INTEGER NOT NULL PRIMARY KEY CHECK (singleton = 1),
    domain_mode TEXT NOT NULL CHECK (domain_mode IN ('plain', 'hmac-sha256-v1')),
    client_mode TEXT NOT NULL CHECK (client_mode IN ('plain', 'hmac-sha256-v1')),
	key_fingerprint TEXT NOT NULL DEFAULT ''
);
