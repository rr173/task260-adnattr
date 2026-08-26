// Package damage 计算文库批次的末端脱氨损伤轮廓（5' 端 C→T、3' 端 G→A 与长度分布）。
package damage

import (
	"fmt"

	"task260-adnattr/internal/model"
	"task260-adnattr/internal/store"
)

// ComputeProfile 聚合某文库全部片段摘要，得到末端脱氨损伤轮廓并持久化。
// 古 DNA 因年代久远，其片段在 5' 端富集 C→T 脱氨、3' 端富集 G→A，且平均长度短；
// 现代污染片段通常不具此特征。该轮廓是后续污染归因的输入。
func ComputeProfile(s *store.Store, libID int64) (*model.DamageProfile, error) {
	st, err := s.AggregateFragments(libID)
	if err != nil {
		return nil, err
	}
	if st.Count == 0 {
		return nil, fmt.Errorf("%w: no fragments for library %d", model.ErrControlMissing, libID)
	}
	return s.UpsertDamageProfile(libID, st.MeanC2T5p, st.MeanG2A3p, st.MeanLen, st.Count)
}
