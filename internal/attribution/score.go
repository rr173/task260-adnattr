// Package attribution 依据末端脱氨损伤轮廓与空白对照相似度，对片段簇的异常碱基替换
// 进行污染归因：真实降解（古 DNA 信号）或现代实验污染，或证据不足。
package attribution

import (
	"fmt"

	"task260-adnattr/internal/control"
	"task260-adnattr/internal/model"
)

// ancientDeamThreshold 末端脱氨率（5' 端 C→T 或 3' 端 G→A）高于此值视为典型古 DNA 信号。
// 真实降解需两端脱氨轮廓同时富集，仅单端信号不足以判定。
const ancientDeamThreshold = 0.08

// ancientLenThreshold 平均片段长度短于此值（bp）视为典型古 DNA 短片段特征。
const ancientLenThreshold = 120.0

// contaminationSimThreshold 与空白对照相似度高于此值视为现代污染嫌疑。
const contaminationSimThreshold = 0.5

// Classify 依据单个片段簇的均值统计给出状态归类：
//   - 两端脱氨均富集（5' C→T 与 3' G→A）且短片段 → damage_consistent（损伤一致，疑似真实古 DNA）；
//   - 脱氨弱且长度偏长 → contamination_suspected（污染可疑）；
//   - 其余（含仅一端脱氨富集等不完整信号）→ low_quality（低质，证据不足）。
func Classify(meanLen, deam5p, deam3p float64) string {
	switch {
	case deam5p > ancientDeamThreshold && deam3p > ancientDeamThreshold && meanLen < ancientLenThreshold:
		return model.FragDamageConsistent
	case deam5p <= ancientDeamThreshold && meanLen >= ancientLenThreshold:
		return model.FragContaminationSuspected
	default:
		return model.FragLowQuality
	}
}

// Score 计算归因结论：类型、置信分 [0,1] 与原因说明。
//   - 两端脱氨均富集（5' C→T 与 3' G→A）且片段短 → 真实降解（degradation），
//     与空白相似度无关（双端高脱氨+短片段天然区别于空白）；
//   - 脱氨弱、长度偏长且与空白高度相似 → 现代污染（modern_contamination）；
//   - 其余（含仅一端脱氨富集等不完整信号）→ 证据不足（insufficient_evidence）。
func Score(prof *model.DamageProfile, blank *model.ControlSample) (kind string, score float64, reason string, err error) {
	if prof == nil || prof.NFrags == 0 {
		return "", 0, "", fmt.Errorf("%w: empty damage profile", model.ErrControlMissing)
	}
	if blank == nil {
		return "", 0, "", fmt.Errorf("%w: blank control required", model.ErrControlMissing)
	}
	sim := control.SimilarityToBlank(blank, prof)
	ancient := prof.Deam5p > ancientDeamThreshold && prof.Deam3p > ancientDeamThreshold && prof.MeanLen < ancientLenThreshold

	switch {
	case ancient:
		return model.AttribDegradation, 0.85,
			fmt.Sprintf("5'端C→T脱氨率=%.3f、3'端G→A脱氨率=%.3f(均>%.2f)且平均长度=%.1fbp(<%.0f)，两端损伤轮廓与短片段共同呈典型古DNA降解特征",
				prof.Deam5p, prof.Deam3p, ancientDeamThreshold, prof.MeanLen, ancientLenThreshold), nil
	case sim >= contaminationSimThreshold:
		return model.AttribModernContamination, 0.80,
			fmt.Sprintf("末端脱氨弱(5'=%.3f,3'=%.3f)且长度偏长(%.1fbp)，与空白对照相似度=%.2f(>=%d%%)，提示现代实验污染",
				prof.Deam5p, prof.Deam3p, prof.MeanLen, sim, int(contaminationSimThreshold*100)), nil
	default:
		return model.AttribInsufficientEvidence, 0.40,
			fmt.Sprintf("损伤轮廓(5'脱氨=%.3f,3'脱氨=%.3f,长度=%.1fbp)与空白相似度=%.2f，两端损伤与短片段特征未共同满足，不足以判定",
				prof.Deam5p, prof.Deam3p, prof.MeanLen, sim), nil
	}
}
