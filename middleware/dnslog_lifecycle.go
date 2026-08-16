package middleware

import (
	"fmt"

	"github.com/jfardello/tdns/syncsqlite"
)

type DNSLogStatus struct {
	Enabled              bool   `json:"enabled"`
	DomainsPseudonymized bool   `json:"domains_pseudonymized"`
	ClientsPseudonymized bool   `json:"clients_pseudonymized"`
	KeyConfigured        bool   `json:"key_configured"`
	RequiresClear        bool   `json:"requires_clear"`
	Reason               string `json:"reason,omitempty"`
	QueuedEvents         int    `json:"queued_events"`
}

func (cs *DNSLog) Status() DNSLogStatus {
	cs.lifecycleMu.RLock()
	defer cs.lifecycleMu.RUnlock()

	privacy := cs.PrivacyStatus()
	cs.queueMu.Lock()
	queued := len(cs.queue)
	cs.queueMu.Unlock()
	return DNSLogStatus{
		Enabled:              cs.enabled,
		DomainsPseudonymized: privacy.DomainsPseudonymized,
		ClientsPseudonymized: privacy.ClientsPseudonymized,
		KeyConfigured:        privacy.KeyConfigured,
		RequiresClear:        privacy.RequiresClear,
		Reason:               privacy.Reason,
		QueuedEvents:         queued,
	}
}

func (cs *DNSLog) IsEnabled() bool {
	cs.lifecycleMu.RLock()
	defer cs.lifecycleMu.RUnlock()
	return cs.enabled
}

func (cs *DNSLog) StartLogging() error {
	cs.lifecycleMu.Lock()
	defer cs.lifecycleMu.Unlock()
	if cs.PrivacyStatus().RequiresClear {
		return ErrDNSLogRequiresClear
	}
	cs.enabled = true
	return nil
}

func (cs *DNSLog) StopLogging() error {
	cs.lifecycleMu.Lock()
	defer cs.lifecycleMu.Unlock()
	cs.enabled = false
	if err := cs.doInsert(); err != nil {
		cs.discardQueued()
		return fmt.Errorf("flush DNS-log queue while stopping: %w", err)
	}
	return nil
}

func (cs *DNSLog) discardQueued() {
	cs.queueMu.Lock()
	cs.queue = nil
	cs.queueMu.Unlock()
}

func (cs *DNSLog) Clear() error {
	cs.lifecycleMu.Lock()
	defer cs.lifecycleMu.Unlock()
	if cs.enabled {
		return ErrDNSLogRunning
	}

	cs.flushMu.Lock()
	defer cs.flushMu.Unlock()
	cs.dashboardMu.Lock()
	defer cs.dashboardMu.Unlock()

	cs.discardQueued()
	privacy, pseudonymizer := cs.privacySettings()
	stmts := []*syncsqlite.ExecStmt{
		{Query: "DELETE FROM tdnslog"},
		{Query: "DELETE FROM dashboard_hourly_stats"},
		{Query: "DELETE FROM hosts"},
		{Query: "UPDATE sqlite_sequence SET seq = 0 WHERE name = 'tdnslog'"},
		{Query: dnsLogPrivacyStateUpsert, Args: privacyStateArgs(privacy, pseudonymizer)},
	}
	if err := cs.se.SyncExecBulk(stmts); err != nil {
		return fmt.Errorf("clear DNS-log data: %w", err)
	}
	privacy.RequiresClear = false
	privacy.Reason = ""
	cs.setPrivacy(pseudonymizer, privacy)
	return nil
}
