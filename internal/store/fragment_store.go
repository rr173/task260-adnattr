package store

import (
	"database/sql"
	"fmt"

	"task260-adnattr/internal/model"
)

// FragStats 片段聚合统计，供损伤轮廓计算使用。
type FragStats struct {
	Count     int
	MeanLen   float64
	MeanC2T5p float64
	MeanG2A3p float64
}

// FragmentInsert is the persistence-ready form of a fragment summary.
type FragmentInsert struct {
	Fingerprint   string
	FragLen       int
	C2T5p         float64
	G2A3p         float64
	MeanBaseError float64
}

// InsertFragmentBatch writes all summaries in one transaction.
func (s *Store) InsertFragmentBatch(libID int64, items []FragmentInsert) (added, ignored int, err error) {
	lb, err := s.GetLibrary(libID)
	if err != nil {
		return 0, 0, err
	}
	if err := model.ValidateLibraryMutable(lb.Status); err != nil {
		return 0, 0, err
	}
	tx, err := s.db.Begin()
	if err != nil {
		return 0, 0, fmt.Errorf("store: begin fragment batch: %w", err)
	}
	defer func() {
		if err != nil {
			_ = tx.Rollback()
		}
	}()
	for _, item := range items {
		res, execErr := tx.Exec(`INSERT OR IGNORE INTO fragment_summaries
			(library_id, fingerprint, frag_len, c2t_5p, g2a_3p, mean_base_error, created_at)
			VALUES(?,?,?,?,?,?,?)`, libID, item.Fingerprint, item.FragLen, item.C2T5p,
			item.G2A3p, item.MeanBaseError, now())
		if execErr != nil {
			return 0, 0, fmt.Errorf("store: insert fragment batch: %w", execErr)
		}
		n, rowsErr := res.RowsAffected()
		if rowsErr != nil {
			return 0, 0, rowsErr
		}
		if n == 1 {
			added++
		} else {
			ignored++
		}
	}
	if err = tx.Commit(); err != nil {
		return 0, 0, fmt.Errorf("store: commit fragment batch: %w", err)
	}
	return added, ignored, nil
}

// InsertFragmentSummary 幂等导入片段摘要：指纹已存在则忽略并返回已有记录。
func (s *Store) InsertFragmentSummary(libID int64, fp string, fragLen int, c2t5p, g2a3p, meanErr float64) (*model.FragmentSummary, bool, error) {
	lb, err := s.GetLibrary(libID)
	if err != nil {
		return nil, false, err
	}
	if lb.Status == model.LibSealed {
		return nil, false, model.ErrSealed
	}
	if fragLen <= 0 {
		return nil, false, model.ErrNonPositive
	}
	// 先尝试按指纹读取（幂等）。
	if existing, err := s.GetFragmentByFingerprint(fp); err == nil {
		return existing, false, nil
	}
	res, err := s.db.Exec(
		`INSERT INTO fragment_summaries(library_id, fingerprint, frag_len, c2t_5p, g2a_3p, mean_base_error, created_at)
		 VALUES(?,?,?,?,?,?,?)`,
		libID, fp, fragLen, c2t5p, g2a3p, meanErr, now())
	if err != nil {
		if existing, readErr := s.GetFragmentByFingerprint(fp); readErr == nil {
			return existing, false, nil
		}
		return nil, false, fmt.Errorf("store: insert fragment: %w", err)
	}
	id, _ := res.LastInsertId()
	fs, err := s.GetFragment(id)
	return fs, true, err
}

// GetFragment 按 ID 读取片段摘要。
func (s *Store) GetFragment(id int64) (*model.FragmentSummary, error) {
	row := s.db.QueryRow(
		`SELECT id, library_id, fingerprint, frag_len, c2t_5p, g2a_3p, mean_base_error, created_at
		 FROM fragment_summaries WHERE id = ?`, id)
	fs := &model.FragmentSummary{}
	if err := row.Scan(&fs.ID, &fs.LibraryID, &fs.Fingerprint, &fs.FragLen, &fs.C2T5p, &fs.G2A3p, &fs.MeanBaseError, &fs.CreatedAt); err != nil {
		if err == sql.ErrNoRows {
			return nil, model.ErrUnknownCluster
		}
		return nil, fmt.Errorf("store: get fragment: %w", err)
	}
	return fs, nil
}

// GetFragmentByFingerprint 按指纹读取片段摘要。
func (s *Store) GetFragmentByFingerprint(fp string) (*model.FragmentSummary, error) {
	row := s.db.QueryRow(
		`SELECT id, library_id, fingerprint, frag_len, c2t_5p, g2a_3p, mean_base_error, created_at
		 FROM fragment_summaries WHERE fingerprint = ?`, fp)
	fs := &model.FragmentSummary{}
	if err := row.Scan(&fs.ID, &fs.LibraryID, &fs.Fingerprint, &fs.FragLen, &fs.C2T5p, &fs.G2A3p, &fs.MeanBaseError, &fs.CreatedAt); err != nil {
		if err == sql.ErrNoRows {
			return nil, model.ErrUnknownCluster
		}
		return nil, fmt.Errorf("store: get fragment by fp: %w", err)
	}
	return fs, nil
}

// ListFragmentsByLibrary 列出某文库的片段摘要。
func (s *Store) ListFragmentsByLibrary(libID int64) ([]*model.FragmentSummary, error) {
	rows, err := s.db.Query(
		`SELECT id, library_id, fingerprint, frag_len, c2t_5p, g2a_3p, mean_base_error, created_at
		 FROM fragment_summaries WHERE library_id = ? ORDER BY id ASC`, libID)
	if err != nil {
		return nil, fmt.Errorf("store: list fragments: %w", err)
	}
	defer rows.Close()
	var out []*model.FragmentSummary
	for rows.Next() {
		fs := &model.FragmentSummary{}
		if err := rows.Scan(&fs.ID, &fs.LibraryID, &fs.Fingerprint, &fs.FragLen, &fs.C2T5p, &fs.G2A3p, &fs.MeanBaseError, &fs.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, fs)
	}
	return out, rows.Err()
}

// AggregateFragments 聚合某文库的片段统计（用于损伤轮廓）。
func (s *Store) AggregateFragments(libID int64) (*FragStats, error) {
	row := s.db.QueryRow(
		`SELECT COUNT(*), COALESCE(AVG(frag_len),0), COALESCE(AVG(c2t_5p),0), COALESCE(AVG(g2a_3p),0)
		 FROM fragment_summaries WHERE library_id = ?`, libID)
	st := &FragStats{}
	if err := row.Scan(&st.Count, &st.MeanLen, &st.MeanC2T5p, &st.MeanG2A3p); err != nil {
		return nil, fmt.Errorf("store: aggregate fragments: %w", err)
	}
	return st, nil
}

// InsertFragmentCluster 写入片段簇（初始 raw）。
func (s *Store) InsertFragmentCluster(libID int64, fp string, meanLen, meanC2T5p, meanG2A3p float64, size int) (*model.FragmentCluster, error) {
	lb, err := s.GetLibrary(libID)
	if err != nil {
		return nil, err
	}
	if lb.Status == model.LibSealed {
		return nil, model.ErrSealed
	}
	if existing, err := s.GetClusterByFingerprint(libID, fp); err == nil {
		return existing, nil
	}
	res, err := s.db.Exec(
		`INSERT INTO fragment_clusters(library_id, fingerprint, status, mean_len, mean_c2t_5p, mean_g2a_3p, size, created_at)
		 VALUES(?,?,?,?,?,?,?,?)`,
		libID, fp, model.FragRaw, meanLen, meanC2T5p, meanG2A3p, size, now())
	if err != nil {
		return nil, fmt.Errorf("store: insert cluster: %w", err)
	}
	id, _ := res.LastInsertId()
	return s.GetCluster(id)
}

// GetClusterByFingerprint reads the existing cluster for an idempotent rerun.
func (s *Store) GetClusterByFingerprint(libID int64, fp string) (*model.FragmentCluster, error) {
	row := s.db.QueryRow(
		`SELECT id, library_id, fingerprint, status, mean_len, mean_c2t_5p, mean_g2a_3p, size, created_at
		 FROM fragment_clusters WHERE library_id = ? AND fingerprint = ?`, libID, fp)
	c := &model.FragmentCluster{}
	if err := row.Scan(&c.ID, &c.LibraryID, &c.Fingerprint, &c.Status, &c.MeanLen, &c.MeanC2T5p, &c.MeanG2A3p, &c.Size, &c.CreatedAt); err != nil {
		return nil, err
	}
	return c, nil
}

// GetCluster 按 ID 读取片段簇。
func (s *Store) GetCluster(id int64) (*model.FragmentCluster, error) {
	row := s.db.QueryRow(
		`SELECT id, library_id, fingerprint, status, mean_len, mean_c2t_5p, mean_g2a_3p, size, created_at
		 FROM fragment_clusters WHERE id = ?`, id)
	c := &model.FragmentCluster{}
	if err := row.Scan(&c.ID, &c.LibraryID, &c.Fingerprint, &c.Status, &c.MeanLen, &c.MeanC2T5p, &c.MeanG2A3p, &c.Size, &c.CreatedAt); err != nil {
		if err == sql.ErrNoRows {
			return nil, model.ErrUnknownCluster
		}
		return nil, fmt.Errorf("store: get cluster: %w", err)
	}
	return c, nil
}

// ListClustersByLibrary 列出某文库的片段簇。
func (s *Store) ListClustersByLibrary(libID int64) ([]*model.FragmentCluster, error) {
	rows, err := s.db.Query(
		`SELECT id, library_id, fingerprint, status, mean_len, mean_c2t_5p, mean_g2a_3p, size, created_at
		 FROM fragment_clusters WHERE library_id = ? ORDER BY id ASC`, libID)
	if err != nil {
		return nil, fmt.Errorf("store: list clusters: %w", err)
	}
	defer rows.Close()
	var out []*model.FragmentCluster
	for rows.Next() {
		c := &model.FragmentCluster{}
		if err := rows.Scan(&c.ID, &c.LibraryID, &c.Fingerprint, &c.Status, &c.MeanLen, &c.MeanC2T5p, &c.MeanG2A3p, &c.Size, &c.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

// UpdateClusterStatus 更新片段簇状态（校验流转）。封存文库下的簇保持只读，拒绝任何状态修改。
func (s *Store) UpdateClusterStatus(id int64, to string) (*model.FragmentCluster, error) {
	c, err := s.GetCluster(id)
	if err != nil {
		return nil, err
	}
	lb, err := s.GetLibrary(c.LibraryID)
	if err != nil {
		return nil, err
	}
	if err := model.ValidateLibraryMutable(lb.Status); err != nil {
		return nil, err
	}
	if err := model.ValidateFragmentTransition(c.Status, to); err != nil {
		return nil, err
	}
	if _, err := s.db.Exec(`UPDATE fragment_clusters SET status = ? WHERE id = ?`, to, id); err != nil {
		return nil, fmt.Errorf("store: update cluster: %w", err)
	}
	return s.GetCluster(id)
}

// ClusterCountByStatus 统计某文库各状态片段簇数量（用于快照 payload）。
func (s *Store) ClusterCountByStatus(libID int64) (map[string]int, error) {
	rows, err := s.db.Query(`SELECT status, COUNT(*) FROM fragment_clusters WHERE library_id = ? GROUP BY status`, libID)
	if err != nil {
		return nil, fmt.Errorf("store: cluster count: %w", err)
	}
	defer rows.Close()
	out := map[string]int{}
	for rows.Next() {
		var st string
		var n int
		if err := rows.Scan(&st, &n); err != nil {
			return nil, err
		}
		out[st] = n
	}
	return out, rows.Err()
}
