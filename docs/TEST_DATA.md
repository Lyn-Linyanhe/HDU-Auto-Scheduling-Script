# 固定测试数据说明

固定测试数据不包含真实账号和真实课表，只用于本地功能验证。

- 源码仓库中位于 `testdata/`。
- release 包中位于 `samples/`。

## 数据文件

- `course.sample.json`：模拟全校任务落实课程。
- `personal-schedule.sample.json`：模拟个人课表底板。

## 覆盖场景

- 同一课程号不同教学班：`高等数学A`、`数据结构`。
- 早八课程：`高等数学A-01`。
- 晚课课程：`创新创业基础`。
- 单双周同一时间段：`大学物理` 和 `形势与政策`。
- 小数学分：`形势与政策` 为 `0.25` 学分。
- 方案级联动示例：`编译原理` 与 `编译原理课程实践`，教师均为 `周老师`。
- 个人课表底板：默认包含 `高等数学A-01` 和 `大学英语`。

自动关联识别目前采用保守的名称规则：课程实践、开发实践、综合实践、课程设计、实验、实践、实训等明确的末尾后缀会与去除后缀的主课程名匹配；末尾的数字、中文序号和 `（甲）`/`（乙）` 等教学班限定词会先归一化。普通的裸 `设计` 不作为关联后缀，避免把“角色与场景设计”之类的普通课程误配。课程名称差异超过这些规则时，应使用方案级自定义约束；不能据此声称覆盖学校所有历史命名。

`scripts/scheduler-worker-smoke.js` 还覆盖 `开发实践`、带 `2（乙）` 限定词的课程实践，以及裸“设计”负例，确保旧状态迁移只在唯一教学班匹配时锁定，并让排课页自动关联继续共用同一套名称规则。

## 手动测试步骤

1. 复制 `course.sample.json` 到项目根目录并改名为 `course.json`。
   - 源码仓库路径：`testdata/course.sample.json`
   - release 包路径：`samples/course.sample.json`
2. 复制 `personal-schedule.sample.json` 到项目根目录并改名为 `personal-schedule.json`。
   - 源码仓库路径：`testdata/personal-schedule.sample.json`
   - release 包路径：`samples/personal-schedule.sample.json`
3. 在源码仓库中运行：

```powershell
go run .
```

   在 release 包中则直接双击或运行：

```powershell
.\HDU-Auto-Scheduling-Script.exe
```

4. 打开排课页，确认个人课表底板已自动导入并锁定。
5. 搜索 `编译原理`，确认自动识别的关联课程中包含课程实践。
6. 生成候选课表，确认单双周课程可以同时间段展示且不算冲突。

测试结束后可以删除根目录下的 `course.json` 和 `personal-schedule.json`。

## 本地模拟教务系统验收

当学校教务系统不可用时，可以运行下面的完整离线验收。它启动仅监听
`127.0.0.1` 的模拟服务，使用虚构的 `test-user` / `test-password`，不会读取
真实账号、密码、Cookie，也不会连接杭电服务器。

```powershell
powershell -ExecutionPolicy Bypass -File scripts\testlab-acceptance.ps1
```

该脚本验证模拟登录、全校课表导出、个人课表导出，以及账号错误、权限
不足、接口损坏、空数据、超时和个人课表单独失败等场景。成功导出的文件会
继续送入排课 Worker、Smart Agent 端到端测试和 Smart Agent UI/API 冒烟测试。

开发时如需保留每个场景的临时输出和诊断文件，可加 `-KeepTemp`。临时目录
为 `tmp-testlab-acceptance/`，默认会在脚本结束时删除。
