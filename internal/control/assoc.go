// Package control 处理空白/参考对照与文库损伤轮廓的相似度比较。
package control

import (
	"math"

	"task260-adnattr/internal/model"
)

// SimilarityToBlank 计算文库损伤轮廓与某空白（负）对照的相似度，返回值 [0,1]：
// 越接近 1 表示文库越像空白对照（缺失古代信号），越可疑为现代污染。
//
// 比较维度：平均片段长度、5' 端 C→T 脱氨率、3' 端 G→A 脱氨率，归一化欧氏距离。
func SimilarityToBlank(blank *model.ControlSample, prof *model.DamageProfile) float64 {
	dLen := (prof.MeanLen - blank.MeanLen) / 200.0
	dC2T := prof.Deam5p - blank.MeanC2T5p
	dG2A := prof.Deam3p - blank.MeanG2A3p
	dist := math.Sqrt(dLen*dLen + dC2T*dC2T + dG2A*dG2A)
	sim := 1.0 - math.Min(1.0, dist)
	if sim < 0 {
		sim = 0
	}
	return sim
}

// PickReference 从候选对照中挑选与文库轮廓差异最大的一个作为参考对照
// （差异越大越能凸显古代信号，而不是被空白污染信号主导）。返回挑选结果，无候选时返回 nil。
func PickReference(blanks []*model.ControlSample, prof *model.DamageProfile) *model.ControlSample {
	var best *model.ControlSample
	bestSim := 1.0
	for _, b := range blanks {
		sim := SimilarityToBlank(b, prof)
		if sim < bestSim {
			bestSim = sim
			best = b
		}
	}
	return best
}

// PreferLinkedBlanks uses explicitly associated blank controls when present;
// otherwise it falls back to the global blank-control pool.
func PreferLinkedBlanks(linked, blanks []*model.ControlSample) []*model.ControlSample {
	selected := make([]*model.ControlSample, 0, len(linked))
	for _, c := range linked {
		if c != nil && c.IsBlank {
			selected = append(selected, c)
		}
	}
	if len(selected) > 0 {
		return selected
	}
	return blanks
}
