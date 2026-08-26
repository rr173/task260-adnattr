package store

import (
	"database/sql"
	"fmt"

	"task260-adnattr/internal/model"
)

// CreateSnapshot 创建可信度快照（草稿）。payload 为 JSON 摘要。
func (s *Store) CreateSnapshot(libID int64, payload string) (*model.ConfidenceSnapshot, error) {
	lb, err := s.GetLibrary(libID)
	if err != nil {
		return nil, err
	}
	if lb.Status == model.LibSealed {
		return nil, model.ErrSealed
	}
	res, err := s.db.Exec(
		`INSERT INTO confidence_snapshots(library_id, status, payload, created_at) VALUES(?,?,?,?)`,
		libID, model.SnapDraft, payload, now())
	if err != nil {
		return nil, fmt.Errorf("store: create snapshot: %w", err)
	}
	id, _ := res.LastInsertId()
	return s.GetSnapshot(id)
}

// GetSnapshot 按 ID 读取快照。
func (s *Store) GetSnapshot(id int64) (*model.ConfidenceSnapshot, error) {
	row := s.db.QueryRow(
		`SELECT id, library_id, status, control_batch_id, payload, created_at, published_at
		 FROM confidence_snapshots WHERE id = ?`, id)
	snap := &model.ConfidenceSnapshot{}
	var ctrl sql.NullInt64
	var pub sql.NullInt64
	if err := row.Scan(&snap.ID, &snap.LibraryID, &snap.Status, &ctrl, &snap.Payload, &snap.CreatedAt, &pub); err != nil {
		if err == sql.ErrNoRows {
			return nil, model.ErrUnknownCluster
		}
		return nil, fmt.Errorf("store: get snapshot: %w", err)
	}
	if ctrl.Valid {
		snap.ControlBatchID = ctrl.Int64
	}
	if pub.Valid {
		snap.PublishedAt = pub.Int64
	}
	return snap, nil
}

// ListSnapshotsByLibrary 列出某文库的快照（含被替代）。
func (s *Store) ListSnapshotsByLibrary(libID int64) ([]*model.ConfidenceSnapshot, error) {
	rows, err := s.db.Query(
		`SELECT id, library_id, status, control_batch_id, payload, created_at, published_at
		 FROM confidence_snapshots WHERE library_id = ? ORDER BY id ASC`, libID)
	if err != nil {
		return nil, fmt.Errorf("store: list snapshots: %w", err)
	}
	defer rows.Close()
	var out []*model.ConfidenceSnapshot
	for rows.Next() {
		snap := &model.ConfidenceSnapshot{}
		var ctrl sql.NullInt64
		var pub sql.NullInt64
		if err := rows.Scan(&snap.ID, &snap.LibraryID, &snap.Status, &ctrl, &snap.Payload, &snap.CreatedAt, &pub); err != nil {
			return nil, err
		}
		if ctrl.Valid {
			snap.ControlBatchID = ctrl.Int64
		}
		if pub.Valid {
			snap.PublishedAt = pub.Int64
		}
		out = append(out, snap)
	}
	return out, rows.Err()
}

// PublishSnapshot 发布快照：draft→published，并冻结对照批次（不可变）。
func (s *Store) PublishSnapshot(id int64, controlBatchID int64) (*model.ConfidenceSnapshot, error) {
	snap, err := s.GetSnapshot(id)
	if err != nil {
		return nil, err
	}
	if err := s.ValidateSnapshotControl(controlBatchID); err != nil {
		return nil, err
	}
	if err := model.ValidateSnapshotTransition(snap.Status, model.SnapPublished); err != nil {
		return nil, err
	}
	if _, err := s.db.Exec(
		`UPDATE confidence_snapshots SET status = ?, control_batch_id = ?, published_at = ? WHERE id = ?`,
		model.SnapPublished, controlBatchID, now(), id); err != nil {
		return nil, fmt.Errorf("store: publish snapshot: %w", err)
	}
	return s.GetSnapshot(id)
}

// SupersedeSnapshot 将已发布快照标记为被替代（保留历史）。
func (s *Store) SupersedeSnapshot(id int64) (*model.ConfidenceSnapshot, error) {
	snap, err := s.GetSnapshot(id)
	if err != nil {
		return nil, err
	}
	if err := model.ValidateSnapshotTransition(snap.Status, model.SnapSuperseded); err != nil {
		return nil, err
	}
	if _, err := s.db.Exec(`UPDATE confidence_snapshots SET status = ? WHERE id = ?`, model.SnapSuperseded, id); err != nil {
		return nil, fmt.Errorf("store: supersede snapshot: %w", err)
	}
	return s.GetSnapshot(id)
}
