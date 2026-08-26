package store

import (
	"database/sql"
	"fmt"

	"task260-adnattr/internal/model"
)

// InsertAttribution 写入一条归因候选。
func (s *Store) InsertAttribution(libID int64, kind, status string, score float64, reason string) (*model.AttributionCandidate, error) {
	lb, err := s.GetLibrary(libID)
	if err != nil {
		return nil, err
	}
	if err := model.ValidateLibraryMutable(lb.Status); err != nil {
		return nil, err
	}
	res, err := s.db.Exec(
		`INSERT INTO attribution_candidates(library_id, kind, status, score, reason, created_at)
		 VALUES(?,?,?,?,?,?)`,
		libID, kind, status, score, reason, now())
	if err != nil {
		return nil, fmt.Errorf("store: insert attribution: %w", err)
	}
	id, _ := res.LastInsertId()
	return s.GetAttribution(id)
}

// GetAttribution 按 ID 读取归因候选。
func (s *Store) GetAttribution(id int64) (*model.AttributionCandidate, error) {
	row := s.db.QueryRow(
		`SELECT id, library_id, kind, status, score, reason, created_at
		 FROM attribution_candidates WHERE id = ?`, id)
	a := &model.AttributionCandidate{}
	if err := row.Scan(&a.ID, &a.LibraryID, &a.Kind, &a.Status, &a.Score, &a.Reason, &a.CreatedAt); err != nil {
		if err == sql.ErrNoRows {
			return nil, model.ErrUnknownCluster
		}
		return nil, fmt.Errorf("store: get attribution: %w", err)
	}
	return a, nil
}

// ListAttributionsByLibrary 列出某文库的归因候选。
func (s *Store) ListAttributionsByLibrary(libID int64) ([]*model.AttributionCandidate, error) {
	rows, err := s.db.Query(
		`SELECT id, library_id, kind, status, score, reason, created_at
		 FROM attribution_candidates WHERE library_id = ? ORDER BY id ASC`, libID)
	if err != nil {
		return nil, fmt.Errorf("store: list attributions: %w", err)
	}
	defer rows.Close()
	var out []*model.AttributionCandidate
	for rows.Next() {
		a := &model.AttributionCandidate{}
		if err := rows.Scan(&a.ID, &a.LibraryID, &a.Kind, &a.Status, &a.Score, &a.Reason, &a.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, a)
	}
	return out, rows.Err()
}

// ConfirmAttribution 确认归因候选（校验流转）。
func (s *Store) ConfirmAttribution(id int64, to string) (*model.AttributionCandidate, error) {
	a, err := s.GetAttribution(id)
	if err != nil {
		return nil, err
	}
	if err := model.ValidateAttributionTransition(a.Status, to); err != nil {
		return nil, err
	}
	if _, err := s.db.Exec(`UPDATE attribution_candidates SET status = ? WHERE id = ?`, to, id); err != nil {
		return nil, fmt.Errorf("store: confirm attribution: %w", err)
	}
	return s.GetAttribution(id)
}

// DeleteOpenAttributions 删除某文库未确认的归因候选（刷新重算前清理）。
func (s *Store) DeleteOpenAttributions(libID int64) (int64, error) {
	res, err := s.db.Exec(
		`DELETE FROM attribution_candidates WHERE library_id = ? AND status != ?`, libID, model.AttribConfirmed)
	if err != nil {
		return 0, fmt.Errorf("store: delete open attributions: %w", err)
	}
	n, _ := res.RowsAffected()
	return n, nil
}
