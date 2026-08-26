package store

import (
	"database/sql"
	"fmt"

	"task260-adnattr/internal/model"
)

// CreateControl 创建空白/参考对照。isBlank=true 表示负对照（无模板/空白提取）。
func (s *Store) CreateControl(name string, isBlank bool, meanLen, meanC2T5p, meanG2A3p float64, libID int64) (*model.ControlSample, error) {
	if name == "" {
		return nil, model.ErrEmptyName
	}
	if libID > 0 {
		if _, err := s.GetLibrary(libID); err != nil {
			return nil, err
		}
	}
	var lib sql.NullInt64
	if libID > 0 {
		lib = sql.NullInt64{Int64: libID, Valid: true}
	}
	res, err := s.db.Exec(
		`INSERT INTO control_samples(name, is_blank, mean_len, mean_c2t_5p, mean_g2a_3p, library_id, created_at)
		 VALUES(?,?,?,?,?,?,?)`,
		name, boolToInt(isBlank), meanLen, meanC2T5p, meanG2A3p, lib, now())
	if err != nil {
		return nil, fmt.Errorf("store: create control: %w", err)
	}
	id, _ := res.LastInsertId()
	return s.GetControl(id)
}

// GetControl 按 ID 读取对照。
func (s *Store) GetControl(id int64) (*model.ControlSample, error) {
	row := s.db.QueryRow(
		`SELECT id, name, is_blank, mean_len, mean_c2t_5p, mean_g2a_3p, library_id, created_at
		 FROM control_samples WHERE id = ?`, id)
	c := &model.ControlSample{}
	var blank int
	var lib sql.NullInt64
	if err := row.Scan(&c.ID, &c.Name, &blank, &c.MeanLen, &c.MeanC2T5p, &c.MeanG2A3p, &lib, &c.CreatedAt); err != nil {
		if err == sql.ErrNoRows {
			return nil, model.ErrUnknownControl
		}
		return nil, fmt.Errorf("store: get control: %w", err)
	}
	c.IsBlank = blank != 0
	if lib.Valid {
		c.LibraryID = lib.Int64
	}
	return c, nil
}

// ListControls 列出全部对照。
func (s *Store) ListControls() ([]*model.ControlSample, error) {
	rows, err := s.db.Query(
		`SELECT id, name, is_blank, mean_len, mean_c2t_5p, mean_g2a_3p, library_id, created_at
		 FROM control_samples ORDER BY id ASC`)
	if err != nil {
		return nil, fmt.Errorf("store: list controls: %w", err)
	}
	defer rows.Close()
	var out []*model.ControlSample
	for rows.Next() {
		c := &model.ControlSample{}
		var blank int
		var lib sql.NullInt64
		if err := rows.Scan(&c.ID, &c.Name, &blank, &c.MeanLen, &c.MeanC2T5p, &c.MeanG2A3p, &lib, &c.CreatedAt); err != nil {
			return nil, err
		}
		c.IsBlank = blank != 0
		if lib.Valid {
			c.LibraryID = lib.Int64
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

// ListBlankControls 列出所有空白（负）对照，用于污染相似度比对。
func (s *Store) ListBlankControls() ([]*model.ControlSample, error) {
	all, err := s.ListControls()
	if err != nil {
		return nil, err
	}
	var out []*model.ControlSample
	for _, c := range all {
		if c.IsBlank {
			out = append(out, c)
		}
	}
	return out, nil
}

// ListAnalysisControls returns explicitly linked blank controls when the
// researcher selected any; otherwise it returns the global blank pool.
func (s *Store) ListAnalysisControls(libID int64) ([]*model.ControlSample, error) {
	linked, err := s.GetLinkedControls(libID)
	if err != nil {
		return nil, err
	}
	if len(linked) > 0 {
		return linked, nil
	}
	return s.ListBlankControls()
}

// ValidateSnapshotControl verifies that a published snapshot freezes a real
// blank control rather than an arbitrary or non-existent ID.
func (s *Store) ValidateSnapshotControl(controlID int64) error {
	c, err := s.GetControl(controlID)
	if err != nil {
		return err
	}
	if !c.IsBlank {
		return model.ErrControlMissing
	}
	return nil
}

// AssociateControl 将某对照关联到文库（锁定参考对照）。重复关联幂等。
//
// 参考对照关系不得形成自引用或跨文库循环：若被关联对照恰好由当前文库产生
// （自引用），或其来源文库已（直接或间接）依赖当前文库（跨文库循环），
// 则拒绝本次关联——即拒绝那一次会闭合循环的关联。
func (s *Store) AssociateControl(libID, controlID int64) error {
	lb, err := s.GetLibrary(libID)
	if err != nil {
		return err
	}
	if lb.Status == model.LibSealed {
		return model.ErrSealed
	}
	c, err := s.GetControl(controlID)
	if err != nil {
		return err
	}
	// 自引用：对照来自当前文库本身。
	if err := model.ValidateControlLink(libID, c.LibraryID); err != nil {
		return err
	}
	// 跨文库循环：该对照的来源文库已依赖当前文库时，再关联即闭合循环。
	// 空白对照无来源文库（LibraryID==0），不可能构成循环，跳过。
	if c.LibraryID > 0 && s.libraryDependsOn(c.LibraryID, libID, make(map[int64]bool)) {
		return model.ErrBatchCycle
	}
	if _, err := s.db.Exec(
		`INSERT OR IGNORE INTO library_control_links(library_id, control_id) VALUES(?,?)`, libID, controlID); err != nil {
		return fmt.Errorf("store: associate control: %w", err)
	}
	return nil
}

// libraryDependsOn 判断对照来源依赖链上 start 是否（直接或间接）到达 target：
// 自 start 出发，沿"该文库关联的参考对照 → 对照的来源文库"逐层展开，
// 一旦命中 target 即返回 true。seen 防止对已访问文库重复展开。
// 用于检测关联后是否会闭合跨文库循环。
func (s *Store) libraryDependsOn(start, target int64, seen map[int64]bool) bool {
	if start == target {
		return true
	}
	if seen[start] {
		return false
	}
	seen[start] = true
	linked, err := s.GetLinkedControls(start)
	if err != nil {
		return false
	}
	for _, c := range linked {
		if c.LibraryID > 0 && s.libraryDependsOn(c.LibraryID, target, seen) {
			return true
		}
	}
	return false
}

// GetLinkedControls 返回某文库已关联的对照。
func (s *Store) GetLinkedControls(libID int64) ([]*model.ControlSample, error) {
	rows, err := s.db.Query(
		`SELECT c.id, c.name, c.is_blank, c.mean_len, c.mean_c2t_5p, c.mean_g2a_3p, c.library_id, c.created_at
		 FROM control_samples c JOIN library_control_links l ON l.control_id = c.id
		 WHERE l.library_id = ? ORDER BY c.id ASC`, libID)
	if err != nil {
		return nil, fmt.Errorf("store: linked controls: %w", err)
	}
	defer rows.Close()
	var out []*model.ControlSample
	for rows.Next() {
		c := &model.ControlSample{}
		var blank int
		var lib sql.NullInt64
		if err := rows.Scan(&c.ID, &c.Name, &blank, &c.MeanLen, &c.MeanC2T5p, &c.MeanG2A3p, &lib, &c.CreatedAt); err != nil {
			return nil, err
		}
		c.IsBlank = blank != 0
		if lib.Valid {
			c.LibraryID = lib.Int64
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}
