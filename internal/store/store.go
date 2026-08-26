// Package store 提供基于纯 Go SQLite（modernc.org/sqlite，CGO 无关）的持久化层。
//
// 设计要点：
//   - 所有建表使用 CREATE TABLE IF NOT EXISTS，幂等可重复执行；
//   - 片段指纹使用 UNIQUE 约束 + INSERT OR IGNORE 实现幂等导入；
//   - 启用外键与 busy_timeout，写入串行化不丢数据；
//   - 关闭重开同一路径后实体、轮廓、候选、快照均可从库读回（重启恢复）。
package store

import (
	"database/sql"
	"fmt"
	"time"

	_ "modernc.org/sqlite"
)

// Store 封装数据库连接与所有读写操作。
type Store struct {
	db *sql.DB
}

// OpenStore 打开（必要时创建）SQLite 数据库并完成迁移。
func OpenStore(path string) (*Store, error) {
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("store: open %q: %w", path, err)
	}
	db.SetMaxOpenConns(4)
	if _, err := db.Exec(`PRAGMA foreign_keys = ON;`); err != nil {
		_ = db.Close()
		return nil, err
	}
	if _, err := db.Exec(`PRAGMA busy_timeout = 5000;`); err != nil {
		_ = db.Close()
		return nil, err
	}
	s := &Store{db: db}
	if err := s.migrate(); err != nil {
		_ = db.Close()
		return nil, err
	}
	return s, nil
}

// Close 关闭数据库连接。
func (s *Store) Close() error { return s.db.Close() }

// DB 暴露底层 *sql.DB（供测试与高级查询）。
func (s *Store) DB() *sql.DB { return s.db }

// now 返回当前 Unix 毫秒，用作时间戳。
func now() int64 { return time.Now().UnixMilli() }

// migrate 执行全部建表语句。
func (s *Store) migrate() error {
	schema := []string{
		`CREATE TABLE IF NOT EXISTS library_batches (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			name TEXT NOT NULL,
			status TEXT NOT NULL,
			created_at INTEGER NOT NULL,
			sealed_at INTEGER,
			note TEXT NOT NULL DEFAULT ''
		);`,
		`CREATE TABLE IF NOT EXISTS fragment_summaries (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			library_id INTEGER NOT NULL,
			fingerprint TEXT NOT NULL UNIQUE,
			frag_len INTEGER NOT NULL,
			c2t_5p REAL NOT NULL,
			g2a_3p REAL NOT NULL,
			mean_base_error REAL NOT NULL,
			created_at INTEGER NOT NULL,
			FOREIGN KEY(library_id) REFERENCES library_batches(id)
		);`,
		`CREATE TABLE IF NOT EXISTS fragment_clusters (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			library_id INTEGER NOT NULL,
			fingerprint TEXT NOT NULL,
			status TEXT NOT NULL,
			mean_len REAL NOT NULL,
			mean_c2t_5p REAL NOT NULL,
			mean_g2a_3p REAL NOT NULL,
			size INTEGER NOT NULL,
			created_at INTEGER NOT NULL,
			FOREIGN KEY(library_id) REFERENCES library_batches(id)
		);`,
		`CREATE TABLE IF NOT EXISTS control_samples (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			name TEXT NOT NULL,
			is_blank INTEGER NOT NULL,
			mean_len REAL NOT NULL,
			mean_c2t_5p REAL NOT NULL,
			mean_g2a_3p REAL NOT NULL,
			library_id INTEGER,
			created_at INTEGER NOT NULL
		);`,
		`CREATE TABLE IF NOT EXISTS damage_profiles (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			library_id INTEGER NOT NULL UNIQUE,
			deam_5p REAL NOT NULL,
			deam_3p REAL NOT NULL,
			mean_len REAL NOT NULL,
			n_frags INTEGER NOT NULL,
			computed_at INTEGER NOT NULL,
			FOREIGN KEY(library_id) REFERENCES library_batches(id)
		);`,
		`CREATE TABLE IF NOT EXISTS attribution_candidates (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			library_id INTEGER NOT NULL,
			kind TEXT NOT NULL,
			status TEXT NOT NULL,
			score REAL NOT NULL,
			reason TEXT NOT NULL,
			created_at INTEGER NOT NULL,
			FOREIGN KEY(library_id) REFERENCES library_batches(id)
		);`,
		`CREATE TABLE IF NOT EXISTS confidence_snapshots (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			library_id INTEGER NOT NULL,
			status TEXT NOT NULL,
			control_batch_id INTEGER,
			payload TEXT NOT NULL,
			created_at INTEGER NOT NULL,
			published_at INTEGER,
			FOREIGN KEY(library_id) REFERENCES library_batches(id)
		);`,
		`CREATE TABLE IF NOT EXISTS library_control_links (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			library_id INTEGER NOT NULL,
			control_id INTEGER NOT NULL,
			UNIQUE(library_id, control_id),
			FOREIGN KEY(library_id) REFERENCES library_batches(id),
			FOREIGN KEY(control_id) REFERENCES control_samples(id)
		);`,
		`CREATE INDEX IF NOT EXISTS idx_frag_lib ON fragment_summaries(library_id);`,
		`CREATE INDEX IF NOT EXISTS idx_cluster_lib ON fragment_clusters(library_id);`,
		`CREATE INDEX IF NOT EXISTS idx_attr_lib ON attribution_candidates(library_id);`,
		`CREATE INDEX IF NOT EXISTS idx_snap_lib ON confidence_snapshots(library_id);`,
		// 片段簇按 (文库, 指纹) 幂等：先清理历史重复（保留最小 id），再加唯一索引，
		// 使重复执行聚类不会产生重复簇。CREATE IF NOT EXISTS 使本步可重复执行。
		`DELETE FROM fragment_clusters
			WHERE id NOT IN (
				SELECT MIN(id) FROM fragment_clusters GROUP BY library_id, fingerprint
			);`,
		`CREATE UNIQUE INDEX IF NOT EXISTS uq_cluster_lib_fp ON fragment_clusters(library_id, fingerprint);`,
	}
	for _, stmt := range schema {
		if _, err := s.db.Exec(stmt); err != nil {
			return fmt.Errorf("store: migrate: %w", err)
		}
	}
	return nil
}
