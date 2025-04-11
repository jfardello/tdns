package plugin

import (
	"github.com/jmoiron/sqlx"
	"net"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/jfardello/tdns/config"
	"github.com/miekg/dns"
)

func TestDNSLog_doInsert(t *testing.T) {
	c := new(dns.Client)
	cf, _ := dns.ClientConfigFromFile("/etc/resolv.conf")
	m := new(dns.Msg)
	m.SetQuestion(dns.Fqdn("www.google.com"), dns.TypeANY)
	r, _, err := c.Exchange(m, net.JoinHostPort(cf.Servers[0], cf.Port))

	addr, _ := net.ResolveUDPAddr("udp", "1.1.1.1:53")
	ctx1 := config.CtxValue{
		RemoteAddr: addr,
	}
	tests := []LogEvent{
		{
			Timestamp: time.Now(),
			Msg:       r,
			CtxValue:  ctx1,
		},
	}

	cs := &DNSLog{}
	mockDB, mock, err := sqlmock.New()
	sqlxDB := sqlx.NewDb(mockDB, "sqlmock")

	mock.ExpectBegin()
	mock.ExpectExec("INSERT INTO tdnslog").WithArgs(tests[0].Timestamp.UnixNano(), "1.1.1.1", "www.google.com.").WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectCommit()

	t.Run("test insert", func(t *testing.T) {
		cs.queue = tests
		cs.db = sqlxDB
		cs.doInsert()
	})
	defer func() {
		_ = sqlxDB.Close()
	}()
	err = mock.ExpectationsWereMet()
	if err != nil {
		t.Errorf("there were unfulfilled expectations: %s", err)
	}
}
