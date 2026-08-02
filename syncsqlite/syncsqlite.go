package syncsqlite

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"sync"

	"github.com/jfardello/tdns/internal/sqliteutil"
	"github.com/jfardello/tdns/log"
	"github.com/sirupsen/logrus"
)

const MaxReadonlyConnections = 10
const DefaultBulkQueueSize = 1024

type SyncExecutor struct {
	roConnPool             chan *sql.DB //RO channel that holds all the RO connections
	rwConnDatabase         *sql.DB      //RW connection to the DB
	mu                     sync.Mutex
	syncExecChan           chan *ExecStmt
	syncResultChan         chan *ExecResult
	bulkExecChan           chan *ExecStmt
	ctx                    context.Context
	cancel                 context.CancelFunc
	MaxReadOnlyConnections int
	initDone               bool
	log                    *logrus.Entry
}

type ExecStmt struct {
	Query string
	Args  []any
}

type ExecResult struct {
	Result *sql.Result
	Err    error
}

func (se *SyncExecutor) Mock(db *sql.DB) {
	se.rwConnDatabase = db
}

func (se *SyncExecutor) GetConn() *sql.DB {
	return <-se.roConnPool
}

func (se *SyncExecutor) FreeConn(c *sql.DB) {
	se.roConnPool <- c
}

// SyncExec locks and sends to the sync serial writer for avoiding multi-thread write errors.
func (se *SyncExecutor) SyncExec(query string, args []any) (sql.Result, error) {
	se.mu.Lock()
	defer se.mu.Unlock()

	stmts := make([]*ExecStmt, 0, len(se.bulkExecChan)+1)
drainBulk:
	for {
		select {
		case stmt := <-se.bulkExecChan:
			if stmt != nil {
				stmts = append(stmts, stmt)
			}
		default:
			break drainBulk
		}
	}
	stmts = append(stmts, &ExecStmt{Query: query, Args: args})
	if err := se.execLocked("BEGIN", nil); err != nil {
		return nil, err
	}
	var lastResult sql.Result
	for _, stmt := range stmts {
		if stmt == nil {
			continue
		}
		result, err := se.execLockedResult(stmt.Query, stmt.Args)
		if err != nil {
			if rollbackErr := se.execLocked("ROLLBACK", nil); rollbackErr != nil {
				return nil, fmt.Errorf("exec failed: %v, rollback failed: %w", err, rollbackErr)
			}
			return nil, err
		}
		lastResult = result
	}
	if err := se.execLocked("COMMIT", nil); err != nil {
		if rollbackErr := se.execLocked("ROLLBACK", nil); rollbackErr != nil {
			return nil, fmt.Errorf("commit failed: %v, rollback failed: %w", err, rollbackErr)
		}
		return nil, err
	}
	return lastResult, nil
}

// ExecNoTx executes a single statement without wrapping it in BEGIN/COMMIT.
func (se *SyncExecutor) ExecNoTx(query string, args []any) (sql.Result, error) {
	se.mu.Lock()
	defer se.mu.Unlock()
	return se.execLockedResult(query, args)
}

// SyncExecBulk executes multiple statements inside a single transaction.
func (se *SyncExecutor) SyncExecBulk(stmts []*ExecStmt) error {
	if len(stmts) == 0 {
		return nil
	}
	se.mu.Lock()
	defer se.mu.Unlock()

	if err := se.execLocked("BEGIN", nil); err != nil {
		return err
	}
	for _, stmt := range stmts {
		if stmt == nil {
			continue
		}
		if err := se.execLocked(stmt.Query, stmt.Args); err != nil {
			if rollbackErr := se.execLocked("ROLLBACK", nil); rollbackErr != nil {
				return fmt.Errorf("bulk exec failed: %v, rollback failed: %w", err, rollbackErr)
			}
			return err
		}
	}
	if err := se.execLocked("COMMIT", nil); err != nil {
		if rollbackErr := se.execLocked("ROLLBACK", nil); rollbackErr != nil {
			return fmt.Errorf("commit failed: %v, rollback failed: %w", err, rollbackErr)
		}
		return err
	}
	return nil
}

func (se *SyncExecutor) Call(stmt *ExecStmt) {
	se.syncExecChan <- stmt
}

func (se *SyncExecutor) execLocked(query string, args []any) error {
	_, err := se.execLockedResult(query, args)
	return err
}

func (se *SyncExecutor) execLockedResult(query string, args []any) (sql.Result, error) {
	s := &ExecStmt{
		Query: query,
		Args:  args,
	}
	se.Call(s)
	return se.getResponse()
}

func (se *SyncExecutor) getResponse() (sql.Result, error) {
	resp := <-se.syncResultChan
	return *resp.Result, resp.Err

}

func (se *SyncExecutor) Close() {
	se.cancel()
}

func (se *SyncExecutor) JournalMode() (string, error) {
	se.mu.Lock()
	defer se.mu.Unlock()

	var mode string
	err := se.rwConnDatabase.QueryRowContext(se.ctx, "PRAGMA journal_mode;").Scan(&mode)
	if err != nil {
		return "", err
	}
	return strings.ToUpper(mode), nil
}

func (se *SyncExecutor) Run() {

	for {
		select {
		case <-se.ctx.Done():
			if se.rwConnDatabase != nil {
				_ = se.rwConnDatabase.Close()
			}
			return
		case stmt := <-se.syncExecChan:
			se.log.WithField("stmt", stmt.Query).Debug("Running query")
			result, err := se.rwConnDatabase.ExecContext(se.ctx, stmt.Query, stmt.Args...)

			if err != nil {
				se.log.Errorf("Error executing query: %s, %v", stmt.Query, err)
			}
			r := &ExecResult{
				Result: &result,
				Err:    err,
			}
			se.syncResultChan <- r
		}
	}

}

// BulkAdd adds a statement to the pending bulk queue to be executed on the next SyncExec.
func (se *SyncExecutor) BulkAdd(stmt *ExecStmt) {
	if stmt == nil {
		return
	}
	if se.bulkExecChan == nil {
		return
	}
	se.bulkExecChan <- stmt
}

func (se *SyncExecutor) InitConnectionPool(connString string) {
	if !se.initDone {
		conn, err := sql.Open(sqliteutil.DriverName(), sqliteutil.ReadWriteDSN(connString))
		if err != nil {
			se.log.Fatalf("Error opening connection pool: %v", err)
		}
		if err := sqliteutil.ConfigureConnection(se.ctx, conn, true); err != nil {
			se.log.Fatalf("Error configuring rw connection pool: %v", err)
		}
		se.rwConnDatabase = conn
		for range se.MaxReadOnlyConnections {
			conn, err := sql.Open(sqliteutil.DriverName(), sqliteutil.ReadOnlyDSN(connString))
			if err != nil {
				panic(err)
			}
			if err := sqliteutil.ConfigureConnection(se.ctx, conn, false); err != nil {
				panic(err)
			}
			se.roConnPool <- conn
		}
		se.initDone = true
	}
}

func ConnString(path string) string {
	return sqliteutil.DSN(path)
}

func NewSyncExecutor(connString string, maxReadOnlyConnections int) *SyncExecutor {
	ctx, cancel := context.WithCancel(context.Background())
	logger := log.GetLogger("synsqlite", "executor")

	executor := SyncExecutor{
		roConnPool:             make(chan *sql.DB, maxReadOnlyConnections),
		syncExecChan:           make(chan *ExecStmt),
		syncResultChan:         make(chan *ExecResult),
		bulkExecChan:           make(chan *ExecStmt, DefaultBulkQueueSize),
		ctx:                    ctx,
		cancel:                 cancel,
		MaxReadOnlyConnections: maxReadOnlyConnections,
		log:                    logger,
	}
	executor.InitConnectionPool(connString)
	go executor.Run()
	return &executor
}
