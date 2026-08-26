// Package snapshot 组装并发布不可变的可信度快照（冻结对照批次）。
package snapshot

import (
	"encoding/json"

	"task260-adnattr/internal/model"
	"task260-adnattr/internal/store"
)

// SnapshotPayload 快照 JSON 摘要内容。
type SnapshotPayload struct {
	LibraryID     int64                         `json:"library_id"`
	LibraryName   string                        `json:"library_name"`
	DamageProfile *model.DamageProfile          `json:"damage_profile"`
	Attributions  []*model.AttributionCandidate `json:"attributions"`
	ClusterCounts map[string]int                `json:"cluster_counts"`
}

// BuildPayload 汇总文库、损伤轮廓、归因候选与片段簇状态计数，序列化为 JSON 摘要。
func BuildPayload(s *store.Store, libID int64) (string, error) {
	lb, err := s.GetLibrary(libID)
	if err != nil {
		return "", err
	}
	prof, err := s.GetDamageProfile(libID)
	if err != nil {
		return "", err
	}
	attrs, err := s.ListAttributionsByLibrary(libID)
	if err != nil {
		return "", err
	}
	counts, err := s.ClusterCountByStatus(libID)
	if err != nil {
		return "", err
	}
	p := SnapshotPayload{
		LibraryID:     libID,
		LibraryName:   lb.Name,
		DamageProfile: prof,
		Attributions:  attrs,
		ClusterCounts: counts,
	}
	b, err := json.MarshalIndent(p, "", "  ")
	if err != nil {
		return "", err
	}
	return string(b), nil
}
