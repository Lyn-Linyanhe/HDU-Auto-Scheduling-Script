# HDU 课表自动化编排助手使用说明

## 文件说明

- `HDU-Auto-Scheduling-Script.exe`：统一启动入口，自动在课程导出页和离线排课页之间切换。
- `samples/course.sample.json`：测试用全校课程数据。
- `samples/personal-schedule.sample.json`：测试用个人课表底板。
- `COURSE_SCHEMA.md`：课程 JSON 的统一字段说明。

## 推荐使用流程

1. 运行 `HDU-Auto-Scheduling-Script.exe`。
2. 程序会检查当前文件夹是否已有 `course.json`。
3. 如果全校课表可用，会直接进入 `HDU课表自动化编排助手`；如果同时有 `personal-schedule.json`，会自动作为已选底板导入并锁定。
4. 如果缺少全校课表，会进入课程导出页；在自动打开的浏览器页面中填写学号、密码、学年和学期。
5. 导出成功后，当前文件夹会生成：
   - `course.json`
   - `personal-schedule.json`
6. 导出完成后会自动跳转到排课助手，并把个人课表作为已选底板导入并锁定。若只想补导个人课表，可在排课页点击“导出/更新课程”回到导出页。

## 无法联网导出时

如果学校接口暂时不可用，可以先用 Excel 或 JSON 测试：

- 把任务落实课程导出的 Excel 放到 `HDU-Auto-Scheduling-Script.exe` 同目录，启动后会尝试自动转换 `course.json`。
- 或把 `samples/course.sample.json` 改名为 `course.json`，把 `samples/personal-schedule.sample.json` 改名为 `personal-schedule.json`，然后启动排课助手测试功能。

## 常见问题

### 浏览器没有自动打开

复制窗口里显示的本地地址，在浏览器中手动打开：

- 统一入口：`http://127.0.0.1:6789`
- 导出页：`http://127.0.0.1:6789/exporter/`
- 排课页：`http://127.0.0.1:6789/scheduler.html`

### 提示端口被占用

先关闭已经打开的旧窗口。如果仍然失败，打开任务管理器结束旧的 `HDU-Auto-Scheduling-Script.exe`。

### 提示账号或密码错误

先确认学校统一身份认证能正常网页登录。如果网页能登录但导出器失败，可能是学校登录页或风控策略发生变化，需要更新导出器。

### 个人课表没有自动导入

确认 `personal-schedule.json` 和 `HDU-Auto-Scheduling-Script.exe` 在同一个目录。若已经手动导入过底板，程序不会覆盖你的本地选择。

### 生成候选方案太少

检查页面里的“没有候选方案”诊断说明。常见原因包括锁定课冲突、学分范围过窄、时间级约束过紧、必选课程未匹配，或者方案级约束要求过强。

### 生成候选方案太多

增加约束条件，例如必选课程、教师偏好、最多早八天数、最多晚课天数、最少全天无课天数。

### 生成候选方案时页面是否会卡住

新版会优先把候选估算和生成放到浏览器 Web Worker 中运行。Worker 不可用时会自动退回同步生成，结果一致，只是页面可能短暂变慢。
