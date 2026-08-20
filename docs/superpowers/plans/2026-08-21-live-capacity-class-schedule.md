# Live Capacity (P2) + Class Schedule (P3) Implementation Plan

> Focus: 教务“实时人数/余量”与“行政班课表”两块展示能力的实施路径。两者共享
> 登录会话、testlab mock 与诊断工具；P2 需要一次真实会话抓包确认字段，P3
> 本身可先基于已有的 `course.json`（含 `jxbzc` 授课班级）落地，不需要新接口。

## 现状（2026-08-21，来自当前工作区证据）

- Smart Agent 已有 `/api/course-capacity`：读取**本地快照**（`course.json` 导出的
  容量/人数字段），页面明确标记“非实时 / 快照更新时间”。
- 主站已有 `/api/export` 与 `/rwlscx/rwlscx_cxRwlsIndex.html?doType=query&gnmkdm=N1548`
  的课程导出能力（含 `jxbzc` 授课班级字段，`school/export.go`）。
- vendor 的 KillCourse 客户端已有 `client.SearchCourse`，请求
  `/xsxk/zzxkyzb_cxZzxkYzbPartDisplay.html`（余量查询），testlab 已 mock 该路由
  （`cmd/hdu-testlab/main.go` 的 `handlePartDisplay`）。
- testlab 的 `killcourse.course.sample.json` 只含 `jxb_id/jxbmc/jxbzc/kklxmc` 等
  基础字段，**不含容量/人数**——P2 需要扩展 fixture 与 mock。

## P2：教务实时人数（live capacity）

### 目标与验收标准

1. 在有效登录会话下，能抓到真实余量接口的响应 shape（字段名、数值单位、分页）。
2. `/api/course/live-capacity`：拉取一次实时容量，写入 `live-capacity.json`
   （TTL/快照语义），合并到课程列表的 `capacity/enrolled/selected/remaining`。
3. 页面在“实时容量”区展示来源时间；实时接口失败时**回退旧快照**并显示失败原因，
   行为与个人课表刷新一致（已知语义，见
   `2026-08-21-live-data-and-killcourse-integration.md` §六）。
4. 不得在非选课期/无会话时误报“实时数据有效”。

### 实施步骤（TDD，先 mock 后真实）

1. **抓包/诊断确认字段**（阻塞步骤，需真实会话）
   - 复用主站会话登录（`school/export.go` 的登录路径），对
     `/xsxk/zzxkyzb_cxZzxkYzbPartDisplay.html`（或抓包确认的等效地址）发一次
     `SearchCourseReq`（`Xkxnm/Xkxqm/Kklxdm/Yllist=1/Filterlist=教学班名`）。
   - 把响应原样写入 `capacity-capture-diagnosis.json`，字段清单列到本计划附录，
     供后续 mock 精确对齐（KillCourse 的 `GetIsCourseOk` 只关心 `tmpList` 空/非空，
     但本方案需要人数列）。
   - 交付物：附录 A（接口路径 + 请求参数 + 响应 JSON 示例）。

   > 工具已就绪（2026-08-21）：Smart Agent 新增
   > `POST /api/course/live-capacity/capture`（复用登录会话调用余量接口，原样保存
   > 原始响应到 `capacity-capture-diagnosis.json` 并返回 `topKeys/arrayCounts` 摘要），
   > 另输出 `rowSchema`（首行各字段的键名/类型/样例），真实字段一眼可见；
   > 已含 testlab 端到端验收（`smart-agent-live-capacity-capture-check.js`）与 Go 集成测试。
   > 现在只差一次真实登录会话触发抓包、回填附录 A 字段。
2. **testlab 扩展**：`handlePartDisplay` 返回真实 shape（含人数字段），并新增
   `capacity-ok` / `capacity-fail` 场景；`killcourse.course.sample.json` 补每门课
   的容量字段。
3. **Smart Agent 后端**：新增 `fetchLiveCapacity(ctx, baseURL, …)`（或放进 executor
   层复用登录），超时/失败落 `warnings` 不清旧文件；写 `live-capacity.json`；
   `/api/course/live-capacity` 读它并回退快照。
4. **前端**：容量卡片支持“实时 N 人 / 快照”两种来源标识与时间；桌面与 390px
   移动 smoke 无横向溢出（沿用 `smart-agent-ui-smoke.js` 断言）。
5. **验收**：`testlab-acceptance.ps1` 加 `capacity-ok/fail` 场景；全程 loopback。

## P3：行政班课表（class schedule）

> 状态：**第一版已落地（2026-08-21）**。离线聚合 + 前端展示 + mock 验收均完成；
> 若“行政班”指独立行政班编排表（非授课班级），仍走抓包流程，见下文风险。

### 事实澄清（避免重复抓包）

- `course.json` 已含 `jxbzc`（授课班级），`GetCourseToExcelResp` 已含
  `Jxbzc/Skdxssxy` 等字段（vendor `client/resp.go`）。因此行政班课表**首期可以
  纯离线**从已有课程库按“授课班级”聚合生成，不需要新增接口。
- 若“行政班”指的不是授课班级而是一套独立的行政班编排表，则属于另一接口：
  先走与 P2 相同的抓包诊断流程（候选路径 `/rwlscx/*` 任务落实），确认后再实现。

### 实施步骤

1. 从 `course.json` 提取 `{jxbzc, jxbmc, kcmc, sksj, jxdd, jzgxx, xf}`，按班级名
   分组 → `行政班-课程` 映射。
2. Smart Agent `/api/class-schedule`：当前该路由是“按 `groupId`（课程组）/
   `displayCode`（教学班）过滤课程库”的视角（`main.go` handleClassSchedule），
   并不是行政班视角。P3 新增 `className=<授课班级>` 查询参数（或新路由
   `/api/admin-class-schedule`），按 `jxbzc` 聚合返回该行政班全部课程。
3. 前端“班级课表”抽屉：选择授课班级 → 该班全部教学班列表 + 时间线视图。
4. testlab 补 `班级课表` fixture（复用 `killcourse.course.sample.json` 的
   多 jxbzc 数据）；UI smoke 无溢出。

### P3 落地记录（2026-08-21）

- 后端：`/api/class-options` 返回去重行政班列表（`name/count`）；`/api/class-schedule`
  支持 `className` 查询参数，按 `className/jxbzc` 的**分隔符分段精确匹配**
  （兼容 `;` `；` `,` `，` `、` 空格等连接符，见 `main.go` splitClassNames）。
- 前端：课程情报区新增“选择行政班 + 查看班级课表”，复用 scheduleCard 渲染该班全部课程。
- 数据：`testdata/course.sample.json` 补 `jxbzc`，全链路（testlab 导出 → Smart Agent）
  可携带班级数据。
- 验收：新增 `scripts/smart-agent-class-schedule-check.js`（`/api/class-options` 计数、
  `className=202601` 2 门 / `202602` 1 门），已接入 `testlab-acceptance.ps1`；桌面与
  390px 移动 UI smoke 均通过、无横向溢出。

## 依赖与风险

- P2 第一步的真实验证依赖一次有效登录会话（用户配合、选课期或可查询期）。
- 真实接口字段/权限可能在学期更迭时变化；用诊断文件固化，避免在代码里写死太多
  未知假设。
- P2 的实时人数不建议高频轮询（教务侧限流）；默认 60s，最低 30s，失败指数退避。

## Appendix A（占位，抓包后回填）

```text
- 接口路径：待确认（候选 /xsxk/zzxkyzb_cxZzxkYzbPartDisplay.html?gnmkdm=N253512）
- 关键请求参数：xkxnm, xkxqm, kklxdm, yllist=1, filterlist=<教学班名>,
  njdm_id_xs, zyh_id_xs
- 期望响应示例：待抓包填入字段名（容量/已选/报名/剩余）
```
