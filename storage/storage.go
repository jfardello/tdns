package storage

type TagMember struct {
	Address      string `json:"address"`
	Host         string `json:"host,omitempty"`
	HasHostAlias bool   `json:"has_host_alias"`
}

type KnownHost struct {
	Address string `json:"address"`
	Host    string `json:"host"`
}

type MemberLabels struct {
	Address string
	Labels  []string
}

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
	GetLabelMemberDetails(label string) ([]TagMember, error)
	GetAllMemberLabels() ([]MemberLabels, error)
	GetLabels() ([]string, error)
	CreateLabel(label string) error
	AddMembersToLabel(label string, members []string) error
	ReplaceMemberLabels(address string, labels []string) error
	RemoveMemberFromLabel(label string, address string) error
	SearchKnownHosts(query string, limit int) ([]KnownHost, error)
	DeleteMember(address string) error
	DeleteLabel(label string) error
}
