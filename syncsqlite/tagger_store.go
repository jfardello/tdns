package syncsqlite

import (
	"database/sql"
	"errors"
	"fmt"
	"slices"
	"strings"

	"github.com/jfardello/tdns/internal/db"
	"github.com/jfardello/tdns/storage"
)

type SQLiteStorage struct {
	executor *SyncExecutor
	path     string
}

func resolveDBPath(path string) (string, error) {
	return db.ResolvePath(path)
}

func (s *SQLiteStorage) Open(path string) error {
	dbPath, err := resolveDBPath(path)
	if err != nil {
		return err
	}

	s.path = dbPath
	s.executor = NewSyncExecutor(ConnString(dbPath), MaxReadonlyConnections)
	return nil
}

func (s *SQLiteStorage) Close() error {
	if s.executor == nil {
		return nil
	}
	s.executor.Close()
	if s.executor.roConnPool != nil {
		for i := 0; i < cap(s.executor.roConnPool); i++ {
			select {
			case conn := <-s.executor.roConnPool:
				_ = conn.Close()
			default:
				return nil
			}
		}
	}
	return nil
}

func (s *SQLiteStorage) queryStrings(query string, args ...any) ([]string, error) {
	db := s.executor.GetConn()
	defer s.executor.FreeConn(db)

	rows, err := db.Query(query, args...)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return []string{}, nil
		}
		return nil, err
	}
	defer rows.Close()

	values := make([]string, 0)
	for rows.Next() {
		var value string
		if err := rows.Scan(&value); err != nil {
			return nil, err
		}
		values = append(values, value)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return values, nil
}

func (s *SQLiteStorage) GetMemberLabels(address string) ([]string, error) {
	return s.queryStrings(
		`SELECT ml.label_name
		FROM member_labels ml
		WHERE ml.member_address = ?
		ORDER BY ml.label_name`,
		address,
	)
}

func (s *SQLiteStorage) GetLabelMembers(label string) ([]string, error) {
	return s.queryStrings(
		`SELECT ml.member_address
		FROM member_labels ml
		WHERE ml.label_name = ?
		ORDER BY ml.member_address`,
		label,
	)
}

func (s *SQLiteStorage) GetLabelMemberDetails(label string) ([]storage.TagMember, error) {
	db := s.executor.GetConn()
	defer s.executor.FreeConn(db)

	rows, err := db.Query(
		`SELECT ml.member_address, COALESCE(h.host, '')
		FROM member_labels ml
		LEFT JOIN hosts h ON h.ipAddr = ml.member_address
		WHERE ml.label_name = ?
		ORDER BY CASE WHEN h.host IS NULL OR h.host = '' THEN 1 ELSE 0 END, h.host, ml.member_address`,
		label,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return []storage.TagMember{}, nil
		}
		return nil, err
	}
	defer rows.Close()

	members := make([]storage.TagMember, 0)
	for rows.Next() {
		var member storage.TagMember
		if err := rows.Scan(&member.Address, &member.Host); err != nil {
			return nil, err
		}
		member.HasHostAlias = strings.TrimSpace(member.Host) != ""
		members = append(members, member)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return members, nil
}

func (s *SQLiteStorage) GetAllMemberLabels() ([]storage.MemberLabels, error) {
	db := s.executor.GetConn()
	defer s.executor.FreeConn(db)

	rows, err := db.Query(
		`SELECT member_address, label_name
		FROM member_labels
		ORDER BY member_address, label_name`,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return []storage.MemberLabels{}, nil
		}
		return nil, err
	}
	defer rows.Close()

	members := make([]storage.MemberLabels, 0)
	indexByAddress := map[string]int{}
	for rows.Next() {
		var address string
		var label string
		if err := rows.Scan(&address, &label); err != nil {
			return nil, err
		}
		if idx, ok := indexByAddress[address]; ok {
			members[idx].Labels = append(members[idx].Labels, label)
			continue
		}
		indexByAddress[address] = len(members)
		members = append(members, storage.MemberLabels{
			Address: address,
			Labels:  []string{label},
		})
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return members, nil
}

func (s *SQLiteStorage) GetLabels() ([]string, error) {
	return s.queryStrings(`SELECT name FROM labels ORDER BY name`)
}

func (s *SQLiteStorage) CreateLabel(label string) error {
	label = strings.TrimSpace(label)
	if label == "" {
		return errors.New("label is empty")
	}
	_, err := s.executor.SyncExec(
		`INSERT INTO labels(name) VALUES (?) ON CONFLICT(name) DO NOTHING`,
		[]any{label},
	)
	return err
}

func (s *SQLiteStorage) AddMembersToLabel(label string, members []string) error {
	label = strings.TrimSpace(label)
	if label == "" {
		return errors.New("label is empty")
	}
	if len(members) == 0 {
		return nil
	}

	stmts := []*ExecStmt{{
		Query: `INSERT INTO labels(name) VALUES (?) ON CONFLICT(name) DO NOTHING`,
		Args:  []any{label},
	}}

	seen := make(map[string]struct{}, len(members))
	for _, member := range members {
		member = strings.TrimSpace(member)
		if member == "" {
			continue
		}
		if _, ok := seen[member]; ok {
			continue
		}
		seen[member] = struct{}{}
		stmts = append(stmts,
			&ExecStmt{
				Query: `INSERT INTO members(address) VALUES (?) ON CONFLICT(address) DO NOTHING`,
				Args:  []any{member},
			},
			&ExecStmt{
				Query: `INSERT INTO member_labels(member_address, label_name) VALUES (?, ?) ON CONFLICT(member_address, label_name) DO NOTHING`,
				Args:  []any{member, label},
			},
		)
	}

	return s.executor.SyncExecBulk(stmts)
}

func normalizeLabels(labels []string) []string {
	clean := make([]string, 0, len(labels))
	for _, label := range labels {
		label = strings.TrimSpace(label)
		if label == "" || slices.Contains(clean, label) {
			continue
		}
		clean = append(clean, label)
	}
	slices.Sort(clean)
	return clean
}

func (s *SQLiteStorage) ReplaceMemberLabels(address string, labels []string) error {
	address = strings.TrimSpace(address)
	if address == "" {
		return errors.New("address is empty")
	}

	labels = normalizeLabels(labels)
	stmts := []*ExecStmt{
		{
			Query: `DELETE FROM members WHERE address = ?`,
			Args:  []any{address},
		},
	}
	if len(labels) == 0 {
		return s.executor.SyncExecBulk(stmts)
	}

	stmts = append(stmts, &ExecStmt{
		Query: `INSERT INTO members(address) VALUES (?)`,
		Args:  []any{address},
	})
	for _, label := range labels {
		stmts = append(stmts,
			&ExecStmt{
				Query: `INSERT INTO labels(name) VALUES (?) ON CONFLICT(name) DO NOTHING`,
				Args:  []any{label},
			},
			&ExecStmt{
				Query: `INSERT INTO member_labels(member_address, label_name) VALUES (?, ?)`,
				Args:  []any{address, label},
			},
		)
	}
	return s.executor.SyncExecBulk(stmts)
}

func (s *SQLiteStorage) RemoveMemberFromLabel(label string, address string) error {
	label = strings.TrimSpace(label)
	address = strings.TrimSpace(address)
	if label == "" || address == "" {
		return errors.New("label or address is empty")
	}

	stmts := []*ExecStmt{
		{
			Query: `DELETE FROM member_labels WHERE member_address = ? AND label_name = ?`,
			Args:  []any{address, label},
		},
		{
			Query: `DELETE FROM members
				WHERE address = ?
				AND NOT EXISTS (
					SELECT 1 FROM member_labels WHERE member_address = ?
				)`,
			Args: []any{address, address},
		},
	}
	return s.executor.SyncExecBulk(stmts)
}

func (s *SQLiteStorage) SearchKnownHosts(query string, limit int) ([]storage.KnownHost, error) {
	if limit <= 0 {
		limit = 20
	}

	query = strings.TrimSpace(query)
	like := "%"
	prefixLike := "%"
	if query != "" {
		escaped := strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`).Replace(query)
		like = "%" + escaped + "%"
		prefixLike = escaped + "%"
	}

	db := s.executor.GetConn()
	defer s.executor.FreeConn(db)

	rows, err := db.Query(
		`SELECT ipAddr, host
		FROM hosts
		WHERE host IS NOT NULL
		  AND TRIM(host) <> ''
		  AND (? = '' OR host LIKE ? ESCAPE '\' OR ipAddr LIKE ? ESCAPE '\')
		ORDER BY
		  CASE
		    WHEN host = ? THEN 0
		    WHEN ipAddr = ? THEN 1
		    WHEN host LIKE ? ESCAPE '\' THEN 2
		    WHEN ipAddr LIKE ? ESCAPE '\' THEN 3
		    ELSE 4
		  END,
		  host,
		  ipAddr
		LIMIT ?`,
		query,
		like,
		like,
		query,
		query,
		prefixLike,
		prefixLike,
		limit,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return []storage.KnownHost{}, nil
		}
		return nil, err
	}
	defer rows.Close()

	hosts := make([]storage.KnownHost, 0)
	for rows.Next() {
		var host storage.KnownHost
		if err := rows.Scan(&host.Address, &host.Host); err != nil {
			return nil, err
		}
		hosts = append(hosts, host)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return hosts, nil
}

func (s *SQLiteStorage) DeleteMember(address string) error {
	address = strings.TrimSpace(address)
	if address == "" {
		return errors.New("address is empty")
	}
	_, err := s.executor.SyncExec(`DELETE FROM members WHERE address = ?`, []any{address})
	return err
}

func (s *SQLiteStorage) DeleteLabel(label string) error {
	label = strings.TrimSpace(label)
	if label == "" {
		return errors.New("label is empty")
	}
	_, err := s.executor.SyncExec(`DELETE FROM labels WHERE name = ?`, []any{label})
	return err
}

func NewSQLiteStorage(opts ...storage.Options) (*SQLiteStorage, error) {
	store := &SQLiteStorage{}
	for _, opt := range opts {
		if err := opt(store); err != nil {
			return nil, fmt.Errorf("open sqlite storage: %w", err)
		}
	}
	return store, nil
}
