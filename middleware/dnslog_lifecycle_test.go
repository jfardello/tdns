package middleware

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/jfardello/tdns/internal/db"
	"github.com/jfardello/tdns/syncsqlite"
)

func TestDNSLogStopIsWriteBarrier(t *testing.T) {
	connString := newTempConnString(t)
	if err := db.RunMigrations(context.Background(), connString, db.TargetDNSLog); err != nil {
		t.Fatalf("RunMigrations: %v", err)
	}
	cs := configuredPrivacyDNSLog(t, connString, "TDNS_TEST_STOP_KEY", encodedDNSLogKey(0x21), false, false)

	for index := 0; index < 25; index++ {
		cs.Append(LogEvent{
			Timestamp: time.Now().UTC().Add(time.Duration(index) * time.Nanosecond),
			Client:    "192.0.2.1",
			Domain:    "example.com.",
		})
	}
	if status := cs.Status(); !status.Enabled || status.QueuedEvents != 25 {
		t.Fatalf("status before stop = %#v", status)
	}
	if err := cs.StopLogging(); err != nil {
		t.Fatalf("StopLogging: %v", err)
	}

	before := dnsLogRowCount(t, cs.se, "tdnslog")
	if before != 25 {
		t.Fatalf("stored rows after stop = %d, want 25", before)
	}
	for index := 0; index < 25; index++ {
		cs.Append(LogEvent{Timestamp: time.Now(), Client: "192.0.2.2", Domain: "after-stop.example."})
	}
	if err := cs.doInsert(); err != nil {
		t.Fatalf("doInsert after stop: %v", err)
	}
	if after := dnsLogRowCount(t, cs.se, "tdnslog"); after != before {
		t.Fatalf("rows changed after stop returned: before=%d after=%d", before, after)
	}
	if status := cs.Status(); status.Enabled || status.QueuedEvents != 0 {
		t.Fatalf("status after stop = %#v", status)
	}
}

func TestDNSLogClearRequiresStopAndDeletesAllData(t *testing.T) {
	connString := newTempConnString(t)
	if err := db.RunMigrations(context.Background(), connString, db.TargetDNSLog); err != nil {
		t.Fatalf("RunMigrations: %v", err)
	}
	cs := configuredPrivacyDNSLog(t, connString, "TDNS_TEST_CLEAR_KEY", encodedDNSLogKey(0x22), true, true)

	cs.Append(LogEvent{Timestamp: time.Now().UTC(), Client: "h1c_test", Domain: "h1d_test"})
	if err := cs.AddAlias("office", "192.0.2.1"); err != nil {
		t.Fatalf("AddAlias: %v", err)
	}
	if _, err := cs.se.SyncExec(`
INSERT INTO dashboard_hourly_stats
	(hour_bucket, total_queries, blocked_queries, allowed_queries, computed_at)
VALUES (?, 1, 0, 1, ?)`, []any{dashboardHourBucket(time.Now()) - 1, time.Now().Unix()}); err != nil {
		t.Fatalf("seed dashboard cache: %v", err)
	}
	if err := cs.Clear(); !errors.Is(err, ErrDNSLogRunning) {
		t.Fatalf("Clear while running error = %v, want ErrDNSLogRunning", err)
	}
	if err := cs.StopLogging(); err != nil {
		t.Fatalf("StopLogging: %v", err)
	}
	if err := cs.Clear(); err != nil {
		t.Fatalf("Clear: %v", err)
	}

	for _, table := range []string{"tdnslog", "hosts", "dashboard_hourly_stats"} {
		if rows := dnsLogRowCount(t, cs.se, table); rows != 0 {
			t.Errorf("%s rows after clear = %d", table, rows)
		}
	}
	conn := cs.se.GetConn()
	defer cs.se.FreeConn(conn)
	var sequence int
	if err := conn.QueryRow("SELECT seq FROM sqlite_sequence WHERE name = 'tdnslog'").Scan(&sequence); err != nil {
		t.Fatalf("read sequence: %v", err)
	}
	if sequence != 0 {
		t.Fatalf("sequence after clear = %d, want 0", sequence)
	}
	if status := cs.Status(); status.Enabled || status.QueuedEvents != 0 || status.RequiresClear {
		t.Fatalf("status after clear = %#v", status)
	}
	if err := cs.StartLogging(); err != nil {
		t.Fatalf("StartLogging after clear: %v", err)
	}
}

func TestDNSLogClearResolvesPrivacyIncompatibility(t *testing.T) {
	connString := newTempConnString(t)
	if err := db.RunMigrations(context.Background(), connString, db.TargetDNSLog); err != nil {
		t.Fatalf("RunMigrations: %v", err)
	}
	se := syncsqlite.NewSyncExecutor(connString, 1)
	if _, err := se.SyncExec(
		"INSERT INTO tdnslog (dt, client, domain, blocked) VALUES (?, ?, ?, 0)",
		[]any{time.Now().UnixNano(), "192.0.2.1", "example.com."},
	); err != nil {
		t.Fatalf("seed plaintext data: %v", err)
	}
	se.Close()

	cs := configuredPrivacyDNSLog(t, connString, "TDNS_TEST_INCOMPATIBLE_CLEAR_KEY", encodedDNSLogKey(0x23), true, false)
	if status := cs.Status(); !status.RequiresClear || status.Enabled {
		t.Fatalf("initial incompatible status = %#v", status)
	}
	if err := cs.StartLogging(); !errors.Is(err, ErrDNSLogRequiresClear) {
		t.Fatalf("StartLogging error = %v, want ErrDNSLogRequiresClear", err)
	}
	if err := cs.Clear(); err != nil {
		t.Fatalf("Clear: %v", err)
	}
	if err := cs.StartLogging(); err != nil {
		t.Fatalf("StartLogging after clear: %v", err)
	}
}

func dnsLogRowCount(t *testing.T, executor *syncsqlite.SyncExecutor, table string) int {
	t.Helper()
	conn := executor.GetConn()
	defer executor.FreeConn(conn)
	var count int
	if err := conn.QueryRow("SELECT COUNT(*) FROM " + table).Scan(&count); err != nil {
		t.Fatalf("count %s: %v", table, err)
	}
	return count
}
