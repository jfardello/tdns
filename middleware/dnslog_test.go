package middleware

import (
	"fmt"
	"net"
	"path/filepath"
	"testing"
	"time"

	"github.com/jfardello/tdns/config"
	"github.com/jfardello/tdns/syncsqlite"
	"github.com/miekg/dns"
)

func newTempConnString(t *testing.T) string {
	t.Helper()
	dbPath := filepath.Join(t.TempDir(), "test.db")
	return fmt.Sprintf("file:%s?cache=shared", dbPath)
}

func TestDNSLog_doInsert(t *testing.T) {
	m := new(dns.Msg)
	m.SetQuestion(dns.Fqdn("www.google.com"), dns.TypeA)

	addr, _ := net.ResolveUDPAddr("udp", "1.1.1.1:53")
	ctx1 := config.CtxValue{
		RemoteAddr: addr,
		Values:     map[string]string{},
	}
	ts := time.Unix(1700000000, 123456789)
	tests := []LogEvent{
		{
			Timestamp: ts,
			Msg:       m,
			CtxValue:  ctx1,
		},
	}

	se := syncsqlite.NewSyncExecutor(newTempConnString(t), 1)
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
