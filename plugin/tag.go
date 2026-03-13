package plugin

import (
	"github.com/jfardello/tdns/config"
	"github.com/jfardello/tdns/log"
	"github.com/jfardello/tdns/storage"
	"os"
	"strings"
)

var BoltMode os.FileMode = 0600

type Tagger struct {
	dbFile  string
	enabled bool
	storage *storage.PebbleStorage
}

func (t *Tagger) Close() error {
	if t.storage == nil {
		return nil
	}
	return t.storage.Close()
}

func (t *Tagger) Run(message *Message) (*Message, error) {
	logger := log.GetLogger("plugin.Tagger", "Run")
	cv, ok := message.ctx.Value(config.CtxKey).(config.CtxValue)
	if ok {
		tags, err := t.storage.GetMember(cv.RemoteAddr.String())
		if err != nil {
			logger.Errorf("Get tags error: %s", err)
			return message, nil
		}
		err = message.AddValue("tags", strings.Join(tags, ","))
		if err != nil {
			logger.Errorf("can't add tags to context for address: %s. Err:%s", cv.RemoteAddr.String(), err)
		}
	}
	return message, nil
}

func (t *Tagger) Info() (string, Ptype) {
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
	if t.enabled {
		t.storage = storage.NewPebbleStorage(storage.WithDbPath(t.dbFile))
	}
	return nil
}

func (t *Tagger) CreateTag(tag string) error {
	return t.storage.SetLabel(tag)
}

func (t *Tagger) DeleteTag(tag string) error {
	return t.storage.DeleteLabel(tag)
}

func (t *Tagger) TagMember(member string, tags []string) error {
	return t.storage.SetMemberLabels(member, tags)
}
func (t *Tagger) GetTags() ([]string, error) {
	return t.storage.GetLabels()
}
