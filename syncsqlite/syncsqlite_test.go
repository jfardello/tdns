package syncsqlite

import (
	"context"
	"database/sql"
	"fmt"
	"path/filepath"
	"testing"

	"github.com/jfardello/tdns/log"
)

func newTempConnString(t *testing.T) string {
	t.Helper()
	dbPath := filepath.Join(t.TempDir(), "test.db")
	return fmt.Sprintf("file:%s?cache=shared", dbPath)
}

func closeExecutor(t *testing.T, se *SyncExecutor) {
	t.Helper()
	if se.rwConnDatabase != nil {
		_ = se.rwConnDatabase.Close()
	}
	if se.roConnPool == nil {
		return
	}
	for i := 0; i < cap(se.roConnPool); i++ {
		select {
		case conn := <-se.roConnPool:
			_ = conn.Close()
		default:
			return
		}
	}
}

func TestNewSyncExecutor_CreatesSchemaAndSequence(t *testing.T) {
	connString := newTempConnString(t)
	se := NewSyncExecutor(connString, 1)
	defer closeExecutor(t, se)

	for _, table := range []string{"tdnslog", "hosts"} {
		var name string
		err := se.rwConnDatabase.QueryRow(
			"SELECT name FROM sqlite_master WHERE type='table' AND name=?",
			table,
		).Scan(&name)
		if err != nil {
			t.Fatalf("expected table %s, query error: %v", table, err)
		}
		if name != table {
			t.Fatalf("expected table %s, got %s", table, name)
		}
	}

	var seq int
	err := se.rwConnDatabase.QueryRow(
		"SELECT seq FROM sqlite_sequence WHERE name='tdnslog'",
	).Scan(&seq)
	if err != nil {
		t.Fatalf("expected sqlite_sequence row for tdnslog, got error: %v", err)
	}
	if seq != 0 {
		t.Fatalf("expected sequence 0, got %d", seq)
	}
}

func TestSyncExec_InsertsHost(t *testing.T) {
	connString := newTempConnString(t)
	se := NewSyncExecutor(connString, 1)
	defer closeExecutor(t, se)

	_, err := se.SyncExec(
		"INSERT INTO hosts (ipAddr, host) VALUES (?, ?)",
		[]any{"1.1.1.1", "example.test"},
	)
	if err != nil {
		t.Fatalf("SyncExec insert error: %v", err)
	}

	var host string
	err = se.rwConnDatabase.QueryRow(
		"SELECT host FROM hosts WHERE ipAddr=?",
		"1.1.1.1",
	).Scan(&host)
	if err != nil {
		t.Fatalf("expected host row, got error: %v", err)
	}
	if host != "example.test" {
		t.Fatalf("expected host example.test, got %s", host)
	}
}

func TestSyncExec_DrainsBulkQueue(t *testing.T) {
	connString := newTempConnString(t)
	se := NewSyncExecutor(connString, 1)
	defer closeExecutor(t, se)

	se.BulkAdd(&ExecStmt{
		Query: "INSERT INTO hosts (ipAddr, host) VALUES (?, ?)",
		Args:  []any{"1.1.1.1", "alpha.test"},
	})

	_, err := se.SyncExec(
		"INSERT INTO hosts (ipAddr, host) VALUES (?, ?)",
		[]any{"2.2.2.2", "beta.test"},
	)
	if err != nil {
		t.Fatalf("SyncExec bulk drain error: %v", err)
	}

	var count int
	err = se.rwConnDatabase.QueryRow(
		"SELECT COUNT(*) FROM hosts",
	).Scan(&count)
	if err != nil {
		t.Fatalf("expected host count, got error: %v", err)
	}
	if count != 2 {
		t.Fatalf("expected 2 hosts, got %d", count)
	}
}

func TestSyncExec_RollsBackOnError(t *testing.T) {
	connString := newTempConnString(t)
	se := NewSyncExecutor(connString, 1)
	defer closeExecutor(t, se)

	se.BulkAdd(&ExecStmt{
		Query: "INSERT INTO hosts (ipAddr, host) VALUES (?, ?)",
		Args:  []any{"1.1.1.1", "alpha.test"},
	})
	se.BulkAdd(&ExecStmt{
		Query: "INSERT INTO missing_table (id) VALUES (1)",
	})

	_, err := se.SyncExec(
		"INSERT INTO hosts (ipAddr, host) VALUES (?, ?)",
		[]any{"2.2.2.2", "beta.test"},
	)
	if err == nil {
		t.Fatalf("expected SyncExec error from missing table")
	}

	var count int
	err = se.rwConnDatabase.QueryRow(
		"SELECT COUNT(*) FROM hosts",
	).Scan(&count)
	if err != nil {
		t.Fatalf("expected host count, got error: %v", err)
	}
	if count != 0 {
		t.Fatalf("expected 0 hosts after rollback, got %d", count)
	}
}

func TestInitConnectionPool_FillsReadOnlyPool(t *testing.T) {
	connString := newTempConnString(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	se := &SyncExecutor{
		roConnPool:             make(chan *sql.DB, 2),
		syncExecChan:           make(chan *ExecStmt),
		syncResultChan:         make(chan *ExecResult),
		ctx:                    ctx,
		cancel:                 cancel,
		MaxReadOnlyConnections: 2,
		log:                    log.GetLogger("syncsqlite", "test"),
	}
	se.InitConnectionPool(connString)
	defer closeExecutor(t, se)

	_, err := se.rwConnDatabase.Exec("CREATE TABLE IF NOT EXISTS test_table (id INTEGER)")
	if err != nil {
		t.Fatalf("rw connection create table failed: %v", err)
	}

	if got := len(se.roConnPool); got != 2 {
		t.Fatalf("expected 2 read-only connections, got %d", got)
	}

	conn := <-se.roConnPool
	defer func() { se.roConnPool <- conn }()

	var one int
	err = conn.QueryRow("SELECT 1").Scan(&one)
	if err != nil {
		t.Fatalf("read-only connection query failed: %v", err)
	}
	if one != 1 {
		t.Fatalf("expected read-only query to return 1, got %d", one)
	}
}

func TestExecNoTx_AllowsVacuumAndReportsJournalMode(t *testing.T) {
	connString := newTempConnString(t)
	se := NewSyncExecutor(connString, 1)
	defer closeExecutor(t, se)

	mode, err := se.JournalMode()
	if err != nil {
		t.Fatalf("JournalMode error: %v", err)
	}
	if mode != "WAL" {
		t.Fatalf("expected WAL journal mode, got %s", mode)
	}

	if _, err := se.ExecNoTx("VACUUM;", nil); err != nil {
		t.Fatalf("ExecNoTx VACUUM error: %v", err)
	}

	if _, err := se.ExecNoTx("PRAGMA wal_checkpoint(TRUNCATE);", nil); err != nil {
		t.Fatalf("ExecNoTx wal_checkpoint error: %v", err)
	}
}
