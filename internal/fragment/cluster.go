package fragment

import (
	"fmt"

	"task260-adnattr/internal/model"
	"task260-adnattr/internal/store"
)

// bucket 将数值按步长四舍五入量化，用于片段分桶。
func bucket(v, step float64) float64 {
	return float64(int(v/step+0.5)) * step
}

type agg struct {
	n                      int
	sumLen, sumC2T, sumG2A float64
}

// ClusterFragments 将某文库的片段摘要按统计近似分桶（长度/末端替换率），
// 对每个桶聚合均值后写入片段簇（初始状态 raw）。返回生成的片段簇列表。
func ClusterFragments(s *store.Store, libID int64) ([]*model.FragmentCluster, error) {
	frags, err := s.ListFragmentsByLibrary(libID)
	if err != nil {
		return nil, err
	}
	if len(frags) == 0 {
		return nil, fmt.Errorf("%w: no fragments to cluster for library %d", model.ErrControlMissing, libID)
	}
	type ckey struct {
		lenB, c2tB, g2aB int
	}
	groups := map[ckey]*agg{}
	for _, f := range frags {
		k := ckey{
			lenB: int(bucket(float64(f.FragLen), 10) * 10),
			c2tB: int(bucket(f.C2T5p, 0.05) * 100),
			g2aB: int(bucket(f.G2A3p, 0.05) * 100),
		}
		a := groups[k]
		if a == nil {
			a = &agg{}
			groups[k] = a
		}
		a.n++
		a.sumLen += float64(f.FragLen)
		a.sumC2T += f.C2T5p
		a.sumG2A += f.G2A3p
	}
	clusters := make([]*model.FragmentCluster, 0, len(groups))
	for _, a := range groups {
		meanLen := a.sumLen / float64(a.n)
		meanC2T := a.sumC2T / float64(a.n)
		meanG2A := a.sumG2A / float64(a.n)
		fp := Fingerprint(libID, int(meanLen), meanC2T, meanG2A, 0)
		c, err := s.InsertFragmentCluster(libID, fp, meanLen, meanC2T, meanG2A, a.n)
		if err != nil {
			return nil, err
		}
		clusters = append(clusters, c)
	}
	return clusters, nil
}
