package store

import (
	"database/sql"
	"fmt"

	"task260-adnattr/internal/model"
)

// CreateLibrary 创建文库批次，初始状态 receiving。
func (s *Store) CreateLibrary(name, note string) (*model.LibraryBatch, error) {
	if name == "" {
		return nil, model.ErrEmptyName
	}
	res, err := s.db.Exec(
		`INSERT INTO library_batches(name, status, created_at, note) VALUES(?,?,?,?)`,
		name, model.LibReceiving, now(), note)
	if err != nil {
		return nil, fmt.Errorf("store: create library: %w", err)
	}
	id, _ := res.LastInsertId()
	return s.GetLibrary(id)
}

// GetLibrary 按 ID 读取文库批次。
func (s *Store) GetLibrary(id int64) (*model.LibraryBatch, error) {
	row := s.db.QueryRow(
		`SELECT id, name, status, created_at, sealed_at, note FROM library_batches WHERE id = ?`, id)
	lb := &model.LibraryBatch{}
	var sealed sql.NullInt64
	if err := row.Scan(&lb.ID, &lb.Name, &lb.Status, &lb.CreatedAt, &sealed, &lb.Note); err != nil {
		if err == sql.ErrNoRows {
			return nil, model.ErrUnknownLibrary
		}
		return nil, fmt.Errorf("store: get library: %w", err)
	}
	if sealed.Valid {
		lb.SealedAt = sealed.Int64
	}
	return lb, nil
}

// ListLibraries 列出全部文库批次（按 ID 升序）。
func (s *Store) ListLibraries() ([]*model.LibraryBatch, error) {
	rows, err := s.db.Query(
		`SELECT id, name, status, created_at, sealed_at, note FROM library_batches ORDER BY id ASC`)
	if err != nil {
		return nil, fmt.Errorf("store: list libraries: %w", err)
	}
	defer rows.Close()
	var out []*model.LibraryBatch
	for rows.Next() {
		lb := &model.LibraryBatch{}
		var sealed sql.NullInt64
		if err := rows.Scan(&lb.ID, &lb.Name, &lb.Status, &lb.CreatedAt, &sealed, &lb.Note); err != nil {
			return nil, err
		}
		if sealed.Valid {
			lb.SealedAt = sealed.Int64
		}
		out = append(out, lb)
	}
	return out, rows.Err()
}

// AdvanceLibrary 推进文库批次状态机一步。校验流转合法性；sealed 后禁止任何写操作。
func (s *Store) AdvanceLibrary(id int64, to string) (*model.LibraryBatch, error) {
	lb, err := s.GetLibrary(id)
	if err != nil {
		return nil, err
	}
	if lb.Status == model.LibSealed {
		return nil, model.ErrSealed
	}
	if err := model.ValidateLibraryTransition(lb.Status, to); err != nil {
		return nil, err
	}
	if _, err := s.db.Exec(`UPDATE library_batches SET status = ? WHERE id = ?`, to, id); err != nil {
		return nil, fmt.Errorf("store: advance library: %w", err)
	}
	return s.GetLibrary(id)
}

// SealLibrary 封存文库批次：终态，禁止后续写操作。返回封存后的实体。
func (s *Store) SealLibrary(id int64) (*model.LibraryBatch, error) {
	lb, err := s.GetLibrary(id)
	if err != nil {
		return nil, err
	}
	if lb.Status == model.LibSealed {
		return lb, nil
	}
	if lb.Status != model.LibPublished {
		return nil, fmt.Errorf("%w: cannot seal from %s", model.ErrInvalidStatus, lb.Status)
	}
	if _, err := s.db.Exec(
		`UPDATE library_batches SET status = ?, sealed_at = ? WHERE id = ?`,
		model.LibSealed, now(), id); err != nil {
		return nil, fmt.Errorf("store: seal library: %w", err)
	}
	return s.GetLibrary(id)
}

// IsLibrarySealed 判断文库是否已被封存。
func (s *Store) IsLibrarySealed(id int64) (bool, error) {
	lb, err := s.GetLibrary(id)
	if err != nil {
		return false, err
	}
	return lb.Status == model.LibSealed, nil
}
