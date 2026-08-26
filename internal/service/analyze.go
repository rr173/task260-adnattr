package service

import (
	"fmt"

	"task260-adnattr/internal/attribution"
	"task260-adnattr/internal/control"
	"task260-adnattr/internal/damage"
	"task260-adnattr/internal/model"
	"task260-adnattr/internal/snapshot"
)

// Analyze 执行文库分析流水线：
//  1. 计算末端脱氨损伤轮廓；
//  2. 选取空白对照（优先差异最大者作为参考）；
//  3. 评分污染归因（降解 / 现代污染 / 证据不足）；
//  4. 刷新未确认候选并写入新候选。
func (svc *Service) Analyze(libID int64) (*model.DamageProfile, *model.AttributionCandidate, error) {
	sealed, err := svc.Store.IsLibrarySealed(libID)
	if err != nil {
		return nil, nil, err
	}
	if sealed {
		return nil, nil, model.ErrSealed
	}
	prof, err := damage.ComputeProfile(svc.Store, libID)
	if err != nil {
		return nil, nil, err
	}
	linked, err := svc.Store.ListAnalysisControls(libID)
	if err != nil {
		return nil, nil, err
	}
	blanks, err := svc.Store.ListBlankControls()
	if err != nil {
		return nil, nil, err
	}
	blanks = control.PreferLinkedBlanks(linked, blanks)
	if len(blanks) == 0 {
		return nil, nil, fmt.Errorf("%w: no blank control available for attribution", model.ErrControlMissing)
	}
	ref := control.PickReference(blanks, prof)
	if ref == nil {
		ref = blanks[0]
	}
	kind, score, reason, err := attribution.Score(prof, ref)
	if err != nil {
		return nil, nil, err
	}
	if _, err := svc.Store.DeleteOpenAttributions(libID); err != nil {
		return nil, nil, err
	}
	cand, err := svc.Store.InsertAttribution(libID, kind, model.AttribOpen, score, reason)
	if err != nil {
		return nil, nil, err
	}
	// 依据同一判定阈值自动归类片段簇（raw → damage_consistent / contamination_suspected / low_quality）。
	clusters, err := svc.Store.ListClustersByLibrary(libID)
	if err != nil {
		return nil, nil, err
	}
	for _, c := range clusters {
		if c.Status != model.FragRaw {
			continue
		}
		to := attribution.Classify(c.MeanLen, c.MeanC2T5p, c.MeanG2A3p)
		if _, err := svc.Store.UpdateClusterStatus(c.ID, to); err != nil {
			return nil, nil, err
		}
	}
	return prof, cand, nil
}

// ExcludeBatch 排除污染批次：将文库中“污染可疑”片段簇标记为排除，
// 并将文库推进到需复核（needs_review）状态。
func (svc *Service) ExcludeBatch(libID int64) (*model.LibraryBatch, error) {
	clusters, err := svc.Store.ListClustersByLibrary(libID)
	if err != nil {
		return nil, err
	}
	for _, c := range clusters {
		if c.Status == model.FragContaminationSuspected {
			if _, err := svc.Store.UpdateClusterStatus(c.ID, model.FragExcluded); err != nil {
				return nil, err
			}
		}
	}
	lb, err := svc.Store.GetLibrary(libID)
	if err != nil {
		return nil, err
	}
	if lb.Status == model.LibPendingAnalysis {
		return svc.Store.AdvanceLibrary(libID, model.LibNeedsReview)
	}
	return lb, nil
}

// PublishSnapshot 组装可信度快照摘要、创建并发布（冻结对照批次），返回已发布快照。
// 仅当参照为真实存在的空白对照时才允许冻结进快照，拒绝不存在的对照编号，以免
// 快照引用无法追溯的对照批次。
func (svc *Service) PublishSnapshot(libID, controlID int64) (*model.ConfidenceSnapshot, error) {
	// 在创建快照草稿前先校验参照：不存在则 ErrUnknownControl，非空白则 ErrControlMissing，
	// 避免写入引用了不实对照批次的已发布快照。
	if err := svc.Store.ValidateSnapshotControl(controlID); err != nil {
		return nil, err
	}
	payload, err := snapshot.BuildPayload(svc.Store, libID)
	if err != nil {
		return nil, err
	}
	snap, err := svc.Store.CreateSnapshot(libID, payload)
	if err != nil {
		return nil, err
	}
	return svc.Store.PublishSnapshot(snap.ID, controlID)
}

// SupersedeSnapshot 将已发布快照标记为被替代（保留历史）。
func (svc *Service) SupersedeSnapshot(id int64) (*model.ConfidenceSnapshot, error) {
	return svc.Store.SupersedeSnapshot(id)
}

// Stats 聚合统计，供 /api/stats 使用。
func (svc *Service) Stats() (map[string]int64, error) {
	rows, err := svc.Store.DB().Query(`
		SELECT 'libraries', COUNT(*) FROM library_batches
		UNION ALL SELECT 'fragments', COUNT(*) FROM fragment_summaries
		UNION ALL SELECT 'clusters', COUNT(*) FROM fragment_clusters
		UNION ALL SELECT 'controls', COUNT(*) FROM control_samples
		UNION ALL SELECT 'attributions', COUNT(*) FROM attribution_candidates
		UNION ALL SELECT 'snapshots', COUNT(*) FROM confidence_snapshots`)
	if err != nil {
		return nil, fmt.Errorf("service: stats: %w", err)
	}
	defer rows.Close()
	out := map[string]int64{}
	for rows.Next() {
		var k string
		var v int64
		if err := rows.Scan(&k, &v); err != nil {
			return nil, err
		}
		out[k] = v
	}
	return out, rows.Err()
}
