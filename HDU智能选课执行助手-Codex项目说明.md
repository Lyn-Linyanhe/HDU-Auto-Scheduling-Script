# HDU 智能选课执行助手：给 Codex 的项目说明

## 背景

我正在做一个面向杭州电子科技大学选课场景的本地工具体系。目前已经有两个独立项目：

1. `HDU-Auto-Scheduling-Script`
   - 作用：离线排课、候选课表生成、课程约束筛选、个人课表底板导入、课表截图导出。
   - 当前状态：已经做到 `v0.5.0`，可作为独立 exe 使用。
   - 最新入口：`HDU-Auto-Scheduling-Script.exe`
   - 关键文件：
     - `course.json`：全校课表
     - `personal-schedule.json`：个人课表
   - 启动逻辑：
     - 有 `course.json` 就进入排课页
     - 没有 `course.json` 就进入课程导出页
     - 没有 `personal-schedule.json` 也允许排课，可之后补导

2. `HDU-KillCourse`
   - 作用：登录教务系统，执行选课、退课、蹲课。
   - 当前状态：基础抢课/选课/退课/蹲课功能已经比较完善。
   - 主要问题：
     - 前端界面比较弱
     - 需要手工填写要选/要退的课程
     - 某门课失败后不会自动给出替代方案
     - 执行结果主要靠日志，不够结构化
   - 配置核心：
     - `config.json`
     - `course` 字段是教学班名称到动作的映射：
       - `"1"` 表示选课
       - `"0"` 表示退课

现在我要新建第三个项目，暂定名称：

```text
HDU-Smart-Course-Agent
```

中文名：

```text
HDU 智能选课执行助手
```

这个新项目不是直接把两个旧项目揉成一个，而是作为上层“智能编排器”，把排课助手和选课脚本串起来。

## 当前实际进度（2026-07-08）

`HDU-Smart-Course-Agent` 已经在 `HDU-Auto-Scheduling-Script/HDU-Smart-Course-Agent/` 下完成第一版本地 Web 应用。

当前已经实现：

1. 本地 Go Web 服务
   - 默认地址：`http://127.0.0.1:6899`
   - 可通过 `HDU_AGENT_NO_BROWSER=1` 禁止自动打开浏览器。
   - 已可打包为 `HDU-Smart-Course-Agent.exe`。

2. 数据状态检测
   - 自动检测排课助手目录。
   - 自动检测 `HDU-KillCourse` 目录。
   - 读取 `course.json`、`personal-schedule.json`。
   - 显示课程数量、个人课表数量、当前学期。
   - 支持在页面中手动保存项目路径到 `agent-settings.json`。

3. 目标课表导入与 diff
   - 支持导入排课助手导出的目标课表 JSON。
   - 计算 `keep`、`select`、`drop`、`locked`。
   - 生成 `fallbackGroups`。
   - 生成 `risks`、`validationIssues`、`explanations`。

4. KillCourse 配置生成
   - 读取已有 `HDU-KillCourse/config.json`。
   - 保留账号、cookie、蹲课、邮箱、开始时间等非课程配置。
   - 只替换 `course` 字段。
   - `select` 写为 `"1"`。
   - `drop` 写为 `"0"`。
   - 支持直接写入 `HDU-KillCourse/config.json`。

5. 执行准备检查
   - 检查 KillCourse 目录。
   - 检查登录/cookie 信息。
   - 检查学年学期。
   - 检查课程动作合法性。
   - 检查 `start_time`。
   - 检查蹲课间隔。
   - 对退课动作标记高风险。

6. dry-run 执行控制台
   - 不真实执行选课/退课。
   - 检查 KillCourse 启动入口。
   - 检查磁盘上的 `config.json` 是否与当前生成配置一致。
   - 输出工作目录、命令、动作数量和风险日志。

7. 授权票据与启动包
   - dry-run 通过后，用户需要输入确认短语。
   - 无退课时确认短语：`我确认准备执行`
   - 有退课时确认短语：`我确认退课风险并准备执行`
   - 生成 `execution-approval.json`。
   - 生成 `run-killcourse.bat`、`execution-runbook.md`、`execution-package.json`。
   - `run-killcourse.bat` 启动前会先暂停一次，KillCourse 自身还会等待一次 Enter。
   - Smart Agent 本身仍不会自动触发真实选退课。

8. release 分发
   - `scripts/build-release.ps1` 已经把 Smart Agent 纳入 release 包。
   - release 包中包含：
     - `HDU-Auto-Scheduling-Script.exe`
     - `smart-agent/HDU-Smart-Course-Agent.exe`
     - `smart-agent/README.md`
     - `smart-agent/SMART_AGENT_QUICKSTART.md`

9. 结构化执行结果解析
   - 已设计 `execution-log.json` schema。
   - 已实现 `HDU-KillCourse/log_files/app.log` 的 post-run 解析 API。
   - 已支持把解析结果写入 `execution-log.json`。
   - 前端已加入“执行结果解析”面板。

当前安全边界：

- Smart Agent 默认不执行真实选课、退课、蹲课。
- 真实执行只会发生在用户手动运行 `run-killcourse.bat`，并在 KillCourse 窗口按 Enter 之后。
- 本项目目前仍属于“计划生成 + 配置生成 + 安全启动包”阶段。

## 预期目录结构

我计划新建一个总文件夹，例如：

```text
HDU 智能选课执行助手/
  HDU-Auto-Scheduling-Script/
  HDU-KillCourse/
  HDU-Smart-Course-Agent/
```

或者英文目录：

```text
HDU-Smart-Course-Agent-Workspace/
  HDU-Auto-Scheduling-Script/
  HDU-KillCourse/
  HDU-Smart-Course-Agent/
```

要求：

- `HDU-Auto-Scheduling-Script` 保持可以独立完整使用。
- `HDU-KillCourse` 也尽量保持可以独立完整使用。
- 新项目 `HDU-Smart-Course-Agent` 负责两者之间的衔接、前端总控、执行计划生成、失败后智能替代。

## 新项目目标

这个项目的核心目标不是单纯“抢课”，而是：

```text
我想要怎样的课表
↓
系统生成候选课表
↓
我选择一个目标方案
↓
系统自动计算要选哪些课、要退哪些课
↓
系统调用选课脚本执行
↓
如果某节课失败，系统自动给出替代方案或进入蹲课
```

换句话说，它应该是一个“排课 + 选课执行 + 失败补救”的智能工作流。

## 推荐架构

建议采用三层架构：

```text
数据层
  course.json
  personal-schedule.json
  candidate-plans.json
  action-plan.json
  execution-log.json

规划层
  候选课表生成
  目标方案选择
  当前课表和目标课表 diff
  选课/退课/蹲课队列生成
  失败后候选方案切换

执行层
  登录
  选课
  退课
  蹲课
  执行结果结构化
```

`HDU-Auto-Scheduling-Script` 偏数据层和规划层。

`HDU-KillCourse` 偏执行层。

`HDU-Smart-Course-Agent` 负责把规划层和执行层连起来。

## 第一阶段不要做太大

第一阶段建议先做：

```text
从排课助手选中的目标方案
生成 HDU-KillCourse 可用的 config.json
同时生成 action-plan.json
```

也就是说，先不要一上来就做全自动智能抢课。

第一阶段的目标应该是：

1. 读取 `course.json`
2. 读取 `personal-schedule.json`
3. 读取或导入一个目标候选课表
4. 对比当前个人课表和目标课表
5. 生成：
   - 要选课程列表
   - 要退课程列表
   - 不可动的锁定课程列表
   - 可替代教学班列表
6. 导出 `action-plan.json`
7. 导出 `config.json`，供 `HDU-KillCourse` 执行

## action-plan.json 建议格式

可以先设计一个中间格式，例如：

```json
{
  "schemaVersion": 1,
  "term": "2026-2027-1",
  "targetPlanId": "plan-001",
  "mode": "safe",
  "current": [
    "(2026-2027-1)-A0001001-01"
  ],
  "target": [
    "(2026-2027-1)-A0001001-02",
    "(2026-2027-1)-A0002001-01"
  ],
  "select": [
    "(2026-2027-1)-A0001001-02",
    "(2026-2027-1)-A0002001-01"
  ],
  "drop": [
    "(2026-2027-1)-A0001001-01"
  ],
  "locked": [
    "(2026-2027-1)-C0000001-01"
  ],
  "fallbackGroups": [
    {
      "courseBase": "(2026-2027-1)-A0001001",
      "courseName": "示例课程",
      "preferred": "(2026-2027-1)-A0001001-02",
      "alternatives": [
        "(2026-2027-1)-A0001001-03",
        "(2026-2027-1)-A0001001-04"
      ]
    }
  ],
  "risks": [
    {
      "level": "high",
      "message": "退课后如果新教学班未选上，可能丢失原课程。"
    }
  ]
}
```

## config.json 生成逻辑

`HDU-KillCourse` 当前需要的配置中，关键字段是：

```json
{
  "course": {
    "(2026-2027-1)-A0001001-02": "1",
    "(2026-2027-1)-A0001001-01": "0"
  }
}
```

其中：

- `"1"` 表示选课
- `"0"` 表示退课

所以新项目应该能根据 `action-plan.json` 自动生成 `HDU-KillCourse/config.json`。

## 执行策略

必须特别注意退课风险。

不能默认简单地“先退再选”，因为可能发生：

```text
退课成功
新课没抢到
原课也没了
```

建议支持几种模式：

1. 保守模式
   - 只自动选课
   - 不自动退课
   - 需要退课时只提示用户确认

2. 安全替换模式
   - 优先尝试选不冲突的课程
   - 对必须退课才能选的课程，要求二次确认
   - 不轻易释放已有课程

3. 激进模式
   - 按完整目标方案自动选退
   - 需要明显风险提示

4. 蹲课模式
   - 对没抢到的课程加入蹲课队列
   - 有余量时再尝试执行
   - 可以和替代方案联动

默认建议是“保守模式”或“安全替换模式”。

## 失败后的智能补救

这是新项目最重要的价值。

如果某节课没抢到，不应该只是显示失败，而应该：

1. 找同课程号的其他教学班
2. 检查是否和当前课表冲突
3. 检查是否违反硬约束
4. 检查是否满足方案级约束，例如教师一致、课程实践联动
5. 如果可行，推荐替代教学班
6. 如果局部替代不可行，切换到下一个候选课表
7. 如果仍不可行，加入蹲课队列

示例流程：

```text
目标：选 A0512040-02
结果：人数已满
系统检查：
  A0512040-03 是否可选？
  A0512040-04 是否可选？
  替换后是否时间冲突？
  替换后是否还能和实验课教师一致？
如果可以：
  推荐替代
如果不可以：
  切换候选方案 2
如果候选方案都不行：
  加入蹲课
```

## 前端建议

新项目应该有比 `HDU-KillCourse` 更完整的前端界面。

建议页面模块：

1. 数据状态
   - 是否存在 `course.json`
   - 是否存在 `personal-schedule.json`
   - 当前学期
   - 课程数量
   - 个人课表数量

2. 目标方案
   - 展示当前个人课表
   - 展示目标课表
   - 展示差异
   - 展示需要选/退的课程

3. 执行计划
   - 选课队列
   - 退课队列
   - 蹲课队列
   - 风险提示
   - 执行模式选择

4. 登录配置
   - 学号
   - 密码
   - 钉钉扫码
   - cookie 状态
   - 学年学期

5. 执行控制台
   - 开始执行
   - 暂停/停止
   - 当前状态
   - 结构化日志
   - 每门课执行结果

6. 失败处理
   - 显示失败原因
   - 推荐替代教学班
   - 推荐切换候选方案
   - 加入蹲课
   - 手动处理

## 后端建议

新项目仍然建议使用 Go。

原因：

- 两个已有项目都是 Go，复用成本低。
- 最终方便打包成 exe。
- 本地浏览器前端 + Go 后端的模式已经在排课助手里跑通。

建议新项目先做成：

```text
Go 本地服务
HTML/CSS/JS 前端
本地 JSON 文件存储
最终打包为 exe
```

第一版不需要数据库。

## 重要原则

1. 不要破坏旧项目独立性。
2. 不要一开始就深度改 `HDU-KillCourse`。
3. 第一阶段先做文件级衔接：
   - 读取排课数据
   - 生成执行计划
   - 生成 KillCourse 配置
4. 第二阶段再考虑把 `HDU-KillCourse` 的执行能力抽成可调用 API。
5. 退课必须谨慎，默认不要自动激进退课。
6. 执行结果必须结构化，不能只靠日志。
7. 失败后智能补救是这个新项目的核心竞争力。

## 当前里程碑状态

### M1：新项目脚手架（已完成）

- 已新建 `HDU-Smart-Course-Agent`。
- 已实现 Go 本地服务和前端页面。
- 已实现两个子项目目录检测。
- 已实现 `course.json` 和 `personal-schedule.json` 检测。
- 已支持手动保存路径设置。

### M2：方案导入与 diff（已完成）

- 已支持导入目标课表 JSON。
- 已展示当前课表、目标课表、选课/退课/保留列表。
- 已计算 `select`、`drop`、`keep`、`locked`。
- 已生成 `action-plan.json`。
- 已输出风险、校验结果和计划解释。

### M3：生成 KillCourse 配置（已完成）

- 已根据执行计划生成 `HDU-KillCourse/config.json`。
- 已保留 KillCourse 原有非课程配置。
- 已支持 config 变更预览。
- 当前默认策略是完整执行计划：`select` 写 `"1"`，`drop` 写 `"0"`。
- 退课风险会在 UI、计划和 readiness 中提示。

### M4：调用 KillCourse（部分完成，仍保持安全边界）

- 已实现 dry-run。
- 已实现执行授权票据。
- 已实现 `run-killcourse.bat` 启动包。
- 已实现 `execution-runbook.md` 和 `execution-package.json`。
- 目前不会由 Smart Agent 自动运行 KillCourse。
- 当前真实执行路径是：用户手动运行 bat，然后在 KillCourse 窗口手动按 Enter。

### M5：结构化执行结果（部分完成）

- 已实现 post-run 日志解析。
- 已能生成 `execution-log.json`。
- 已能识别成功、失败、人数满、登录过期、课程不存在、配置错误、网络错误等基础类型。
- 尚未实时读取 KillCourse 执行日志。
- 尚未直接从 KillCourse 内部返回结构化事件。

### M6：失败后替代方案（未完成）

- 已经有 `fallbackGroups` 的静态备选教学班信息。
- 尚未根据真实执行失败结果自动推荐替代教学班。
- 尚未自动切换候选课表。
- 尚未和蹲课队列联动。

## 给 Codex 的工作方式要求

请先阅读两个旧项目的代码结构，尤其是：

```text
HDU-Auto-Scheduling-Script/
  README.md
  shared.js
  scheduler.js
  scheduler-worker.js
  main.go
  docs/COURSE_SCHEMA.md

HDU-KillCourse/
  README.md
  config.example.json
  config/config.go
  pkg/course/killCourse.go
  pkg/course/waitCourse.go
  pkg/login/login.go
  cmd/HDU-KillCourse/main.go
```

然后再开始设计新项目。

不要直接修改旧项目，除非明确需要。

优先在新项目里做桥接层和编排层。

如果必须改旧项目，应该尽量做成小改动，并说明原因。

## 当前最推荐你先做的事情

下一步不要直接做全自动执行，建议优先做“失败后替代方案推荐”：

1. 读取 `execution-log.json`。
2. 找出 `status = failed` 的课程动作。
3. 对 `failureType = full` 或 `not-found` 的选课失败，查询 `fallbackGroups`。
4. 用排课助手已有的时间冲突和约束判断逻辑，筛选可替代教学班。
5. 输出 `fallback-recommendations.json`。
6. 前端展示替代建议：同课程号教学班、时间冲突情况、教师差异、是否影响联动课程。
7. 第一版只推荐，不自动写入新 config。

只有当替代建议可靠后，才适合继续做半自动执行控制台或真正的智能补救。
