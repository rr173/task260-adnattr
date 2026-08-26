基于 Go 实现的古 DNA 片段损伤与污染归因 Web 项目，一款后端服务，计算末端脱氨损伤轮廓、比对空白对照并归因现代污染与真实降解信号。

# BENZHI 评测说明

本目录为 Git 根（`go.mod` 位于此），提供可运行、可测试的 Go 服务，使用纯 Go SQLite 持久化（`modernc.org/sqlite`，无需 CGO）。

## 运行命令

```bash
# 构建
CGO_ENABLED=0 GOTOOLCHAIN=local go build -o adnattr ./cmd/adnattr

# 启动 HTTP 服务（默认 :8080，库文件 ./adnattr.db）
./adnattr --addr :8080 --db ./adnattr.db

# 自检（不启动长驻服务，建库、跑核心闭环、关闭重开验证恢复后以 0 退出）
./adnattr --smoke-test
```

## 业务闭环

导入片段摘要 / 文库批次 / 空白对照 → 计算末端脱氨损伤轮廓（5' C→T、3' G→A、长度分布）
→ 与空白对照比较相似度 → 归因现代污染或真实降解 → 研究者确认/排除污染批次 →
发布不可变可信度快照（冻结对照批次）。

## API 契约（前缀 /api，共 29 个端点）

- `POST /api/libraries` 创建文库批次
- `POST /api/libraries/{id}/advance` 推进状态机
- `POST /api/libraries/{id}/analyze` 计算损伤轮廓 + 归因
- `POST /api/libraries/{id}/cluster` 片段聚类
- `POST /api/libraries/{id}/exclude-batch` 排除污染批次
- `POST /api/libraries/{id}/seal` 封存
- `GET /api/libraries` / `GET /api/libraries/{id}`
- `POST /api/fragments` / `POST /api/fragments/batch` 幂等导入片段摘要
- `GET /api/fragments?library_id=` / `GET /api/fragments/{id}`
- `GET /api/fragment-clusters?library_id=` / `GET /api/fragment-clusters/{id}`
- `POST /api/fragment-clusters/{id}/classify` 片段簇归类
- `POST /api/controls` / `GET /api/controls` / `GET /api/controls/{id}`
- `POST /api/controls/{id}/associate` 关联参考对照
- `GET /api/damage-profiles?library_id=`
- `GET /api/attributions?library_id=` / `GET /api/attributions/{id}`
- `POST /api/attributions/{id}/confirm` 确认归因候选
- `POST /api/snapshots` 发布可信度快照（冻结对照）
- `GET /api/snapshots?library_id=` / `GET /api/snapshots/{id}`
- `POST /api/snapshots/{id}/supersede` 替代旧版本
- `GET /api/stats` / `GET /api/self-check` 统计与自检

## 状态机

- 文库批次：receiving → pending_analysis → needs_review → published → sealed
- 片段簇：raw → damage_consistent / contamination_suspected / low_quality → excluded
- 归因候选：open → confirmed（kind ∈ degradation / modern_contamination / insufficient_evidence）
- 可信度快照：draft → published → superseded

## --smoke-test 契约

真实建库、建空白对照、导入两条文库片段（一条典型古 DNA 降解、一条现代污染）、
聚类、分析归因、确认/排除、发布快照，关闭并重新打开同一数据库验证恢复，全部断言通过后以退出码 0 结束。
