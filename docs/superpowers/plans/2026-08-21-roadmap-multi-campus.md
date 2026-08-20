# HDU-Auto-Scheduling-Script 后期规划（2026-08-21）

> **For agentic workers:** 后续落地建议使用 superpowers:subagent-driven-development 或
> superpowers:executing-plans 按任务逐条执行。任务用复选框（`- [ ]`）跟踪。

**Goal:** 把“HDU 专用选退课脚本”演进为一个**可长期维护、可多校适配、体验可信**的本地智能选课工具。
**Architecture:** 现状是“主站（导出/排课）+ Smart Agent（智能选课执行）+ vendor KillCourse + testlab 验收”
四层。未来是把“学校接口”抽象成可插拔适配器（Portal Adapter），学校差异收敛到 profile/映射表，
业务逻辑与 UI 保持学校无关。
**Tech Stack:** Go（后端/主站/适配器）、原生 Web（Smart Agent UI，无框架，便于单 exe 内嵌）、
Node/PowerShell（验收脚本、发布）、可选（Python 指标、Android 备忘）。
**Spec:** 本文档即方向性 spec；细则按任务各自展开成独立实施计划（遵循 `docs/superpowers/plans/` 惯例）。

## 全局约束

- 本地工具定位：默认**不上云**，凭据只落本地；对外提供自托管/局域网可选方案时需明确合规边界。
- 学校差异只允许出现在**适配层**（portal profile / 字段映射 / 会话与验证码策略），业务逻辑不得出现校名分支。
- 每次适配必须有 testlab（loopback mock）级别验收 + rowSchema/抓包诊断回填，禁止“写死学校字段猜”进主干。
- 新能力沿用既有验收设施：`scripts/testlab-acceptance.ps1` + `custom-*.js` 全链路必须通过才发布。
- 发布保持 v0.5.x 规则：VERSION + README 变更 → `build-release.ps1` → `release-integrity-check.ps1`。

---

## 一、战略方向总览（广）

| 方向 | 内容 | 优先级 |
|---|---|---|
| A. 可靠性工程 | 会话保鲜/重登重试（已有）、错误分级、请求限流与退避（已有）、审计日志、崩溃自愈 | P0 持续 |
| B. 产品化 | 一键安装包、自动更新、配置导入导出、多档案、快捷键、i18n | P1 |
| C. 多校适配 | 正方/强智/URP 等适配器化（见第三节专项） | P1（用户点名） |
| D. 数据服务 | 课表导出 ICS/全景图、空闲教室、选课热度统计、先修/冲突检查 | P2 |
| E. AI 增强 | 自然语言目标课表（“周三下午别排课”）、智能冲突建议、抢课时机预测、落点分析 | P2 |
| F. 安全与合规 | 凭据本机加密、最小化上传、开源/第三方 NOTICE、免责声明与使用条款 | P0 贯穿 |
| G. 社区/生态 | 适配器模板市场、使用文档站、贡献指南、悬赏适配 | P3 |

## 二、分阶段路线

### Phase 0：收口与加固（已接近完成）
- [x] P4 稳定性六项全落地（重登、退避、状态上报、shape 漂移诊断）
- [x] 方案 B 实时容量（schema-first + 映射层 + origin 回退）
- [x] 发布完整性（SHA256SUMS + 校验脚本 + CI 绿）
- [ ] CI 上把 Node20 弃用告警清理干净
- [ ] 凭据本机加密（DPAPI/Keyring），而非明文 config.json

### Phase 1：多校适配（详见第三节）
- [ ] 定义 `Portal` 适配接口与 schools 配置 schema
- [ ] 把 HDU 现有能力收敛进 `HduZfPort`（纯重构，回归不变）
- [ ] 新增一所正方学校 PoC + 一个非正方系统 PoC
- [ ] 适配文档模板 `docs/ADAPT-A-NEW-SCHOOL.md`

### Phase 2：体验可信度（接 UX 审核 P1/P2）
- [ ] 顶部状态格新增“实时容量”卡（可发现性）
- [ ] 移动端顶栏折叠菜单 + ≥44px 热区
- [ ] 计数口径角标（目标/课程库/当前/实时 的来源说明）
- [ ] 全局操作反馈独立化（每类操作独立 in-flight/成功/失败区）

### Phase 3：数据与 AI
- [ ] 课表导出 ICS/图片全景 + 空闲时段视图
- [ ] 自然语言→目标课表解析
- [ ] 选课热度/余量走势迷你图（复用 live-capacity 数据攒时序）
- [ ] 先修检查与冲突智能提示

### Phase 4：发布/生态
- [ ] Windows 安装器 + 自动更新通道
- [ ] 使用文档站 + 贡献指南 + 网关免责声明
- [ ] 可选：局域网自托管（明确不上公网的边界）

---

## 三、跨校适配专项（用户点名方向，重点展开）

### 3.1 为什么可以抽象
HDU 链路已把“登录/导出课程/个人课表/余量/选退课”走通，且 Smart Agent 已有：
- 抓包诊断工具 `POST /api/course/live-capacity/capture`（输出 rowSchema：键名/类型/样例）
- 教务与砍课两套登录（CAS 统一身份认证 + newjw）
- schema-first 的字段映射层（`mapCapacityRow` 候选键表）——已证明“未知字段”能被收敛进一处

这正是适配器的雏形：**换一所学校 = 换一个 portal profile + 换一张字段映射表 + 换一套 mock fixture**。

### 3.2 适配器抽象（新增层）
```
web/ 业务逻辑（校无关）
  └─ portal/  ← 新增
       ├─ portal.go        Portal 接口 + 能力协商
       ├─ profile.go       schools/<school>/profile.json（URL、认证、学期编码、能力开关）
       ├─ hduzf/           HDU/正方 实现（迁移现有 login/export/capacity/killcourse）
       ├─ otherschool/...  新学校实现
       └─ map.go           通用字段探测（复用 mapCapacityRow 思路，提升为共享）
testdata/schools/<school>/  每校 mock fixtures + server.out 契约
```

### 3.3 适配一所新学校的标准流程（写进 `docs/ADAPT-A-NEW-SCHOOL.md`）
1. 收集：教务系统名称/厂商（正方、强智、URP、青果…）、登录方式（CAS、验证码、微信/企微扫码）、
   课程搜索页 URL、余量页 URL、选/退课 URL。
2. 抓包：复用 capture 工具拿 rowSchema → 填 profile 字段映射。
3. 契约测试：建 `testdata/schools/<school>/` mock fixtures，与抓包字段一一对应。
4. 实现：写 `<school>Port`，只动 portal 层；业务层/前端不改。
5. 验收：testlab acceptance 为该学校新增 profile 场景，全绿 + CI。
6. 发布：纳入 multi-campus release（`school=xxx` 生效）。

### 3.4 任务分解（粗粒度，供后续细化为实施计划）
- [ ] **Task 1** 定义 `Portal` 接口与 `schools/<school>/profile.json` schema（登录、导出、课表、余量、选退、能力清单）；
      含 Go 结构体、JSON 样例、默认 HDU profile。文件：`portal/*`。
- [ ] **Task 2** 将 HDU 现有登录/导出/余量/选退逻辑收敛进 `hduzf` 实现，**行为不变**，
      以现有全量 testlab acceptance + Go 测试为回归基线。
- [ ] **Task 3** 提升字段探测为共享 `portal/map.go`，实现“rowSchema 自动出 profile 映射草稿”
      辅助工具（读 `capacity-capture-diagnosis.json` 自动建议 `rl/xkrs/skrs/syl`）。
- [ ] **Task 4** 新增一所**正方系统**学校 PoC（与 HDU 同族，复用最多）：新 profile + mock + 一门课验证。
- [ ] **Task 5** 新增一所**非正方系统** PoC（验证抽象不是贴皮）：按 3.3 流程完整走一遍。
- [ ] **Task 6** `ADAPT-A-NEW-SCHOOL.md` 教程 + schools 目录模板 + 验收清单。
- [ ] **Task 7** UI 支持学校切换（设置里选校），标题/学期/能力按 profile 变化；移动端不回归。

### 3.5 适配风险与对策
- 验证码/短信/扫码：抽象为“认证策略”（CAS/验证码 OCR/扫码/手动 Cookie 导入），
  OCR 走可插拔服务，默认不依赖云。
- 接口字段每年漂移：shape 漂移诊断（已有）+ 抓包回填映射（已有）兜底。
- 频率限制/封 IP：统一限流阀值 + 指数退避（已有），禁止高频并发。
- 合规：仅本人学习/排课用途；README 强调不传播敏感数据。
- 工作量：正方同族 3–5 个适应点；非正方 6–10 个，需真实抓包配合。

---

## 四、执行建议

1. 多校适配作为**最大杠杆**：桌面端可用的选课工具不多，能覆盖“正方系以外”就有差异化。
2. 每个 Phase 独立验收、独立发布（0.5.x 递增），避免大爆炸式重构。
3. 继续坚持“testlab 先行，mock 后真实”，真实抓包只在选课期窗口做，平时不空转。
4. 安全与合规（本地凭据加密、免责声明）建议在 Phase 1 与多校适配并行推进，避免上线后被质疑。
5. 方向取舍建议：**A(可靠) > C(多校) > B(产品化) > E(AI)**，D/F/G 视社区反馈再加码。

## 五、执行交接

本规划（尤其是第三节跨校适配）如需开工，可二选一执行：
1. **子代理驱动（推荐，用户偏好）**：每个 Task 派一个全新子代理实现，任务间做两级审核。
2. **本会话内分批执行**：按 executing-plans 用检查点推进。

等待用户确认起点（建议从 Task 1 的 Portal 接口 + profile schema 开始）。
