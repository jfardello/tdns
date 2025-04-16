package plugin

import (
	"context"
	"fmt"
	"github.com/jfardello/tdns/config"
	"github.com/jfardello/tdns/log"
	"github.com/miekg/dns"
	"net"
	"sync"
	"time"

	"github.com/jmoiron/sqlx"
	_ "github.com/mattn/go-sqlite3"
)

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

type LogEvent struct {
	Timestamp time.Time
	CtxValue  config.CtxValue
	Msg       *dns.Msg
}

type DNSLog struct {
	db *sqlx.DB

	duration     time.Duration
	done         chan bool
	mustExit     chan bool
	full         chan bool
	incomingData chan LogEvent
	queue        []LogEvent
	mu           sync.Mutex
}

func (cs *DNSLog) Run(ctx context.Context, m *dns.Msg) (*dns.RR, bool, error) {
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

	return nil, false, nil
}

func (cs *DNSLog) Append(d LogEvent) {
	if !cs.put(d) {
		//Notify that buffer is full
		//<- will wait until space available
		cs.full <- true
		cs.incomingData <- d
	}
}

func (cs *DNSLog) put(d LogEvent) bool {
	//Try to append the data
	//If channel is full, do nothing, then return false
	select {
	case cs.incomingData <- d:
		return true
	default:
		//channel is full
		return false
	}
}

func getEventDomain(c config.CtxValue, m *dns.Msg) (string, string, bool) {
	//Extract details from CtxValue and Msg
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

func (cs *DNSLog) doInsert() {
	logger := log.GetLogger("DNSLog", "doInsert")
	logger.Debugf("dnslog: %#v", cs)
	if len(cs.queue) > 0 {
		tx, err := cs.db.Begin()
		if err != nil {
			logger.Fatal(err)
		}
		cs.mu.Lock()
		for _, logEvent := range cs.queue {
			logger.Debugf("Incoming message: %#v", logEvent)
			logger.Debugf("  Message unix nano: %d", logEvent.Timestamp.UTC().UnixNano())
			client, domain, _ := getEventDomain(logEvent.CtxValue, logEvent.Msg)
			cs.db.MustExec("INSERT INTO tdnslog (dt, client, domain) VALUES (MAX(?, (SELECT seq FROM sqlite_sequence) + 1), ?, ?)",
				logEvent.Timestamp.UTC().UnixNano(), client, domain)

		}
		err = tx.Commit()
		if err != nil {
			logger.Fatal(err)
		}
		cs.queue = nil
		cs.mu.Unlock()
	}
}

func (cs *DNSLog) append() {
	for {
		logEvent, closed := <-cs.incomingData
		if !closed {
			return
		}

		cs.mu.Lock()
		cs.queue = append(cs.queue, logEvent)
		cs.mu.Unlock()
	}
}

func (cs *DNSLog) runAsync() {
	defer func() {
		cs.doInsert()
		fmt.Println("Last insert")
		close(cs.done)
	}()
	timer := time.NewTimer(cs.duration)
	time.Sleep(2 * time.Second)
	for {
		select {
		case <-timer.C:
			cs.doInsert()
			fmt.Println("Timer Expired")
			timer.Reset(cs.duration)
		case <-cs.full:
			if !timer.Stop() {
				<-timer.C
			}
			cs.doInsert()
			fmt.Println("Full")
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
	if !cf.DNSLog.Enabled {
		return nil
	}
	logger := log.GetLogger("DNSLog", "Config")
	logger.Debug(">>>> Calling config!")
	//connect to db
	var err error
	cs.db, err = sqlx.Open("sqlite3", cf.DNSLog.File)
	if err != nil {
		logger.Error(err)
		return err
	}

	// Create logs table if not exists
	schema := `create table if not exists tdnslog ( dt INTEGER primary key autoincrement, domain  TEXT, client  TEXT, blocked INT default 0);`
	schemaHosts := `create table if not exists hosts ( ipAddr TEXT not null constraint hosts_pk primary key, host   text );`
	sqlSequence := `INSERT into sqlite_sequence (seq,name) VALUES (0, 'tdnslog');`
	if _, err := cs.db.Exec(schema); err != nil {
		logger.Error(err)
		return err
	}
	if _, err := cs.db.Exec(schemaHosts); err != nil {
		logger.Error(err)
		return err
	}
	if _, err := cs.db.Exec(sqlSequence); err != nil {
		logger.Error(err)
		return err
	}

	cs.incomingData = make(chan LogEvent, 200)
	cs.duration = 5 * time.Second
	cs.full = make(chan bool)
	cs.done = make(chan bool, 1)
	cs.mustExit = make(chan bool, 1)

	return nil

}

func (cs *DNSLog) Init() error {
	go cs.append()
	go cs.runAsync()
	return nil
}

func (cs *DNSLog) Stop() {
	cs.mustExit <- true
}

func (cs *DNSLog) WaitDone() {
	<-cs.done
}
