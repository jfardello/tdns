package middleware

import (
	"errors"
	"net"
	"slices"
	"strings"
	"sync"

	"github.com/jfardello/tdns/config"
	"github.com/jfardello/tdns/log"
	"github.com/jfardello/tdns/storage"
	"github.com/jfardello/tdns/syncsqlite"
)

type Tagger struct {
	dbFile     string
	enabled    bool
	storage    storage.Storage
	matcherMu  sync.RWMutex
	exactMatch map[string][]string
	cidrMatch  []cidrLabels
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

	tags := t.lookupLabels(address)
	if len(tags) == 0 {
		return message, nil
	}
	if err := message.AddLabels(tags...); err != nil {
		logger.Errorf("can't add tags to context for address: %s. Err:%s", address, err)
	}
	return message, nil
}

func (t *Tagger) Info() (string, Stage) {
	return "tagger", PreRouting
}

func (t *Tagger) Config(cf config.Config) error {
	if cf.Tagger.Enabled {
		if cf.Database.File == "" {
			return errors.New("database file is empty")
		}
		t.dbFile = cf.Database.File
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
	return t.refreshMatchers()
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
	if err := t.storage.DeleteLabel(tag); err != nil {
		return err
	}
	return t.refreshMatchers()
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

func (t *Tagger) GetMemberDetails(tag string) ([]storage.TagMember, error) {
	if err := t.ensureReady(); err != nil {
		return nil, err
	}
	return t.storage.GetLabelMemberDetails(tag)
}

func (t *Tagger) AddMembers(tag string, members []string) error {
	if err := t.ensureReady(); err != nil {
		return err
	}
	if err := t.storage.AddMembersToLabel(tag, members); err != nil {
		return err
	}
	return t.refreshMatchers()
}

func (t *Tagger) RemoveMember(tag string, address string) error {
	if err := t.ensureReady(); err != nil {
		return err
	}
	if err := t.storage.RemoveMemberFromLabel(tag, address); err != nil {
		return err
	}
	return t.refreshMatchers()
}

func (t *Tagger) SearchKnownHosts(query string, limit int) ([]storage.KnownHost, error) {
	if err := t.ensureReady(); err != nil {
		return nil, err
	}
	return t.storage.SearchKnownHosts(query, limit)
}

func (t *Tagger) UpsertMember(address string, labels []string) error {
	if err := t.ensureReady(); err != nil {
		return err
	}
	if err := t.storage.ReplaceMemberLabels(address, labels); err != nil {
		return err
	}
	return t.refreshMatchers()
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
	if err := t.storage.DeleteMember(address); err != nil {
		return err
	}
	return t.refreshMatchers()
}

func (t *Tagger) ensureReady() error {
	if t.storage == nil {
		return errors.New("tagger storage is not initialized")
	}
	return nil
}

func (t *Tagger) refreshMatchers() error {
	members, err := t.storage.GetAllMemberLabels()
	if err != nil {
		return err
	}

	exactMatch := make(map[string][]string, len(members))
	cidrMatch := make([]cidrLabels, 0)

	for _, member := range members {
		address := strings.TrimSpace(member.Address)
		labels := normalizeLabels(member.Labels)
		if address == "" || len(labels) == 0 {
			continue
		}
		if _, network, err := net.ParseCIDR(address); err == nil {
			cidrMatch = append(cidrMatch, cidrLabels{
				network: network,
				labels:  labels,
			})
			continue
		}
		exactMatch[address] = labels
	}

	t.matcherMu.Lock()
	t.exactMatch = exactMatch
	t.cidrMatch = cidrMatch
	t.matcherMu.Unlock()
	return nil
}

func (t *Tagger) lookupLabels(address string) []string {
	ip := net.ParseIP(address)

	t.matcherMu.RLock()
	defer t.matcherMu.RUnlock()

	labels := append([]string(nil), t.exactMatch[address]...)
	if ip == nil {
		return labels
	}

	for _, candidate := range t.cidrMatch {
		if candidate.network != nil && candidate.network.Contains(ip) {
			for _, label := range candidate.labels {
				if slices.Contains(labels, label) {
					continue
				}
				labels = append(labels, label)
			}
		}
	}

	return normalizeLabels(labels)
}
