package middleware

import (
	"context"
	"github.com/jfardello/tdns/config"
	"github.com/miekg/dns"
	"net"
	"slices"
	"strings"
	"sync"
	"testing"
)

func TestMessage_GetCtxValue(t *testing.T) {
	addr, _ := net.ResolveTCPAddr("tcp", "127.0.0.1:9999")
	c := context.WithValue(context.Background(), config.CtxKey, config.CtxValue{
		RemoteAddr: addr,
		Values: map[string]string{
			"middleware/foo": "bar",
		},
	})
	type fields struct {
		msg *dns.Msg
		ctx context.Context
	}
	tests := []struct {
		name      string
		fields    fields
		wantValue string
		wantErr   bool
	}{
		{name: "all_nil", fields: fields{msg: nil, ctx: c}, wantValue: "bar"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := &Message{msg: tt.fields.msg, ctx: tt.fields.ctx, mu: sync.Mutex{}}
			gotValue, err := m.GetCtxValue()
			if (err != nil) != tt.wantErr {
				t.Errorf("GetCtxValue() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if gotValue.Values["middleware/foo"] != tt.wantValue {
				t.Errorf("GetCtxValue() want = %s, not in : %+v", tt.wantValue, gotValue.Values)
			}

		})
	}
}

func TestMessage_AddCtxValue(t *testing.T) {
	addr, _ := net.ResolveTCPAddr("tcp", "127.0.0.1:9999")
	c := context.WithValue(context.Background(), config.CtxKey, config.CtxValue{
		RemoteAddr: addr,
		Values: map[string]string{
			"middleware/foo": "bar",
		},
	})
	type fields struct {
		rr  *dns.RR
		msg *dns.Msg
		ctx context.Context
	}
	tests := []struct {
		name      string
		fields    fields
		wantValue string
		wantErr   bool
	}{
		{name: "all_nil", fields: fields{rr: nil, msg: nil, ctx: c}, wantValue: "bar"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := &Message{msg: tt.fields.msg, ctx: tt.fields.ctx, mu: sync.Mutex{}}
			_ = m.AddValue(tt.wantValue, tt.wantValue)
			gotValue, err := m.GetCtxValue()
			if (err != nil) != tt.wantErr {
				t.Errorf("GetCtxValue() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if gotValue.Values[tt.wantValue] != tt.wantValue {
				t.Errorf("GetCtxValue() want = %s, not in : %+v", tt.wantValue, gotValue.Values)
			}

		})
	}
}

func TestMessage_AddLabels(t *testing.T) {
	addr, _ := net.ResolveTCPAddr("tcp", "127.0.0.1:9999")
	c := context.WithValue(context.Background(), config.CtxKey, config.CtxValue{
		RemoteAddr: addr,
		Labels:     []string{"existing"},
		Values:     map[string]string{},
	})

	m := &Message{ctx: c, mu: sync.Mutex{}}
	if err := m.AddLabels("existing", "family", "kids"); err != nil {
		t.Fatalf("AddLabels error: %v", err)
	}

	got := m.Labels()
	want := []string{"existing", "family", "kids"}
	if !slices.Equal(got, want) {
		t.Fatalf("Labels got %v, want %v", got, want)
	}
	if !m.HasLabel("family") {
		t.Fatal("expected HasLabel(family) to be true")
	}
}

func TestMessage_Answer(t *testing.T) {
	addr, _ := net.ResolveTCPAddr("tcp", "127.0.0.1:9999")
	c := context.WithValue(context.Background(), config.CtxKey, config.CtxValue{
		RemoteAddr: addr,
		Values: map[string]string{
			"middleware/foo": "bar",
		},
	})

	rr, _ := dns.NewRR("tdns.org. A 2.2.2.2")
	msg := new(dns.Msg)
	msg.Answer = append(msg.Answer, rr)

	type fields struct {
		msg *dns.Msg
		ctx context.Context
	}
	tests := []struct {
		name      string
		fields    fields
		wantValue string
		wantNil   bool
	}{
		{name: "nil", fields: fields{msg: nil, ctx: c}, wantNil: true},
		{name: "msg", fields: fields{msg: msg, ctx: c}, wantValue: "2.2.2.2"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := &Message{msg: tt.fields.msg, ctx: tt.fields.ctx, mu: sync.Mutex{}}
			msg := m.Answer()
			if tt.wantNil && msg != nil {
				t.Errorf("Answer() msg = %v, wantErr %v", msg, tt.wantNil)
				return
			}
			if tt.wantNil && msg == nil {
				t.Logf("Answer() msg = %v, wantNil %v", msg, tt.wantNil)
			} else {
				answer := m.Answer()
				if !strings.Contains(answer.Answer[0].String(), tt.wantValue) {
					t.Errorf("Answer() want = %v, got: %v", tt.wantValue, msg.Answer)
				}
			}
		})
	}
}
