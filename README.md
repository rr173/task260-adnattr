# 古 DNA 片段损伤与污染归因服务（task260-adnattr）

面向古 DNA 研究者的后端服务：导入测序片段摘要、文库批次与空白对照，计算末端脱氨损伤轮廓，
比对空白对照，对片段簇的异常碱基替换进行污染归因（真实降解 vs 现代实验污染），并发布不可变的可信度快照。

## 标准命令

```bash
CGO_ENABLED=0 GOTOOLCHAIN=local go build -o adnattr ./cmd/adnattr
./adnattr --addr :8080 --db ./adnattr.db
./adnattr --smoke-test
CGO_ENABLED=0 GOTOOLCHAIN=local go vet ./...
CGO_ENABLED=0 GOTOOLCHAIN=local go test ./...
```

## 业务域

- 文库批次（library batch）：一次建库测序产物。
- 片段摘要（fragment summary）：对测序读段聚合后的长度、末端碱基替换率等统计。
- 末端损伤轮廓（damage profile）：5' 端 C→T、3' 端 G→A 脱氨率与长度分布——古 DNA 因年代久远呈典型末端富集、短片段特征。
- 空白对照（blank control）：无模板/空白提取的负对照，理论上不应出现古代信号；若某文库与空白高度相似，则提示现代污染。
- 归因候选（attribution candidate）：降解 / 现代污染 / 证据不足 / 确认。

## API 入口（前缀 /api，详见 BENZHI_README.md）

文库、片段簇、对照、损伤分析、污染裁决、快照发布、统计与自检，共 28 个端点。
