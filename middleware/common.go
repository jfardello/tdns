package middleware

import (
	"context"
	"errors"
	"github.com/jfardello/tdns/config"
	"github.com/miekg/dns"
	"slices"
	"sync"
)

type Stage int

const (
	PreRouting Stage = iota
	Resolving
	PostRouting
)

type Middleware interface {
	Run(*Message) (*Message, error)
	Info() (string, Stage)
	Config(config.Config) error
	Init() error
}

type Message struct {
	msg      *dns.Msg
	ctx      context.Context
	mu       sync.Mutex
	resolved bool
}

func (m *Message) Resolved(r bool) {
	m.resolved = r
}

func (m *Message) IsResolved() bool {
	return m.resolved
}

func (m *Message) Context() context.Context {
	return m.ctx
}

// Answer returns a nil value if there is no Msg nor RR attached, Msg has precedence over RR,
// which will be converted to *dns.Msg
func (m *Message) Answer() *dns.Msg {
	if m.msg != nil {
		return m.msg
	}
	return nil
}

func (m *Message) AddValue(key string, value string) error {
	var cv config.CtxValue
	cv, ok := m.Context().Value(config.CtxKey).(config.CtxValue)
	if !ok {
		return errors.New("no context value")
	}
	if cv.Values == nil {
		cv.Values = make(map[string]string)
	}
	cv.Values[key] = value
	m.mu.Lock()
	m.ctx = context.WithValue(m.ctx, config.CtxKey, cv)
	m.mu.Unlock()
	return nil
}

func (m *Message) AddLabels(labels ...string) error {
	cv, ok := m.Context().Value(config.CtxKey).(config.CtxValue)
	if !ok {
		return errors.New("no context value")
	}
	for _, label := range labels {
		if label == "" || slices.Contains(cv.Labels, label) {
			continue
		}
		cv.Labels = append(cv.Labels, label)
	}
	m.mu.Lock()
	m.ctx = context.WithValue(m.ctx, config.CtxKey, cv)
	m.mu.Unlock()
	return nil
}

func (m *Message) GetCtxValue() (value *config.CtxValue, err error) {
	cv, ok := m.Context().Value(config.CtxKey).(config.CtxValue)
	if !ok {
		return nil, errors.New("no context value")
	}
	return &cv, nil
}

func (m *Message) GetValue(key string) (value string, ok bool) {
	cv, err := m.GetCtxValue()
	if err != nil {
		return "", false
	}
	val, ok := cv.Values[key]
	return val, ok
}

func (m *Message) Labels() []string {
	cv, err := m.GetCtxValue()
	if err != nil {
		return nil
	}
	return append([]string(nil), cv.Labels...)
}

func (m *Message) HasLabel(label string) bool {
	for _, each := range m.Labels() {
		if each == label {
			return true
		}
	}
	return false
}

func (m *Message) GetMsg() (*dns.Msg, error) {
	if m.msg != nil {
		return m.msg, nil
	}
	return nil, errors.New("no msg")
}

func (m *Message) SetMsg(msg *dns.Msg) {
	m.mu.Lock()
	m.msg = msg
	m.mu.Unlock()
}

func (m *Message) SetCtx(ctx context.Context) {
	m.mu.Lock()
	m.ctx = ctx
	m.mu.Unlock()
}
