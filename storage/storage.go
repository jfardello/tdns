package storage

var (
	allTagsPrefix = "tdns/alltags"
	tagPrefix     = "tdns/tag:"
	memberPrefix  = " tdns/member:"
)

/**

Simulate having different buckets by prefixing keys:

  +--------------------------------------+
  | tdns/alltags  = [red, green]         |
  +--------------------------------------+
  |	tdns/tag:red:127.0.0.1  = 1          |
  |	tdns/tag:green:127.0.0.1 = 1         |
  +--------------------------------------+
  |	tdns/member:127.0.0.1 = [red, green] |
  |	tdns/member:127.0.0.2 = [green]      |
  +--------------------------------------+

The only expensive is the deleteTag op:
	+ delete a tag:
		- iterate on preffix "tdns/tag:green" and get all members
			* for each member
				remove green
			* delete the member "tdns/tag:green:<member>"
	+ delete green from tdns/alltags

*/

type Options func(Storage) error

func WithDbPath(path string) Options {
	return func(s Storage) error {
		return s.Open(path)
	}
}

type Storage interface {
	Open(string) error
	Close() error
	GetMember(address string) ([]string, error)
	GetLabels() ([]string, error)
	SetLabel(label string) error
	SetMemberLabels(key string, labels []string) error
	DeleteMember(address string) error
	DeleteLabel(address string) error
}
