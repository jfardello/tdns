package storage

import (
	"encoding/json"
	"errors"
	"fmt"
	"github.com/cockroachdb/pebble"
	"github.com/jfardello/tdns/log"
	"slices"
	"strings"
)

type PebbleStorage struct {
	db *pebble.DB
}

func (p *PebbleStorage) Open(dir string) error {
	var err error
	pebbleLogger := log.GetLogger("pebbleStorage", "db")
	p.db, err = pebble.Open(dir, &pebble.Options{Logger: pebbleLogger})

	if err != nil {
		return fmt.Errorf("error opening pebble db, %w", err)
	}
	return nil
}

func (p *PebbleStorage) Close() error {
	return p.db.Close()
}

func (p *PebbleStorage) GetMember(address string) ([]string, error) {
	logger := log.GetLogger("pebbleStorage", "GetMember")
	key := []byte(fmt.Sprintf("%s:%s", memberPrefix, address))
	m, closer, err := p.db.Get(key)
	if err != nil {
		return nil, fmt.Errorf("error getting member %s, %w", address, err)
	}
	defer func() {
		err := closer.Close()
		logger.Fatal(err)
	}()
	val := make([]string, 0)
	err = json.Unmarshal(m, &val)
	if err != nil {
		return nil, fmt.Errorf("error unmarshalling member %s, %w", address, err)
	}
	return val, nil
}

func (p *PebbleStorage) GetLabels() ([]string, error) {
	logger := log.GetLogger("pebbleStorage", "GetLabels")
	value, closer, err := p.db.Get([]byte(allTagsPrefix))
	if err != nil && !errors.Is(err, pebble.ErrNotFound) {
		return nil, fmt.Errorf("error getting all tags, %w", err)
	}
	if closer != nil {
		defer func() {
			err := closer.Close()
			if err != nil {
				logger.Fatal(err)
			}
		}()
	}
	var ret []string
	if errors.Is(err, pebble.ErrNotFound) {
		return ret, nil
	}
	err = json.Unmarshal(value, &ret)
	if err != nil {
		return nil, fmt.Errorf("error unmarshalling labels, %w", err)
	}
	return ret, nil
}

func (p *PebbleStorage) SetLabel(label string) error {
	logger := log.GetLogger("pebbleStorage", "SetLabel")
	labels, err := p.GetLabels()
	logger.Debugf(">>>> labels: %s, err: %+v", labels, err)
	if err != nil {
		return err
	}
	for l := range labels {
		if labels[l] == label {
			return nil
		}
	}
	labels = append(labels, label)
	newValue, err := json.Marshal(labels)
	if err != nil {
		return fmt.Errorf("error marshalling labels, %w", err)
	}
	err = p.db.Set([]byte(allTagsPrefix), newValue, pebble.NoSync)
	if err != nil {
		return fmt.Errorf("error setting labels, %w", err)
	}
	return nil
}

// DeleteLabel is an expensive method:
//   - removes the label from the global list,
//   - deletes global members (tdns/tag:label:*)
//   - edits all the tags for each individual member: tdns/member:addr = [green, red]
//
// This greatly improves dns performance as everything is precomputed.
func (p *PebbleStorage) DeleteLabel(label string) error {

	logger := log.GetLogger("storage", "PebbleStorage.DeleteLabel")

	//get prefix positions for label members: tdns/tag:green:*
	prefixIterOptions := func(prefix []byte) *pebble.IterOptions {
		return &pebble.IterOptions{
			LowerBound: prefix,
			UpperBound: keyUpperBound(prefix),
		}
	}
	prefix := fmt.Sprintf("%s:%s:", tagPrefix, label)
	iter, _ := p.db.NewIter(prefixIterOptions([]byte(prefix)))
	defer func() {
		err := iter.Close()
		if err != nil {
			logger.Fatal(err)
		}

	}()
	start, end := iter.RangeBounds()

	//Iterate and extract the address, and remove the tag from each member

	for iter.First(); iter.Valid(); iter.Next() {
		fmt.Printf("%s\n", iter.Key())
		addr := strings.Split(string(iter.Key()), ":")[2]
		//this edits tdns/member:addr = [l1, l2] and removes the affected label."
		err := p.memberRemoveLabel(addr, label)
		if err != nil {
			return fmt.Errorf("error removing label for %s member: %w", addr, err)
		}
	}

	//delete the range of label members: tdns/tag:green:*
	err := p.db.DeleteRange(start, end, pebble.NoSync)
	if err != nil {
		return fmt.Errorf("error deleting labels, %w", err)
	}

	//delete label from label list
	labels, err := p.GetLabels()
	if err != nil {
		return err
	}
	logger.Infof("Deleting label: %s from %v", label, labels)

	labels = slices.DeleteFunc(labels, func(s string) bool {
		return s == label
	})

	labelsValue, err := json.Marshal(labels)
	logger.Debugf(">>>> labelsValue: %s", string(labelsValue))
	if err != nil {
		return fmt.Errorf("error marshalling labels, %w", err)
	}
	err = p.db.Set([]byte(allTagsPrefix), labelsValue, pebble.Sync)
	if err != nil {
		return fmt.Errorf("error deleting labels, %w", err)
	}
	return nil
}

func (p *PebbleStorage) SetMemberLabels(key string, labels []string) error {
	k := []byte(fmt.Sprintf("%s:%s", memberPrefix, key))
	l, err := json.Marshal(labels)
	if err != nil {
		return fmt.Errorf("error marshalling labels, %w", err)
	}
	err = p.db.Set(k, l, pebble.Sync)
	return nil
}

func (p *PebbleStorage) DeleteMember(address string) error {
	k := []byte(fmt.Sprintf("%s:%s", memberPrefix, address))
	err := p.db.Delete(k, pebble.Sync)
	if err != nil {
		return fmt.Errorf("error deleting member %s, %w", address, err)
	}
	return nil
}

func (p *PebbleStorage) memberRemoveLabel(address string, label string) error {
	logger := log.GetLogger("PebbleStorage", "memberRemoveLabel")
	key := []byte(fmt.Sprintf("%s:%s", memberPrefix, address))
	_v, closer, err := p.db.Get(key)
	defer func() {
		err := closer.Close()
		if err != nil {
			logger.Fatal(err)
		}
	}()
	if err != nil {
		return fmt.Errorf("error getting member labels, %w", err)
	}
	value := make([]string, 0)
	err = json.Unmarshal(_v, &value)
	if err != nil {
		return fmt.Errorf("error unmarshalling labels for member %s: %w", address, err)
	}
	slices.DeleteFunc(value, func(s string) bool {
		return s == label
	})
	encodedValue, err := json.Marshal(value)
	if err != nil {
		return fmt.Errorf("error marshalling labels for member %s: %w", address, err)
	}

	return p.db.Set(key, encodedValue, pebble.Sync)

}

func keyUpperBound(b []byte) []byte {
	end := make([]byte, len(b))
	copy(end, b)
	for i := len(end) - 1; i >= 0; i-- {
		end[i] = end[i] + 1
		if end[i] != 0 {
			return end[:i+1]
		}
	}
	return nil
}

func NewPebbleStorage(opts ...Options) *PebbleStorage {
	logger := log.GetLogger("storage", "badger")
	ps := &PebbleStorage{}
	for _, opt := range opts {
		err := opt(ps)
		if err != nil {
			logger.Fatal(err)
		}
	}
	return ps
}
