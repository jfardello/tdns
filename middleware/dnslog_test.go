package middleware

import (
	"context"
	"fmt"
	"path/filepath"
	"testing"
	"time"

	"github.com/jfardello/tdns/internal/db"
	"github.com/jfardello/tdns/syncsqlite"
)

func newTempConnString(t *testing.T) string {
	t.Helper()
	dbPath := filepath.Join(t.TempDir(), "test.db")
	return fmt.Sprintf("file:%s?cache=shared", dbPath)
}

func TestDNSLog_doInsert(t *testing.T) {
	ts := time.Unix(1700000000, 123456789)
	tests := []LogEvent{
		{
			Timestamp: ts,
			Client:    "1.1.1.1",
			Domain:    "www.google.com.",
		},
	}

	connString := newTempConnString(t)
	if err := db.RunMigrations(context.Background(), connString, db.TargetDNSLog); err != nil {
		t.Fatalf("RunMigrations error: %v", err)
	}
	se := syncsqlite.NewSyncExecutor(connString, 1)
	cs := &DNSLog{se: se}

	t.Run("test insert", func(t *testing.T) {
		cs.queue = tests
		cs.doInsert()
	})

	db := se.GetConn()
	defer se.FreeConn(db)
	var count int
	err := db.QueryRow("SELECT COUNT(*) FROM tdnslog").Scan(&count)
	if err != nil {
		t.Fatalf("count query error: %v", err)
	}
	if count != 1 {
		t.Fatalf("expected 1 row, got %d", count)
	}

	var client, domain string
	var blocked int
	err = db.QueryRow("SELECT client, domain, blocked FROM tdnslog").Scan(&client, &domain, &blocked)
	if err != nil {
		t.Fatalf("row query error: %v", err)
	}
	if client != "1.1.1.1" {
		t.Fatalf("expected client 1.1.1.1, got %s", client)
	}
	if domain != "www.google.com." {
		t.Fatalf("expected domain www.google.com., got %s", domain)
	}
	if blocked != 0 {
		t.Fatalf("expected blocked 0, got %d", blocked)
	}
}

func TestDNSLogRotateRejectsUnsafeRetention(t *testing.T) {
	cs := &DNSLog{}
	for _, value := range []string{"", "0s", "181d", "forever"} {
		if err := cs.Rotate(value); err == nil {
			t.Errorf("Rotate(%q) accepted an unsafe retention", value)
		}
	}
}

func TestSQLStmt_Build(t *testing.T) {
	type fields struct {
		SelectStr string
		From      string
		Where     string
		OrderBy   string
		Limit     string
		GroupBy   string
	}
	tests := []struct {
		name    string
		fields  fields
		want    string
		wantErr bool
	}{
		{name: "selectFrom", fields: fields{SelectStr: "SELECT *", From: "FROM pepe"}, want: "SELECT * FROM pepe", wantErr: false},
		{name: "allFields",
			fields: fields{
				SelectStr: "SELECT *",
				From:      "FROM pepe",
				Where:     `WHERE pepe like '%lol%'`,
				GroupBy:   "GROUP BY popo",
				OrderBy:   "ORDER BY popo",
				Limit:     "LIMIT 10",
			},
			want:    `SELECT * FROM pepe WHERE pepe like '%lol%' GROUP BY popo ORDER BY popo LIMIT 10`,
			wantErr: false,
		},
		{name: "noFrom", fields: fields{SelectStr: "SELECT *", From: ""}, want: "", wantErr: true},
		{name: "errNoSelect", fields: fields{SelectStr: "", From: "pepe"}, want: "", wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sq := &SQLStmt{
				SelectStr: tt.fields.SelectStr,
				From:      tt.fields.From,
				Where:     tt.fields.Where,
				OrderBy:   tt.fields.OrderBy,
				GroupBy:   tt.fields.GroupBy,
				Limit:     tt.fields.Limit,
			}
			got, err := sq.Build()
			if (err != nil) != tt.wantErr {
				t.Errorf("Build() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("Build() got = %#v, want %#v", got, tt.want)
			}
		})
	}
}

func TestDNSLog_GetDashboardStats(t *testing.T) {
	connString := newTempConnString(t)
	if err := db.RunMigrations(context.Background(), connString, db.TargetDNSLog); err != nil {
		t.Fatalf("RunMigrations error: %v", err)
	}

	se := syncsqlite.NewSyncExecutor(connString, 1)
	cs := &DNSLog{se: se}

	now := time.Now().UTC().Truncate(time.Hour)
	stmts := []*syncsqlite.ExecStmt{
		{
			Query: "INSERT INTO tdnslog (dt, client, domain, blocked) VALUES (?, ?, ?, ?)",
			Args:  []any{now.Add(10 * time.Minute).UnixNano(), "1.1.1.1", "allowed-now.example.", 0},
		},
		{
			Query: "INSERT INTO tdnslog (dt, client, domain, blocked) VALUES (?, ?, ?, ?)",
			Args:  []any{now.Add(20 * time.Minute).UnixNano(), "1.1.1.1", "blocked-now.example.", 1},
		},
		{
			Query: "INSERT INTO tdnslog (dt, client, domain, blocked) VALUES (?, ?, ?, ?)",
			Args:  []any{now.Add(-50 * time.Minute).UnixNano(), "1.1.1.1", "allowed-prev.example.", 0},
		},
	}
	if err := se.SyncExecBulk(stmts); err != nil {
		t.Fatalf("SyncExecBulk error: %v", err)
	}

	stats, err := cs.GetDashboardStats(2)
	if err != nil {
		t.Fatalf("GetDashboardStats error: %v", err)
	}

	if stats.WindowHours != 2 {
		t.Fatalf("expected window 2, got %d", stats.WindowHours)
	}
	if stats.Summary.TotalQueries != 3 {
		t.Fatalf("expected total 3, got %d", stats.Summary.TotalQueries)
	}
	if stats.Summary.BlockedQueries != 1 {
		t.Fatalf("expected blocked 1, got %d", stats.Summary.BlockedQueries)
	}
	if stats.Summary.AllowedQueries != 2 {
		t.Fatalf("expected allowed 2, got %d", stats.Summary.AllowedQueries)
	}
	if len(stats.Hourly) != 2 {
		t.Fatalf("expected 2 hourly buckets, got %d", len(stats.Hourly))
	}
	if stats.Hourly[0].TotalQueries != 1 {
		t.Fatalf("expected first bucket total 1, got %d", stats.Hourly[0].TotalQueries)
	}
	if stats.Hourly[0].BlockedQueries != 0 {
		t.Fatalf("expected first bucket blocked 0, got %d", stats.Hourly[0].BlockedQueries)
	}
	if stats.Hourly[0].AllowedQueries != 1 {
		t.Fatalf("expected first bucket allowed 1, got %d", stats.Hourly[0].AllowedQueries)
	}
	if stats.Hourly[1].TotalQueries != 2 {
		t.Fatalf("expected second bucket total 2, got %d", stats.Hourly[1].TotalQueries)
	}
	if stats.Hourly[1].BlockedQueries != 1 {
		t.Fatalf("expected second bucket blocked 1, got %d", stats.Hourly[1].BlockedQueries)
	}
	if stats.Hourly[1].AllowedQueries != 1 {
		t.Fatalf("expected second bucket allowed 1, got %d", stats.Hourly[1].AllowedQueries)
	}
}

func TestDNSLog_DashboardCacheLifecycle(t *testing.T) {
	connString := newTempConnString(t)
	if err := db.RunMigrations(context.Background(), connString, db.TargetDNSLog); err != nil {
		t.Fatalf("RunMigrations error: %v", err)
	}

	se := syncsqlite.NewSyncExecutor(connString, 1)
	t.Cleanup(se.Close)
	cs := &DNSLog{se: se}
	now := time.Date(2026, time.August, 2, 16, 30, 0, 0, time.UTC)
	currentBucket := dashboardHourBucket(now)

	stmts := make([]*syncsqlite.ExecStmt, 0, dashboardWindowHours+1)
	for offset := -dashboardWindowHours; offset <= 0; offset++ {
		bucket := currentBucket + int64(offset)
		stmts = append(stmts, &syncsqlite.ExecStmt{
			Query: "INSERT INTO tdnslog (dt, client, domain, blocked) VALUES (?, ?, ?, ?)",
			Args:  []any{bucket*nanosecondsPerHour + int64(10*time.Minute), "1.1.1.1", fmt.Sprintf("%d.example.", offset), offset%2 == 0},
		})
	}
	if err := se.SyncExecBulk(stmts); err != nil {
		t.Fatalf("seed dashboard rows: %v", err)
	}

	history, err := cs.GetDashboardHistoryAt(now)
	if err != nil {
		t.Fatalf("GetDashboardHistoryAt error: %v", err)
	}
	if len(history.Hourly) != dashboardHistoryHours {
		t.Fatalf("history bucket count = %d, want %d", len(history.Hourly), dashboardHistoryHours)
	}
	if history.Hourly[0].HourBucket != currentBucket-dashboardHistoryHours {
		t.Fatalf("first history bucket = %d", history.Hourly[0].HourBucket)
	}
	if history.Hourly[len(history.Hourly)-1].HourBucket != currentBucket-1 {
		t.Fatalf("last history bucket = %d", history.Hourly[len(history.Hourly)-1].HourBucket)
	}

	conn := se.GetConn()
	var cached int
	if err := conn.QueryRow("SELECT COUNT(*) FROM dashboard_hourly_stats").Scan(&cached); err != nil {
		se.FreeConn(conn)
		t.Fatalf("count dashboard cache: %v", err)
	}
	se.FreeConn(conn)
	if cached != dashboardHistoryHours {
		t.Fatalf("cached bucket count = %d, want %d", cached, dashboardHistoryHours)
	}

	missingBucket := currentBucket - 5
	if _, err := se.SyncExec("DELETE FROM dashboard_hourly_stats WHERE hour_bucket = ?", []any{missingBucket}); err != nil {
		t.Fatalf("delete cached bucket: %v", err)
	}
	repaired, err := cs.GetDashboardHistoryAt(now)
	if err != nil {
		t.Fatalf("repair dashboard history: %v", err)
	}
	if len(repaired.Hourly) != dashboardHistoryHours {
		t.Fatalf("repaired bucket count = %d, want %d", len(repaired.Hourly), dashboardHistoryHours)
	}

	if _, err := se.SyncExec(
		"INSERT INTO tdnslog (dt, client, domain, blocked) VALUES (?, ?, ?, ?)",
		[]any{(currentBucket-1)*nanosecondsPerHour + int64(20*time.Minute), "1.1.1.1", "late.example.", 0},
	); err != nil {
		t.Fatalf("insert late previous-hour row: %v", err)
	}
	if err := cs.refreshDashboardCacheAt(now); err != nil {
		t.Fatalf("refresh dashboard cache: %v", err)
	}
	refreshed, err := cs.GetDashboardHistoryAt(now)
	if err != nil {
		t.Fatalf("read refreshed dashboard history: %v", err)
	}
	if got := refreshed.Hourly[len(refreshed.Hourly)-1].TotalQueries; got != 2 {
		t.Fatalf("refreshed previous-hour total = %d, want 2", got)
	}

	current, err := cs.GetDashboardCurrentAt(now)
	if err != nil {
		t.Fatalf("GetDashboardCurrentAt error: %v", err)
	}
	if len(current.Hourly) != 1 || current.Hourly[0].HourBucket != currentBucket || current.Summary.TotalQueries != 1 {
		t.Fatalf("unexpected current dashboard stats: %#v", current)
	}

	combined, err := cs.GetDashboardStatsAt(now, dashboardWindowHours)
	if err != nil {
		t.Fatalf("GetDashboardStatsAt error: %v", err)
	}
	if len(combined.Hourly) != dashboardWindowHours {
		t.Fatalf("combined bucket count = %d, want %d", len(combined.Hourly), dashboardWindowHours)
	}
	if combined.Summary.TotalQueries != refreshed.Summary.TotalQueries+current.Summary.TotalQueries {
		t.Fatalf("combined total = %d, want history + current", combined.Summary.TotalQueries)
	}

	if err := cs.rotate(time.Hour); err != nil {
		t.Fatalf("rotate DNS log: %v", err)
	}
	conn = se.GetConn()
	if err := conn.QueryRow("SELECT COUNT(*) FROM dashboard_hourly_stats").Scan(&cached); err != nil {
		se.FreeConn(conn)
		t.Fatalf("count invalidated dashboard cache: %v", err)
	}
	se.FreeConn(conn)
	if cached != 0 {
		t.Fatalf("cached buckets after DNS-log rotation = %d, want 0", cached)
	}
}

func TestDNSLog_GetTopFiltersByStatusAndClient(t *testing.T) {
	connString := newTempConnString(t)
	if err := db.RunMigrations(context.Background(), connString, db.TargetDNSLog); err != nil {
		t.Fatalf("RunMigrations error: %v", err)
	}

	se := syncsqlite.NewSyncExecutor(connString, 1)
	cs := &DNSLog{se: se}

	now := time.Now().UTC().Add(-5 * time.Minute)
	stmts := []*syncsqlite.ExecStmt{
		{
			Query: "INSERT INTO hosts (ipAddr, host) VALUES (?, ?)",
			Args:  []any{"1.1.1.1", "office"},
		},
		{
			Query: "INSERT INTO tdnslog (dt, client, domain, blocked) VALUES (?, ?, ?, ?)",
			Args:  []any{now.UnixNano(), "1.1.1.1", "blocked.example.", 1},
		},
		{
			Query: "INSERT INTO tdnslog (dt, client, domain, blocked) VALUES (?, ?, ?, ?)",
			Args:  []any{now.Add(1 * time.Minute).UnixNano(), "1.1.1.1", "blocked.example.", 1},
		},
		{
			Query: "INSERT INTO tdnslog (dt, client, domain, blocked) VALUES (?, ?, ?, ?)",
			Args:  []any{now.Add(2 * time.Minute).UnixNano(), "1.1.1.1", "allowed.example.", 0},
		},
		{
			Query: "INSERT INTO tdnslog (dt, client, domain, blocked) VALUES (?, ?, ?, ?)",
			Args:  []any{now.Add(3 * time.Minute).UnixNano(), "2.2.2.2", "allowed.example.", 0},
		},
	}
	if err := se.SyncExecBulk(stmts); err != nil {
		t.Fatalf("SyncExecBulk error: %v", err)
	}

	blocked, err := cs.GetTop(10, "24h", "blocked", "", "")
	if err != nil {
		t.Fatalf("GetTop blocked error: %v", err)
	}
	if len(blocked) != 1 {
		t.Fatalf("expected 1 blocked result, got %d", len(blocked))
	}
	if blocked[0].Domain != "blocked.example." || blocked[0].Counter != 2 || blocked[0].Host != "office" {
		t.Fatalf("unexpected blocked result: %#v", blocked[0])
	}

	allowedOffice, err := cs.GetTop(10, "24h", "allowed", "office", "host")
	if err != nil {
		t.Fatalf("GetTop allowed office error: %v", err)
	}
	if len(allowedOffice) != 1 {
		t.Fatalf("expected 1 allowed office result, got %d", len(allowedOffice))
	}
	if allowedOffice[0].Domain != "allowed.example." || allowedOffice[0].Counter != 1 || allowedOffice[0].Host != "office" {
		t.Fatalf("unexpected allowed office result: %#v", allowedOffice[0])
	}

	allowedIP, err := cs.GetTop(10, "24h", "allowed", "2.2.2.2", "ip")
	if err != nil {
		t.Fatalf("GetTop allowed ip error: %v", err)
	}
	if len(allowedIP) != 1 {
		t.Fatalf("expected 1 allowed ip result, got %d", len(allowedIP))
	}
	if allowedIP[0].Host != "2.2.2.2" {
		t.Fatalf("expected unresolved client host to be IP, got %#v", allowedIP[0])
	}
}

func TestDNSLog_SearchClients(t *testing.T) {
	connString := newTempConnString(t)
	if err := db.RunMigrations(context.Background(), connString, db.TargetDNSLog); err != nil {
		t.Fatalf("RunMigrations error: %v", err)
	}

	se := syncsqlite.NewSyncExecutor(connString, 1)
	cs := &DNSLog{se: se}

	now := time.Now().UTC().Add(-5 * time.Minute)
	stmts := []*syncsqlite.ExecStmt{
		{
			Query: "INSERT INTO hosts (ipAddr, host) VALUES (?, ?)",
			Args:  []any{"1.1.1.1", "office"},
		},
		{
			Query: "INSERT INTO tdnslog (dt, client, domain, blocked) VALUES (?, ?, ?, ?)",
			Args:  []any{now.UnixNano(), "1.1.1.1", "example.com.", 0},
		},
		{
			Query: "INSERT INTO tdnslog (dt, client, domain, blocked) VALUES (?, ?, ?, ?)",
			Args:  []any{now.Add(1 * time.Minute).UnixNano(), "2.2.2.2", "example.org.", 0},
		},
	}
	if err := se.SyncExecBulk(stmts); err != nil {
		t.Fatalf("SyncExecBulk error: %v", err)
	}

	clients, err := cs.SearchClients("off", 10)
	if err != nil {
		t.Fatalf("SearchClients alias error: %v", err)
	}
	if len(clients) != 1 || clients[0].Address != "1.1.1.1" || clients[0].Host != "office" {
		t.Fatalf("unexpected alias search result: %#v", clients)
	}

	clients, err = cs.SearchClients("2.2", 10)
	if err != nil {
		t.Fatalf("SearchClients ip error: %v", err)
	}
	if len(clients) != 1 || clients[0].Address != "2.2.2.2" {
		t.Fatalf("unexpected ip search result: %#v", clients)
	}
}
