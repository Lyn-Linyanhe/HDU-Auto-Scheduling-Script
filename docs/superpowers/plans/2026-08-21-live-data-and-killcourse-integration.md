# 2026-08-21 教务实时数据与 KillCourse 自动化集成方案

> 本文档回答三个问题：
> 1. “真实教务数据不稳定”到底是什么问题、影响面多大、可以怎么防御。
> 2. 行政班课表与教务实时选课人数如何接入。
> 3. 外部开源项目 HDU-KillCourse 如何纳入本仓库并实现自动选/退课。

---

## 一、真实教务数据稳定性：问题界定

### 1.1 现状

本仓库访问教务只有两类入口，全部依赖学校系统：

| 数据 | 入口 | 现状 |
| --- | --- | --- |
| 全校课程 | `newjw.hdu.edu.cn/jwglxt/rwlscx/rwlscx_cxRwlsIndex.html`（任务落实导出） | 接口导出，失败时浏览器兜底 |
| 个人课表 | `newjw.hdu.edu.cn/jwglxt/kbcx/xskbcx_cxXsgrkb.html` | 接口导出，失败时浏览器兜底 |
| 登录 | CAS 账号密码 / 新教务账号密码 / 浏览器 | 会话 Cookie 由本仓库维护 |

代码位置：`school/export.go`（接口与登录）、`school/browser_export.go`（浏览器兜底）。

### 1.2 不稳定点（按影响排序）

1. **登录会话失效**：Cookie（JSESSIONID / route）由学校服务端控制，异地登录、超时、密码修改都会失效。接口返回 403 / 跳登录页时，本仓库会重新走登录，但登录本身也可能被学校风控拦截。
2. **接口行为变化**：学校更新教务系统（换域名、改参数、改返回结构）后，本仓库的固定接口与解析逻辑会失效；历史已出现 403、页面未渲染课程列表等问题。
3. **验证码 / 单点登录**：CAS 密码登录依赖公钥加密流程，学校加验证码或改 SSO 跳转后需要同步适配；钉钉扫码登录在上游 KillCourse 中支持，但本仓库未实现。
4. **并发/频率限制**：短时间高频轮询可能被限流或触发风控；蹲课场景尤其敏感。
5. **数据时效性**：`course.json` 是导出快照，容量/人数字段是导出那一刻的值；本仓库已用 `stale: true` 明确标注，但没有自动刷新。

### 1.3 结论（明确问题）

**这不是本仓库可以“修复”的 bug，而是外部依赖的不确定性。** 本仓库能做的是防御性设计，把不稳定影响降到最低：

- 会话失败自动重登 + 指数退避；
- 接口失败自动回退浏览器兜底（已有）；
- 所有数据带 `sourceUpdatedAt` / `stale` 标记（已有）；
- 任何实时数据都允许“上次成功快照 + 失败提示”降级；
- 轮询频率可配置、默认保守（建议 ≥60s）；
- 把学校接口路径/参数集中到常量与诊断文件，便于接口变更时快速定位（已有 diagnosis 文件）。

**明确边界**：本仓库承诺“数据尽可能新 + 失败可解释 + 快照可回退”，不承诺“教务永远可用”。

---

## 二、行政班课表 + 教务实时人数方案

### 2.1 行政班课表

**目标**：用户输入行政班号（如 22012014），一键导出该班课表并作为底板导入排课页。

**数据源（待抓包确认）**：正方教务系统班级课表接口候选路径：

- `https://newjw.hdu.edu.cn/jwglxt/kbcx/xskbcx_cxXskbcxIndex.html?gnmkdm=N2152`（班级课表页面）
- 实际取数接口通常为 `kbcx/xskbcx_cxBjKb.html` 或 `kbcx/xskbcx_cxBjjkb.html`，POST 参数含 `bjmc`（班级名称）、`xnm`、`xqm`。

**实现步骤**：
1. 在现有登录会话（Cookie Jar）上，先 GET 班级课表页面完成 warmup，再 POST 取数接口；
2. 解析返回 JSON 中的课程列表，复用 `normalizeCourseData` 归一化为排课页可导入格式；
3. 输出文件 `class-schedule.json`（schema 与 `personal-schedule.json` 一致，`source: "class"`）；
4. 导出页新增“行政班课表”输入框 + “一键提取”按钮；成功后写入 `class-schedule.json` 并提示可导入排课页；
5. 失败时回退浏览器兜底（打开班级课表页，用户手动导出 JSON 后导入）。

**验收标准**：
- mock testlab 增加 `class-schedule` 场景（成功/失败）；
- 导出的班级课表能通过现有“导入 JSON 底板”流程匹配课程；
- 真实环境手工验证一次后，把接口路径写回常量并留诊断文件。

### 2.2 教务实时人数

**目标**：排课页/智能选课助手中显示“当前选课人数/容量”，并支持蹲课刷新。

**数据源**：任务落实接口 `rwlscx_cxRwlsIndex.html` 返回中已含 `jxbrl`（容量）、`jxbrs`/`xkrs`（已选人数）字段；本仓库导出 `course.json` 时已保留（`capacity` / `enrolled` / `selected`）。

**方案 A（推荐，复用现有会话）**：
- 新增 `/api/course/live-capacity` 接口：使用当前登录会话 POST 任务落实查询，返回全部或指定教学班的最新 `jxbrl/jxbrs/xkrs` 与 `sourceUpdatedAt`；
- 排课页“课程详情弹窗”显示实时人数，并标注“×分钟前更新”；
- 智能选课助手在蹲课/执行阶段，用同一接口做余量轮询，避免依赖外部脚本。

**方案 B（KillCourse 客户端）**：上游 KillCourse 的 `client` 包已经实现教务 HTTP 客户端与课程查询，集成后可直接复用其 `GetCourseOnline` 获取实时数据（见第三部分）。

**刷新策略**：
- 手动刷新按钮 + 可配置自动刷新（默认 60s，范围 15–300s）；
- 轮询只在“已登录且页面可见”时进行；连续 3 次失败自动停止并提示；
- 所有展示都带快照时间；无会话时显示“请先登录”。

**验收标准**：
- 接口返回 `stale: true/false` 与 `sourceUpdatedAt`；
- 有会话时能取到比 `course.json` 更新的 `jxbrs`（mock 与真实各验一次）；
- 无会话/会话失效时返回可读错误，前端显示上次快照。

---

## 三、KillCourse 自动化集成方案（含调研结论）

### 3.1 上游调研结论

- 仓库：`https://github.com/cr4n5/HDU-KillCourse`（Apache-2.0，244 stars，33 forks，最近提交 2026-07-05，v1.4.9，维护活跃）。
- 本机已有副本：`E:\fascinating project\HDU-KillCourse-main`（module `github.com/cr4n5/HDU-KillCourse`，Go 1.23）。
- 能力：选课、退课、蹲课（`wait_course`）、钉钉扫码登录、SMTP 通知、Web 配置页（6688 端口）。
- 结构：`cmd/HDU-KillCourse/main.go` 是**交互式程序**（启动后等待回车、结束前再等待回车），不适合直接子进程编排；但 `config`、`client`、`pkg/login`、`pkg/course` 都是普通 Go 包，**导出了可直接调用的函数**：
  - `config.LoadConfig()` / `config.InitCfg()`
  - `login.Login(cfg)`
  - `course.GetCourse(c, cfg)` / `course.GetCourseOnline(...)`
  - `course.SelectCourse(...)` / `course.CancelCourse(...)` / `course.HandleCourse(...)`
  - `course.KillCourse(ctx, channel, c, cfg, courses)` / `course.WaitCourse(...)`
- 配置格式与 Smart Agent 已生成的 `KillCourseConfig` 完全对应（`cas_login` / `newjw_login` / `cookies` / `time` / `course` / `wait_course` / `smtp_email` / `start_time`），集成时可直接沿用现有生成器。

### 3.2 集成方式对比

| 方式 | 优点 | 缺点 | 结论 |
| --- | --- | --- | --- |
| 子进程调用上游 exe | 改动最小 | 交互式 main 需要喂回车；状态只能靠日志解析；版本同步麻烦 | 不推荐 |
| `go.mod require` 远程依赖 | 上游更新直接可用 | 构建依赖 GitHub 可达性；Smart Agent 需单独 module 依赖；上游结构可能变动 | 可用但不是首选 |
| **vendor 到本仓库 + `replace`** | 离线可构建、版本锁定、保留 Apache-2.0 许可；可直接调用 pkg 函数 | 上游更新需手动同步（可写脚本） | **推荐** |

### 3.3 推荐方案：vendor + replace + Smart Agent 执行器

1. **vendor 上游源码**到 `HDU-Smart-Course-Agent/third_party/HDU-KillCourse/`（只复制 Go 源码、`LICENSE`、`go.mod`、`go.sum`，不含 `config.json`、`log/`、个人数据）；
2. 保留并引用 Apache-2.0 `LICENSE`，在 `NOTICE`/README 中标注出处与修改点（上游 main.go 的交互逻辑不会被使用）；
3. Smart Agent `go.mod` 增加：
   ```go
   require github.com/cr4n5/HDU-KillCourse v1.4.9
   replace github.com/cr4n5/HDU-KillCourse => ./third_party/HDU-KillCourse
   ```
4. 新增执行器包（如 `executor`），封装：
   - `LoadConfig()`：读取现有 `config.json`（由计划生成器产出）；
   - `Login()`：调用 `login.Login`，失败返回可读错误；
   - `Select/Drop`：逐课调用 `course.SelectCourse/CancelCourse`，记录每次结果（成功/失败/原因/时间）；
   - `WaitCourse`：蹲课模式调用 `course.WaitCourse`，支持 ctx 取消；
   - 进度与结果写入 `execution-log.json`（沿用现有 schema），供前端轮询展示；
5. Smart Agent 前端在“人工确认授权”后新增“一键执行/蹲课”入口：
   - 执行前展示完整计划与风险确认；
   - 执行中显示每门课状态（排队/登录/选课中/成功/失败）；
   - 失败课程支持“重试”与“换候选”；
6. 不自动绕过确认：保留现有 dry-run + 授权票据流程，默认不自动执行，用户点“开始”才运行。

### 3.4 安全与合规

- Apache-2.0：vendor 必须保留 `LICENSE`；如需修改，在 `NOTICE` 注明；
- **绝不提交** 上游 `config.json`、真实账号密码、Cookie、SMTP 授权码（仓库布局检查需新增对 `third_party/HDU-KillCourse` 内敏感文件的拒绝规则）；
- 自动执行属于高风险操作：默认关闭、需要显式授权；所有执行结果落本地日志，不上传。

### 3.5 实施步骤与验收

1. 拷贝上游源码到 `third_party/HDU-KillCourse`，跑通 `go test`（Smart Agent module）；
2. 用 testlab mock 教务先验证 `Login/SelectCourse/CancelCourse` 可调用；
3. 接入现有 `KillCourseConfig` 生成器 → `executor` 冒烟测试；
4. 前端授权 + 一键执行 UI + 状态轮询；
5. 验收：mock 全流程（生成计划 → 授权 → 执行 → 日志）通过；真实环境由用户手工确认后再执行一次。

---

## 四、实施顺序建议

1. **P0（文档与安全）**：本方案定稿；仓库布局检查增加第三方敏感文件拒绝规则；
2. **P1（KillCourse 集成）**：vendor + executor + mock 验收（约 1–2 天）；
3. **P2（实时人数）**：`/api/course/live-capacity` + 前端展示（约 0.5–1 天）；
4. **P3（行政班课表）**：接口抓包 + 导出页入口 + testlab 场景（约 0.5–1 天）；
5. **P4（稳定强化）**：重登退避、轮询频率限制、接口变更诊断增强（按真实反馈迭代）。

> 备注：行政班课表与实时人数的“真实接口路径”需要在真实登录会话下抓包确认；本文档中的候选路径基于正方教务通用约定，实施第一步应先用诊断工具验证。

---

## 五、P1 落地记录（2026-08-21）

- vendor：`HDU-Smart-Course-Agent/third_party/HDU-KillCourse`（Apache-2.0，见 `NOTICE` 修改清单）。
- executor：`HDU-Smart-Course-Agent/executor`（`RunOnce`/`StartWait`，登录走 `LoginSave` 不落盘 cookies）。
- API：`/api/execution/start|status|stop`，复用 dry-run → 授权票据（15 分钟过期）门控；单例执行。
- 前端：执行准备页新增“内置一键执行（当前计划）”与“蹲课模式”，1.5s 轮询状态并写入 `execution-log.json`。
- testlab：新增 KillCourse 协议 mock 路由与 `killcourse`/`killcourse-fail` 场景；`testlab-acceptance.ps1` 全绿。
- 仓库布局检查：新增 `third_party/HDU-KillCourse/config.json` 拒绝规则并通过负向测试。
- 真实环境执行仍需用户手工确认后由“一键执行”触发；P2–P4 另行排期。
