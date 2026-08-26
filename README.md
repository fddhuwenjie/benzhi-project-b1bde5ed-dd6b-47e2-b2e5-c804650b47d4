# 海缆维护窗口放行台

海缆维护窗口放行台是一套面向海底通信光缆维护组织方的单流程浏览器应用。它把维护建档、船舶与装备核验、三方协调、风险审查、开工门禁、现场日志、严重偏差暂停与复核恢复，以及完工冻结归档串成一条可追溯链路。所有数据保存在本机，不依赖外部服务或 Node 构建链。

## 业务规则

- 草稿必须具有有效时间窗、海缆区段、作业范围、协调员和初始风险基线。
- 船舶、关键人员、定位能力、维修装备和应急物资五类证据全部有效后，才能进入协调。
- 海事联络、海缆业主和现场船方三个席位全部确认后，才能进入安全审查。
- 拖锚、气象、通信中断及邻近设施风险均须有等级与控制措施。审查员也可携原因退回草稿补充。
- 人工气象读数、警戒区、通信测试和人员点名全部通过，且处于允许的窗口边界内，才会激活作业。
- `major` 或 `critical` 偏差会自动暂停；纠正措施经安全审查员复核通过后才能恢复。
- 海缆恢复、工具清点、海域撤场和相关方收讫四类证据齐备，且没有未解决严重偏差时，才可关闭。关闭摘要带有确定性 SHA-256 摘要，归档后拒绝所有写命令。

每个写命令都要求 `request_id`、`actor`、`role` 和 `expected_revision`。同一 `request_id` 与同一命令会返回原结果；复用标识提交不同载荷会返回幂等冲突。不同客户端同时修改同一个案时，过期的 `expected_revision` 会得到修订冲突。

## 构建与运行

要求 Go 1.23 或更新版本。

```text
go build ./cmd/server
go run ./cmd/server -addr=127.0.0.1:19081 -data-dir=./data
```

打开 `http://127.0.0.1:19081/` 使用浏览器工作台。默认地址是 `127.0.0.1:19081`，只允许回环监听。未显式传入 `-addr` 时，可以通过 `PORT` 指定端口，服务会绑定 `127.0.0.1:<PORT>`；显式 `-addr` 始终优先。

## 测试与自检

```text
go test ./...
go run ./cmd/server -self-check -addr=127.0.0.1:19081
```

`-self-check` 使用临时数据目录启动真实 HTTP 监听，通过公开 JSON 端点完成完整闭环，核验关闭摘要、审计数量和哈希链后主动退出。可用 `-self-check-timeout` 调整整体截止时间。

## 持久化与接口

`-data-dir` 下的 `cases/*.json` 是个案快照，`audit.jsonl` 是只追加哈希链，`requests.json` 是幂等结果索引，`transactions/` 用于崩溃恢复。写入通过同目录临时文件、文件同步和原子替换完成；启动恢复完成前不会开始监听。

主要查询包括 `GET /api/cases`、`GET /api/dashboard`、`GET /api/cases/{id}`、`GET /api/cases/{id}/closure-precheck`、`GET /api/cases/{id}/archive`、`GET /api/cases/{id}/audit`、`GET /api/cases/{id}/audit/verify`、`GET /api/cases/{id}/audit/summary` 和 `GET /api/cases/{id}/summary`。证据查询支持 `evidence_category` 与 `evidence_status` 筛选，状态会区分过期、无法覆盖窗口、七日内临期和有效。建档使用 `POST /api/cases`；状态命令统一提交至 `POST /api/cases/{id}/commands/{action}`，支持草稿修订、协调整改轮次、高风险控制核验和门禁复检。服务严格拒绝未知 JSON 字段、错误内容类型和超过 1 MiB 的请求体。
