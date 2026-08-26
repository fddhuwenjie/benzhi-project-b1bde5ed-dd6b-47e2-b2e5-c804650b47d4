# BENZHI_README

## 项目说明
- 项目：benzhi-project-b1bde5ed-dd6b-47e2-b2e5-c804650b47d4
- 项目用途：已完整实现海缆维护窗口放行台：以浏览器工作台和同源 JSON API 串联建档、五类准入证据、三方协调、四项风险审查、开工门禁、现场进展、严重偏差暂停复核、四类关闭证据及冻结归档；本地持久化具有乐观并发、命令幂等、事务恢复、追加式哈希链审计和确定性关闭与审计摘要。
- Go 工具链：`golang:1.23`
- 前端工具链：原生 HTML、CSS 和 JavaScript，由 Go 服务直接提供

## 项目描述
- 项目名称：海缆维护窗口放行台
- 项目介绍：面向海底通信光缆维护组织方的单流程浏览器应用，将维护窗口申请、船舶与装备核验、相关方协调、风险审查、开工门禁、现场偏差处置和完工归档串成可追溯的状态闭环，避免在准入条件不完整时进入海上作业阶段。
- 项目概述：面向海底通信光缆维护组织方的单流程浏览器应用，将维护窗口申请、船舶与装备核验、相关方协调、风险审查、开工门禁、现场偏差处置和完工归档串成可追溯的状态闭环，避免在准入条件不完整时进入海上作业阶段。
- 核心工作流：维护协调员建立草稿并提交船舶、人员和装备证据，相关方完成逐项确认后由安全审查员评估风险与控制措施；开工前门禁全部通过即激活维护窗口，现场负责人持续记录进展并处置偏差，作业完成后提交恢复证据并关闭为不可变归档。
- 对外接口：由 Go 服务提供原生 HTML、CSS 和 JavaScript 的浏览器工作台，同源 JSON 接口承担查询和状态命令；页面覆盖个案列表、证据核验、协调确认、风险审查、开工门禁、现场日志、偏差处置和关闭摘要，不引入 Node 构建链。

## 标准构建、运行和测试命令
进入容器后执行：
cd '/app' && GOTOOLCHAIN=local go build ./...

cd /app && GOTOOLCHAIN=local go run ./cmd/server -self-check -addr=127.0.0.1:19081

cd '/app' && GOTOOLCHAIN=local go test ./...

## Docker 构建和进入容器
chmod +x build_benzhi_docker.sh

./build_benzhi_docker.sh benzhi-project-b1bde5ed-dd6b-47e2-b2e5-c804650b47d4-amd64 linux/amd64

./build_benzhi_docker.sh benzhi-project-b1bde5ed-dd6b-47e2-b2e5-c804650b47d4-arm64 linux/arm64

docker run -it benzhi-project-b1bde5ed-dd6b-47e2-b2e5-c804650b47d4-amd64:latest

## 题目验证命令
1. 预期退出码 0：`go test ./...`
2. 预期退出码 0：`go run ./cmd/server -self-check -addr=127.0.0.1:19081`
