package plugin

import (
	"context"
	"errors"
	"fmt"
	"net"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/jfardello/tdns/config"
	"github.com/jfardello/tdns/log"
	"github.com/jfardello/tdns/sched"
	"github.com/jfardello/tdns/syncsqlite"
	"github.com/miekg/dns"
	str2duration "github.com/xhit/go-str2duration/v2"

	"github.com/jmoiron/sqlx"
	_ "github.com/mattn/go-sqlite3"
)

// RIPPER_FUZZINESS defines the maximum wait time for dnslog's ripper tasks,
// it will wait randomly from 0 to this value before starting periodic tasks,
// this prevents jamming CPU intensive tasks at certain hours.
const RIPPER_FUZZINESS = 30

func DNSErrorString(errorCode int) string {
	switch errorCode {
	case dns.RcodeFormatError:
		return "FormatError"
	case dns.RcodeServerFailure:
		return "ServerFailure"
	case dns.RcodeNameError:
		return "NameError"
	case dns.RcodeNotImplemented:
		return "NotImplemented"
	case dns.RcodeRefused:
		return "Refused"
	case dns.RcodeYXDomain:
		return "YXDomain"
	case dns.RcodeYXRrset:
		return "YXRrset"
	case dns.RcodeNXRrset:
		return "NXRrset"
	case dns.RcodeNotAuth:
		return "NotAuth"
	case dns.RcodeNotZone:
		return "NotZone"
	case dns.RcodeBadSig:
		return "BadSig"
	case dns.RcodeBadKey:
		return "BadKey"
	case dns.RcodeBadTime:
		return "BadTime"
	case dns.RcodeBadMode:
		return "BadMode"
	case dns.RcodeBadName:
		return "BadName"
	case dns.RcodeBadAlg:
		return "BadAlg"
	case dns.RcodeBadTrunc:
		return "BadTrunc"
	case dns.RcodeBadCookie:
		return "BadCookie"
	}
	return fmt.Sprintf("Unknown error %d", errorCode)
}

type SQLStmt struct {
	SelectStr string
	From      string
	Where     string
	OrderBy   string
	Limit     string
	GroupBy   string
}

// Build is a naive Sql statement builder for this use case, it handles conditional where clause in the SQLStmt type
func (sq *SQLStmt) Build() (string, error) {
	logger := log.GetLogger("dbslog", "SQLStmt")
	var sb strings.Builder
	if sq.SelectStr != "" {
		sb.WriteString(sq.SelectStr)
		sb.WriteString(" ")
	} else {
		return "", errors.New("selectStr is empty")
	}
	if sq.From != "" {
		sb.WriteString(sq.From)
		sb.WriteString(" ")
	} else {
		return "", errors.New("from is empty")
	}
	sb.WriteString(sq.Where)
	sb.WriteString(" ")
	sb.WriteString(sq.GroupBy)
	sb.WriteString(" ")
	sb.WriteString(sq.OrderBy)
	sb.WriteString(" ")
	sb.WriteString(sq.Limit)

	statement := strings.TrimSpace(sb.String())
	logger.Info(statement)
	return statement, nil
}

var MaxDNSLogEntries int = 50

type LogEvent struct {
	Timestamp time.Time
	CtxValue  config.CtxValue
	Msg       *dns.Msg
}

type DNSLog struct {
	se           *syncsqlite.SyncExecutor
	duration     time.Duration
	purge        chan time.Duration
	done         chan bool
	mustExit     chan bool
	full         chan bool
	incomingData chan LogEvent
	queue        []LogEvent
	mu           sync.Mutex
	qmu          sync.Mutex
	ctx          context.Context
	cancel       context.CancelFunc
}

func (cs *DNSLog) Run(mess *Message) (*Message, error) {
	m, err := mess.GetMsg()
	if err != nil {
		return mess, err
	}
	ctx := mess.Context()
	logger := log.GetLogger("DNSLog", "Run")
	cv, ok := ctx.Value(config.CtxKey).(config.CtxValue)
	if ok {
		cs.Append(LogEvent{
			Timestamp: time.Now(),
			CtxValue:  cv,
			Msg:       m,
		})
	} else {
		logger.Debugf("#### Domain: %#v", m)
	}
	return mess, nil
}

func (cs *DNSLog) Append(d LogEvent) {
	logger := log.GetLogger("DNSLog", "Append")
	logger.Debug("Append(): calling cs.put()")
	if !cs.put(d) {
		logger.Debug("Append(): cs.put() returned false, waiting")
		//Notify that buffer is full
		//<- will wait until space available
		cs.full <- true
		cs.incomingData <- d
		logger.Debug("Append(): directly wrote to incomingData channel")
	}
}

func (cs *DNSLog) put(d LogEvent) bool {
	//Try to append the data
	//If channel is full, do nothing, then return false
	logger := log.GetLogger("DNSLog", "put")
	logger.Debug("called")
	select {
	case cs.incomingData <- d:
		logger.Debug("put() sent")
		return true
	default:
		//channel is full
		logger.Debug("put() failed, channel is full.")
		return false
	}
}

func getEventDomain(c config.CtxValue, m *dns.Msg) (string, string, bool) {
	//Extract details from CtxValue and msg
	domain := m.Question[0].Name
	ok := false
	logger := log.GetLogger("DNSLog", "getEventDomain")
	logger.Debugf("Getting  message: %#v", m)
	if m.MsgHdr.Rcode == dns.RcodeSuccess {
		if len(m.Answer) > 0 {
			domain = m.Answer[0].Header().Name
			ok = true
		}
	}
	var remote string
	switch addr := c.RemoteAddr.(type) {
	case *net.UDPAddr:
		remote = addr.IP.String()
	case *net.TCPAddr:
		remote = addr.IP.String()
	}
	return remote, domain, ok
}

type LogDetails struct {
	Domain  string `db:"domain"`
	Counter int    `db:"counter"`
	Host    string `db:"host"`
}

func (cs *DNSLog) AddAlias(alias string, addr string) error {
	ip := net.ParseIP(addr)

	if ip == nil {
		return fmt.Errorf("invalid address: %s", addr)
	}
	sql := `INSERT INTO hosts (host, ipAddr) values (?, ?) ON CONFLICT DO UPDATE SET host=excluded.host`

	_, err := cs.se.SyncExec(sql, []any{alias, ip.String()})
	if err != nil {
		return fmt.Errorf("error adding alias, %w", err)
	}
	return nil
}

//Todo Add a Query(domain, start) method  and expose it in the API

func (cs *DNSLog) GetTop(top int, since string) ([]LogDetails, error) {
	logger := log.GetLogger("DNSLog", "GetTop")
	where := ""
	if since != "" {
		d, err := str2duration.ParseDuration(since)
		if err != nil {
			return nil, err
		}
		//ToDo: d.Nanoseconds() should be passed as an arguemnt.
		where = fmt.Sprintf("Where (d.dt between  unixepoch()*1000000000 - %d  and unixepoch()*1000000000)", d.Nanoseconds())
	}
	ss := SQLStmt{
		SelectStr: `SELECT d.domain, COUNT(d.domain) AS counter,  COALESCE(h.host, d.client) AS host`,
		From:      `FROM tdnslog d LEFT JOIN hosts h ON d.client == h.ipAddr`,
		Where:     where,
		GroupBy:   "GROUP BY d.domain, d.client",
		OrderBy:   "ORDER BY counter DESC",
		Limit:     "LIMIT ?",
	}
	sql, err := ss.Build()
	if err != nil {
		return nil, err
	}
	if top > MaxDNSLogEntries {
		top = MaxDNSLogEntries
	}
	dest := make([]LogDetails, 0)

	db := cs.se.GetConn()
	dbx := sqlx.NewDb(db, "sqlite3")
	dbl := &log.SQLLogger{
		Queryer: dbx, Logger: logger, DebugSql: true,
	}

	defer func() {
		cs.se.FreeConn(db)
	}()
	err = sqlx.Select(dbl, &dest, sql, top)
	if err != nil {
		return dest, err
	}
	return dest, nil
}

func (cs *DNSLog) doInsert() {
	logger := log.GetLogger("DNSLog", "doInsert")
	if len(cs.queue) > 0 {
		cs.mu.Lock()
		stmts := make([]*syncsqlite.ExecStmt, 0, len(cs.queue))
		for _, logEvent := range cs.queue {
			logger.Debugf("Incoming message: %#v", logEvent)
			logger.Debugf("  Message unix nano: %d", logEvent.Timestamp.UTC().UnixNano())
			client, domain, _ := getEventDomain(logEvent.CtxValue, logEvent.Msg)
			blocked := 0
			_blocked, ok := logEvent.CtxValue.Values["blocked"]
			if ok {
				var err error
				blocked, err = strconv.Atoi(_blocked)
				if err != nil {
					logger.Errorf("Error parsing blocked value from context: %s", err)
					blocked = 0
				}
			} else {
				blocked = 0
			}
			stmts = append(stmts, &syncsqlite.ExecStmt{
				Query: "INSERT INTO tdnslog (dt, client, domain, blocked) VALUES (MAX(?, (SELECT seq FROM sqlite_sequence) + 1), ?, ?, ?)",
				Args:  []any{logEvent.Timestamp.UTC().UnixNano(), client, domain, blocked},
			})
		}
		if err := cs.se.SyncExecBulk(stmts); err != nil {
			logger.Error(err)
			cs.mu.Unlock()
			return
		}
		cs.queue = nil
		cs.mu.Unlock()
	}
}

func (cs *DNSLog) append() {
	logger := log.GetLogger("DNSLog", "append")
	for {
		logEvent, closed := <-cs.incomingData
		logger.Debugf("append() got: %#v from channel", logEvent)

		if !closed {
			return
		}
		cs.qmu.Lock()
		cs.queue = append(cs.queue, logEvent)
		cs.qmu.Unlock()
	}
}

func (cs *DNSLog) runAsync() {
	logger := log.GetLogger("DNSLog", "runAsync")
	defer func() {
		logger.Debug("Writing last entries")
		cs.doInsert()
		close(cs.done)
	}()
	timer := time.NewTimer(cs.duration)
	time.Sleep(2 * time.Second)
	for {
		select {
		case <-timer.C:
			cs.doInsert()
			timer.Reset(cs.duration)
		case <-cs.full:
			if !timer.Stop() {
				<-timer.C
			}
			cs.doInsert()
			timer.Reset(cs.duration)
		case s := <-cs.purge:
			err := cs.rotate(s)
			if err != nil {
				//Todo: dunno if recoverable.
				logger.Error("$$ ", err)
			}
			timer.Reset(cs.duration)
		case <-cs.mustExit:
			if !timer.Stop() {
				<-timer.C
			}
			return
		}
	}
}

func (cs *DNSLog) Info() (string, Ptype) {
	return "dnslog", PostRouting
}
func (cs *DNSLog) Config(cf config.Config) error {
	logger := log.GetLogger("DNSLog", "Config")
	if !cf.DNSLog.Enabled {
		logger.Debug("DNSLog disabled")
		return nil
	}
	if cf.DNSLog.File == "" {
		return errors.New("dnslog file is empty")
	}
	cs.se = syncsqlite.NewSyncExecutor(syncsqlite.ConnString(cf.DNSLog.File), syncsqlite.MaxReadonlyConnections)

	cs.incomingData = make(chan LogEvent, 200)
	cs.duration = 5 * time.Second
	cs.purge = make(chan time.Duration, 1)
	cs.full = make(chan bool)
	cs.done = make(chan bool, 1)
	cs.mustExit = make(chan bool, 1)
	cs.ctx, cs.cancel = context.WithCancel(context.Background())

	return nil
}

func (cs *DNSLog) Rotate(since string) error {
	s, err := str2duration.ParseDuration(since)
	if err != nil {
		return err
	}
	cs.purge <- s
	return nil

}

func (cs *DNSLog) rotate(since time.Duration) error {
	logger := log.GetLogger("DNSLog", "rotate")
	cs.mu.Lock()
	defer cs.mu.Unlock()

	logger.Debug("Rotating db")
	now := time.Now().UTC().UnixNano()

	res, err := cs.se.SyncExec("DELETE from tdnslog Where (tdnslog.dt < ?);", []any{now - since.Nanoseconds()})

	if err != nil {
		logger.Error(err)
		return err
	}
	ra, err := res.RowsAffected()

	if err != nil {
		logger.Error(err)
		return err
	}
	logger.Debugf("Deleted %d rows", ra)

	_, err = cs.se.ExecNoTx("VACUUM;", nil)
	if err != nil {
		logger.Error(err)
		return err
	}

	mode, err := cs.se.JournalMode()
	if err != nil {
		logger.Error(err)
		return err
	}
	if mode == "WAL" {
		_, err = cs.se.ExecNoTx("PRAGMA wal_checkpoint(TRUNCATE);", nil)
		if err != nil {
			logger.Error(err)
			return err
		}
	}

	return nil
}

func (cs *DNSLog) Init() error {
	cf := config.GetRunningConfig()
	mf := int64(RIPPER_FUZZINESS * time.Second)
	t := sched.Task{
		Name: "DnsLogRipper",
		Fn: sched.FuzzyTask("DnsLogRipper", cs.ctx, mf, func(context.Context) {
			err := cs.Rotate(cf.DNSLog.Purge)
			if err != nil {
				log.GetLogger("DNSLog", "DnsLogRipper").Error(err)
			}
		}),
		Expr: "0 0 * * *",
	}
	sched.Add(t)
	go cs.append()
	go cs.runAsync()
	return nil
}

func (cs *DNSLog) Stop() {
	logger := log.GetLogger("DNSLog", "Stop")
	logger.Info("Stopping dnslog storage")
	if cs.mustExit != nil {
		cs.mustExit <- true
	}
	time.Sleep(30 * time.Millisecond)
	if cs.se != nil {
		cs.se.Close()
	}
}

func (cs *DNSLog) WaitDone() {
	<-cs.done
}
