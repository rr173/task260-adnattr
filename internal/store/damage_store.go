package store

import (
	"fmt"

	"task260-adnattr/internal/model"
)

// UpsertDamageProfile 写入（或更新）某文库的末端脱氨损伤轮廓。
func (s *Store) UpsertDamageProfile(libID int64, deam5p, deam3p, meanLen float64, n int) (*model.DamageProfile, error) {
	_, err := s.GetLibrary(libID)
	if err != nil {
		return nil, err
	}
	if _, err := s.db.Exec(
		`INSERT INTO damage_profiles(library_id, deam_5p, deam_3p, mean_len, n_frags, computed_at)
		 VALUES(?,?,?,?,?,?)
		 ON CONFLICT(library_id) DO UPDATE SET
		   deam_5p=excluded.deam_5p, deam_3p=excluded.deam_3p,
		   mean_len=excluded.mean_len, n_frags=excluded.n_frags, computed_at=excluded.computed_at`,
		libID, deam5p, deam3p, meanLen, n, now()); err != nil {
		return nil, fmt.Errorf("store: upsert damage: %w", err)
	}
	return s.GetDamageProfile(libID)
}

// GetDamageProfile 读取某文库的损伤轮廓。
func (s *Store) GetDamageProfile(libID int64) (*model.DamageProfile, error) {
	row := s.db.QueryRow(
		`SELECT id, library_id, deam_5p, deam_3p, mean_len, n_frags, computed_at
		 FROM damage_profiles WHERE library_id = ?`, libID)
	p := &model.DamageProfile{}
	if err := row.Scan(&p.ID, &p.LibraryID, &p.Deam5p, &p.Deam3p, &p.MeanLen, &p.NFrags, &p.ComputedAt); err != nil {
		return nil, fmt.Errorf("store: get damage: %w", err)
	}
	return p, nil
}
