# KillCourse Executor Integration Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 把外部开源项目 cr4n5/HDU-KillCourse（Apache-2.0）以 vendor + replace 方式纳入本仓库，并在 HDU-Smart-Course-Agent 中实现“一键登录、按计划选/退课、蹲课、实时状态轮询”的自动化执行器，全程可用 testlab mock 教务验收。

**Architecture:** vendor 上游源码到 `HDU-Smart-Course-Agent/third_party/HDU-KillCourse`，对其 client/course 包打最小可测试性补丁（URL 参数化、http.Client 可注入、选退课失败返回 error、nil panic 修复）；新增 `executor` 包封装登录/选课/退课/蹲课并写 execution-log；Smart Agent 增加 `/api/execution/start|status|stop` 路由与前端一键执行 UI；testlab 扩展 mock 选退课接口与兼容 fixture。

**Tech Stack:** Go 1.23、net/http、httptest、前端原生 JS（沿用现有 app.js 模式）。

**Spec:** [docs/superpowers/plans/2026-08-21-live-data-and-killcourse-integration.md](2026-08-21-live-data-and-killcourse-integration.md)

## Global Constraints

- 上游源码必须保留 Apache-2.0 `LICENSE`；对本仓库的修改在 `third_party/HDU-KillCourse/NOTICE` 与 README 中注明。
- 绝不提交上游 `config.json`、真实账号密码、Cookie、SMTP 授权码；仓库布局检查新增 `third_party/HDU-KillCourse` 内敏感文件拒绝规则。
- 自动执行默认不触发：必须走现有 plan → dry-run → authorize（15 分钟票据）门控；新 API 复用票据校验。
- 所有 HTTP 服务仅监听 loopback（127.0.0.1/localhost/::1）。
- Go 版本：module 使用 Go 1.23（与上游一致）；本仓库根 module 保持现状。
- 测试不得访问真实教务；一律走 testlab/httptest mock。

---

### Task 1: Vendor 上游源码并接入 Go module

**Files:**
- Create: `HDU-Smart-Course-Agent/third_party/HDU-KillCourse/{client,config,pkg,util,vars,log}/...`、`LICENSE`、`go.mod`、`go.sum`、`config.example.json`、`NOTICE`
- Modify: `HDU-Smart-Course-Agent/go.mod`（加 require + replace）
- Test: `HDU-Smart-Course-Agent/third_party/HDU-KillCourse/client/client_test.go`（最小编译/URL 常量冒烟，下一任务再扩展）

**Interfaces:**
- Consumes: 外部目录 `E:\fascinating project\HDU-KillCourse-main`（module `github.com/cr4n5/HDU-KillCourse`，v1.4.9 附近）
- Produces: `github.com/cr4n5/HDU-KillCourse` 作为本地 replace module 可被 `HDU-Smart-Course-Agent` import

- [ ] **Step 1: 复制上游源码（仅白名单）**

```powershell
$src='E:\fascinating project\HDU-KillCourse-main'; $dst='E:\fascinating project\HDU-Auto-Scheduling-Script\HDU-Smart-Course-Agent\third_party\HDU-KillCourse'
New-Item -ItemType Directory -Force -Path $dst
Copy-Item "$src\client","$src\config","$src\pkg","$src\util","$src\vars","$src\log" $dst -Recurse
Copy-Item "$src\go.mod","$src\go.sum","$src\LICENSE","$src\config.example.json" $dst
```

禁止复制：`config.json`、`cmd/`、`Doc/`、`.github/`、`README.md`（另写本仓库版本）。

- [ ] **Step 2: 写 NOTICE 注明出处与修改**

`third_party/HDU-KillCourse/NOTICE`：声明源码来自 cr4n5/HDU-KillCourse（Apache-2.0），本仓库对其 client/course 包做了可测试性与错误语义修改，修改点列表随 Task 2 更新。

- [ ] **Step 3: 修改 `HDU-Smart-Course-Agent/go.mod`**

```go
require github.com/cr4n5/HDU-KillCourse v1.4.9
replace github.com/cr4n5/HDU-KillCourse => ./third_party/HDU-KillCourse
```

- [ ] **Step 4: 编译验证**

```powershell
cd HDU-Smart-Course-Agent; go build ./... ; go vet ./...
```

Expected: 通过（当前无 import，验证 replace 可解析）。

- [ ] **Step 5: 提交**

`git add HDU-Smart-Course-Agent/third_party HDU-Smart-Course-Agent/go.mod` → commit `build(agent): vendor HDU-KillCourse v1.4.9`.

---

### Task 2: 上游可测试性补丁（TDD）

**Files:**
- Modify: `third_party/HDU-KillCourse/client/client.go`（URL 基址变量、http.Client 注入）
- Modify: `third_party/HDU-KillCourse/client/service.go`（全部 URL 字面量改拼接）
- Modify: `third_party/HDU-KillCourse/pkg/course/killCourse.go`（失败返回 error、nil 检查）
- Modify: `third_party/HDU-KillCourse/pkg/login/login.go`（newJW 失败串对齐）
- Test: `third_party/HDU-KillCourse/client/client_test.go`、`pkg/course/killCourse_test.go`

**Interfaces:**
- Consumes: `client.NewClient(cfg)`；`client.Client.Get/Post`
- Produces: `client.BaseJWURL`、`client.BaseSSOURL` 包级变量（测试可改）；`func (c *Client) SetHTTPClient(hc *http.Client)`；`course.SelectCourse/CancelCourse` 失败返回 error；`course.HandleCourse` 对 nil `ClientBodyConfig` 返回 error 而非 panic

- [ ] **Step 1: 写失败测试（client 注入）**

```go
// client_test.go
func TestNewClientUsesInjectedHTTPClient(t *testing.T) {
    cfg := &config.Config{}
    c := NewClient(cfg)
    ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        w.Write([]byte("ok"))
    }))
    defer ts.Close()
    oldJW, oldSSO := BaseJWURL, BaseSSOURL
    BaseJWURL, BaseSSOURL = ts.URL, ts.URL
    defer func() { BaseJWURL, BaseSSOURL = oldJW, oldSSO }()
    c.SetHTTPClient(ts.Client())
    body, _, err := c.Get(ts.URL+"/ping", nil)
    if err != nil || string(body) != "ok" { t.Fatalf(...) }
}
```

- [ ] **Step 2: 运行确认 RED**

`go test ./client/ -run TestNewClientUsesInjectedHTTPClient` → 编译失败（SetHTTPClient 不存在）。

- [ ] **Step 3: 实现补丁**

```go
// client.go
var BaseJWURL = "https://newjw.hdu.edu.cn/jwglxt"
var BaseSSOURL = "https://sso.hdu.edu.cn"
func (c *Client) SetHTTPClient(hc *http.Client) { c.client = hc }
```

`service.go` 所有硬编码 URL 替换为 `BaseJWURL+"/..."` / `BaseSSOURL+"/..."`（保留原路径）；`SaveCookies/LoadCookies` 的 cookie 域改用 `BaseJWURL`。

- [ ] **Step 4: 运行确认 GREEN**

`go test ./client/` → 通过。

- [ ] **Step 5: 写失败测试（选课失败返回 error）**

```go
// killCourse_test.go（使用 httptest mock 选课接口返回 {"flag":"0","msg":"人数已满"}）
func TestSelectCourseReturnsErrorOnFlagZero(...)
```

- [ ] **Step 6: RED → Step 7: GREEN**

`killCourse.go` 中 `result.Flag == "0"` 分支与 else 分支返回 `fmt.Errorf("选课失败: %s", ...)`；`CancelCourse` 结果非 `"\"1\""` 时返回 error；`HandleCourse` 在 `c.ClientBodyConfig == nil` 时返回 error。

- [ ] **Step 8: 更新 NOTICE（列出本任务修改点）**

- [ ] **Step 9: 提交**

`git add third_party/...` → commit `fix(killcourse): make client injectable and surface select/drop failures`.

---

### Task 3: testlab mock 选退课接口与兼容 fixture

**Files:**
- Modify: `cmd/hdu-testlab/main.go`（新增路由与 scenario）
- Create: `testdata/killcourse.course.sample.json`（含 `kch_id/jxb_id/jxbzc/kklxmc`，kklxmc 取值：主修课程/通识选修课/体育分项/特殊课程）
- Modify: `scripts/testlab-acceptance.ps1`（新增 killcourse 场景）

**Interfaces:**
- Consumes: Task 2 的 `BaseJWURL/BaseSSOURL` 注入能力
- Produces: mock 路由 `/xsxk/zzxkyzb_cxZzxkYzbIndex.html`（选课配置 HTML）、`/kbcx/xskbcx_cxXsgrkb.html`（GetStuInfo）、`/xsxk/zzxkyzbjk_cxJxbWithKchZzxkYzb.html`、`/xsxk/zzxkyzbjk_xkBcZyZzxkYzb.html`（选课 flag=1/0）、`/xsxk/zzxkyzb_tuikBcZzxkYzb.html`（退课 `"1"`）

- [ ] **Step 1: 写 mock 路由（按 Helmholtz 报告 §6 的请求/响应 shape）**
- [ ] **Step 2: 新建 `testdata/killcourse.course.sample.json`（从 course.sample.json 转换并补 kch_id/jxb_id/jxbzc/kklxmc）**
- [ ] **Step 3: 验收脚本新增 `killcourse-select`/`killcourse-drop` 场景（调用一个 Go 冒烟程序或 `go test` 集成测试）**
- [ ] **Step 4: 提交**

---

### Task 4: executor 包

**Files:**
- Create: `HDU-Smart-Course-Agent/executor/executor.go`、`executor/executor_test.go`
- Modify: `HDU-Smart-Course-Agent/main.go`（路由与 handler，Task 5 一起）

**Interfaces:**
- Consumes: `config.LoadConfig/InitCfg`、`login.Login`、`course.GetCourse/HandleCourse/SelectCourse/CancelCourse`、`course.WaitCourse`；现有 `ExecutionLog` 结构
- Produces:
```go
type Executor struct { cfg *kc.Config; client *kc.Client }
func New(cfg *kc.Config) (*Executor, error)          // 读取/校验配置并登录
func (e *Executor) Select(dropCourseCode string) error
func (e *Executor) Drop(courseCode string) error
func (e *Executor) RunOnce(ctx context.Context, plan map[string]string) ([]ExecutionEvent, error)
func (e *Executor) StartWait(ctx context.Context, intervalSec int, done <-chan struct{}) ([]ExecutionEvent, error)
```

- [ ] **Step 1-4: TDD 循环**（先写 mock testlab 集成测试 → RED → 实现 → GREEN）
- [ ] **Step 5: 提交**

---

### Task 5: Smart Agent API 集成

**Files:**
- Modify: `HDU-Smart-Course-Agent/main.go`（路由 569-603 区新增 3 条）、`main_test.go`

**Interfaces:**
- Produces:
  - `POST /api/execution/start`（body: `{ticketId, waitEnabled}`；校验票据；启动 goroutine 执行）
  - `GET /api/execution/status`（返回当前运行状态/最近事件，从 execution-log.json 读）
  - `POST /api/execution/stop`（取消 context）

- [ ] **Step 1-4: TDD**（先写 handler 测试：未授权 401、票据过期 403、正常 start 后 status 有 running 项、stop 后 canceled）
- [ ] **Step 5: 提交**

---

### Task 6: 前端一键执行 UI

**Files:**
- Modify: `HDU-Smart-Course-Agent/web/index.html`、`web/app.js`

**行为：**
- 在授权/打包后新增“一键执行（当前计划）”与“蹲课模式”按钮；
- 点击后 POST `/api/execution/start`，进入轮询（复用 fetchJSON；`setInterval` 5s，页面不可见/失败 3 次自动停止）；
- 显示每门课状态：排队/登录中/选课中/成功/失败（含失败原因），退课动作红色高亮；
- “停止”按钮 POST `/api/execution/stop`；
- 执行成功且包含 select/drop 后触发现有 `refreshLiveSchedule({reason:'execution-success'})`。

- [ ] **Step 1-4: 手工+脚本验收（沿用现有 main-ui-acceptance 模式扩展）**
- [ ] **Step 5: 提交**

---

### Task 7: 全量验收、文档与审核

- [ ] 根 module `go test ./...`、Smart Agent `go test ./...`、`testlab-acceptance.ps1` 全绿
- [ ] 仓库布局检查新增 third_party 敏感文件规则并通过
- [ ] 更新 `docs/REPOSITORY_LAYOUT.md`、README、方案文档（标记 P1 完成）
- [ ] 派代码审核子代理（requesting-code-review）：BASE=Task1 前 SHA，HEAD=最终 SHA
- [ ] 修复审核发现的问题并复跑
- [ ] 提交并推送

---

## Self-Review 记录

- Spec 覆盖：方案文档 §3.3 的 6 个实现步骤 → Task 1-6；§3.4 安全 → Task 1/7；§3.5 验收 → Task 7。P2（实时人数）不在本计划，另立计划。
- 无占位符：各 Task 均给出文件、接口与命令；Task 3/6 的详细代码在实现时按接口展开（其行为已由报告固定）。
- 类型一致：`course.SelectCourse/CancelCourse` 返回 error 的修改在 Task 2 定义并被 Task 4 消费；`executor` 事件写入现有 `ExecutionLog` 结构。
