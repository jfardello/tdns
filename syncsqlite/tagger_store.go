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
