package middleware

import (
	"context"
	"net"
	"slices"
	"testing"

	"github.com/jfardello/tdns/config"
	"github.com/jfardello/tdns/storage"
)

type fakeTaggerStorage struct {
	memberLabels []storage.MemberLabels
}

func (f *fakeTaggerStorage) Open(string) error                        { return nil }
func (f *fakeTaggerStorage) Close() error                             { return nil }
func (f *fakeTaggerStorage) GetMemberLabels(string) ([]string, error) { return nil, nil }
func (f *fakeTaggerStorage) GetLabelMembers(string) ([]string, error) { return nil, nil }
func (f *fakeTaggerStorage) GetLabelMemberDetails(string) ([]storage.TagMember, error) {
	return nil, nil
}
func (f *fakeTaggerStorage) GetAllMemberLabels() ([]storage.MemberLabels, error) {
	return append([]storage.MemberLabels(nil), f.memberLabels...), nil
}
func (f *fakeTaggerStorage) GetLabels() ([]string, error)               { return nil, nil }
func (f *fakeTaggerStorage) CreateLabel(string) error                   { return nil }
func (f *fakeTaggerStorage) AddMembersToLabel(string, []string) error   { return nil }
func (f *fakeTaggerStorage) ReplaceMemberLabels(string, []string) error { return nil }
func (f *fakeTaggerStorage) RemoveMemberFromLabel(string, string) error { return nil }
func (f *fakeTaggerStorage) SearchKnownHosts(string, int) ([]storage.KnownHost, error) {
	return nil, nil
}
func (f *fakeTaggerStorage) DeleteMember(string) error { return nil }
func (f *fakeTaggerStorage) DeleteLabel(string) error  { return nil }

func TestTaggerRunAddsLabelsForExactAndCIDRMembers(t *testing.T) {
	tagger := &Tagger{
		storage: &fakeTaggerStorage{
			memberLabels: []storage.MemberLabels{
				{Address: "192.168.1.50", Labels: []string{"desktop"}},
				{Address: "192.168.1.0/24", Labels: []string{"lan", "family"}},
			},
		},
	}
	if err := tagger.refreshMatchers(); err != nil {
		t.Fatalf("refreshMatchers error: %v", err)
	}

	msg := &Message{}
	msg.SetCtx(context.WithValue(context.Background(), config.CtxKey, config.CtxValue{
		RemoteAddr: &net.UDPAddr{IP: net.ParseIP("192.168.1.50")},
		Values:     map[string]string{},
	}))

	if _, err := tagger.Run(msg); err != nil {
		t.Fatalf("Run error: %v", err)
	}

	got := msg.Labels()
	want := []string{"desktop", "family", "lan"}
	if !slices.Equal(got, want) {
		t.Fatalf("labels got %v, want %v", got, want)
	}
}
