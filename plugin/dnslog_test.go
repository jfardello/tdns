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
