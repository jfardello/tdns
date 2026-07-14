package overrides

import (
	"context"
	"database/sql"
	"time"

	"github.com/jfardello/tdns/internal/db"
	"github.com/jfardello/tdns/internal/sqliteutil"
)

type Store struct {
	conn *sql.DB
}

func Open(ctx context.Context, dbPath string) (*Store, error) {
	resolvedPath, err := db.ResolvePath(dbPath)
	if err != nil {
		return nil, err
	}
	conn, err := sql.Open(sqliteutil.DriverName(), sqliteutil.ReadWriteDSN(resolvedPath))
	if err != nil {
		return nil, err
	}
	if err := sqliteutil.ConfigureConnection(ctx, conn, true); err != nil {
		_ = conn.Close()
		return nil, err
	}
	return &Store{conn: conn}, nil
}

func (s *Store) Close() error {
	if s == nil || s.conn == nil {
		return nil
	}
	return s.conn.Close()
}

func (s *Store) List(ctx context.Context) ([]Row, error) {
	rows, err := s.conn.QueryContext(ctx, `
SELECT id, kind, op, target, value, created_at, updated_at
FROM config_overrides
ORDER BY kind, target, id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	overrides := []Row{}
	for rows.Next() {
		var row Row
		var createdAt int64
		var updatedAt int64
		if err := rows.Scan(&row.ID, &row.Kind, &row.Op, &row.Target, &row.Value, &createdAt, &updatedAt); err != nil {
			return nil, err
		}
		row.CreatedAt = time.Unix(createdAt, 0).UTC()
		row.UpdatedAt = time.Unix(updatedAt, 0).UTC()
		overrides = append(overrides, row)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return overrides, nil
}

func (s *Store) ListByKind(ctx context.Context, kind Kind) ([]Row, error) {
	rows, err := s.conn.QueryContext(ctx, `
SELECT id, kind, op, target, value, created_at, updated_at
FROM config_overrides
WHERE kind = ?
ORDER BY target, id`, int(kind))
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	overrides := []Row{}
	for rows.Next() {
		var row Row
		var createdAt int64
		var updatedAt int64
		if err := rows.Scan(&row.ID, &row.Kind, &row.Op, &row.Target, &row.Value, &createdAt, &updatedAt); err != nil {
			return nil, err
		}
		row.CreatedAt = time.Unix(createdAt, 0).UTC()
		row.UpdatedAt = time.Unix(updatedAt, 0).UTC()
		overrides = append(overrides, row)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return overrides, nil
}

func (s *Store) Upsert(ctx context.Context, kind Kind, op Op, target string, value string) error {
	now := time.Now().UTC().Unix()
	_, err := s.conn.ExecContext(ctx, `
INSERT INTO config_overrides (kind, op, target, value, created_at, updated_at)
VALUES (?, ?, ?, ?, ?, ?)
ON CONFLICT(kind, target) DO UPDATE SET
  op = excluded.op,
  value = excluded.value,
  updated_at = excluded.updated_at`,
		int(kind), int(op), target, value, now, now)
	return err
}

func (s *Store) Delete(ctx context.Context, kind Kind, target string) error {
	_, err := s.conn.ExecContext(ctx, `DELETE FROM config_overrides WHERE kind = ? AND target = ?`, int(kind), target)
	return err
}

func (s *Store) DeleteByKind(ctx context.Context, kind Kind) error {
	_, err := s.conn.ExecContext(ctx, `DELETE FROM config_overrides WHERE kind = ?`, int(kind))
	return err
}

func (s *Store) GetValue(ctx context.Context, kind Kind, target string) (*Row, error) {
	row := Row{}
	var createdAt int64
	var updatedAt int64
	err := s.conn.QueryRowContext(ctx, `
SELECT id, kind, op, target, value, created_at, updated_at
FROM config_overrides
WHERE kind = ? AND target = ?`,
		int(kind), target,
	).Scan(&row.ID, &row.Kind, &row.Op, &row.Target, &row.Value, &createdAt, &updatedAt)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	row.CreatedAt = time.Unix(createdAt, 0).UTC()
	row.UpdatedAt = time.Unix(updatedAt, 0).UTC()
	return &row, nil
}
