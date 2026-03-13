package storage

type Options func(Storage) error

func WithDbPath(path string) Options {
	return func(s Storage) error {
		return s.Open(path)
	}
}

type Storage interface {
	Open(string) error
	Close() error
	GetMemberLabels(address string) ([]string, error)
	GetLabelMembers(label string) ([]string, error)
	GetLabels() ([]string, error)
	CreateLabel(label string) error
	AddMembersToLabel(label string, members []string) error
	ReplaceMemberLabels(address string, labels []string) error
	RemoveMemberFromLabel(label string, address string) error
	DeleteMember(address string) error
	DeleteLabel(label string) error
}
