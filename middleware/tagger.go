package middleware

import (
	"errors"
	"net"
	"strings"

	"github.com/jfardello/tdns/config"
	"github.com/jfardello/tdns/log"
	"github.com/jfardello/tdns/storage"
	"github.com/jfardello/tdns/syncsqlite"
)

type Tagger struct {
	dbFile  string
	enabled bool
	storage storage.Storage
}

func remoteAddressKey(addr net.Addr) string {
	switch typed := addr.(type) {
	case *net.UDPAddr:
		return typed.IP.String()
	case *net.TCPAddr:
		return typed.IP.String()
	default:
		if addr == nil {
			return ""
		}
		return addr.String()
	}
}

func (t *Tagger) Close() error {
	if t.storage == nil {
		return nil
	}
	return t.storage.Close()
}

func (t *Tagger) Run(message *Message) (*Message, error) {
	logger := log.GetLogger("middleware.Tagger", "Run")
	if err := t.ensureReady(); err != nil {
		return message, nil
	}
	cv, ok := message.ctx.Value(config.CtxKey).(config.CtxValue)
	if !ok {
		return message, nil
	}

	address := remoteAddressKey(cv.RemoteAddr)
	if address == "" {
		return message, nil
	}

	tags, err := t.storage.GetMemberLabels(address)
	if err != nil {
		logger.Errorf("Get tags error: %s", err)
		return message, nil
	}
	if err := message.AddValue("tags", strings.Join(tags, ",")); err != nil {
		logger.Errorf("can't add tags to context for address: %s. Err:%s", address, err)
	}
	return message, nil
}

func (t *Tagger) Info() (string, Stage) {
	return "tagger", PreRouting
}

func (t *Tagger) Config(cf config.Config) error {
	if cf.Tagger.Enabled {
		t.dbFile = cf.Tagger.File
		t.enabled = true
	}
	return nil
}

func (t *Tagger) Init() error {
	if !t.enabled {
		return nil
	}

	store, err := syncsqlite.NewSQLiteStorage(storage.WithDbPath(t.dbFile))
	if err != nil {
		return err
	}
	t.storage = store
	return nil
}

func (t *Tagger) CreateTag(tag string) error {
	if err := t.ensureReady(); err != nil {
		return err
	}
	return t.storage.CreateLabel(tag)
}

func (t *Tagger) DeleteTag(tag string) error {
	if err := t.ensureReady(); err != nil {
		return err
	}
	return t.storage.DeleteLabel(tag)
}

func (t *Tagger) GetTags() ([]string, error) {
	if err := t.ensureReady(); err != nil {
		return nil, err
	}
	return t.storage.GetLabels()
}

func (t *Tagger) GetMembers(tag string) ([]string, error) {
	if err := t.ensureReady(); err != nil {
		return nil, err
	}
	return t.storage.GetLabelMembers(tag)
}

func (t *Tagger) AddMembers(tag string, members []string) error {
	if err := t.ensureReady(); err != nil {
		return err
	}
	return t.storage.AddMembersToLabel(tag, members)
}

func (t *Tagger) RemoveMember(tag string, address string) error {
	if err := t.ensureReady(); err != nil {
		return err
	}
	return t.storage.RemoveMemberFromLabel(tag, address)
}

func (t *Tagger) UpsertMember(address string, labels []string) error {
	if err := t.ensureReady(); err != nil {
		return err
	}
	return t.storage.ReplaceMemberLabels(address, labels)
}

func (t *Tagger) GetMemberLabels(address string) ([]string, error) {
	if err := t.ensureReady(); err != nil {
		return nil, err
	}
	return t.storage.GetMemberLabels(address)
}

func (t *Tagger) DeleteMember(address string) error {
	if err := t.ensureReady(); err != nil {
		return err
	}
	return t.storage.DeleteMember(address)
}

func (t *Tagger) ensureReady() error {
	if t.storage == nil {
		return errors.New("tagger storage is not initialized")
	}
	return nil
}
