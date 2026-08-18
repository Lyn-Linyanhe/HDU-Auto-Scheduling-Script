# HDU-Smart-Course-Agent 快速开始

这个工具用于把排课助手导出的目标课表转换成 `HDU-KillCourse/config.json` 和执行启动包。它默认不会真实执行选课或退课。

## 目录准备

推荐目录结构：

```text
HDU-Smart-Course-Agent-Workspace/
  HDU-Auto-Scheduling-Script/
  HDU-KillCourse-main/
  HDU-Smart-Course-Agent/
```

如果自动检测不到目录，可以打开页面右上角“设置”后手动填写：

- 排课助手目录：包含 `course.json` 的目录。
- HDU-KillCourse 目录：包含 `config.example.json` 或 `cmd/HDU-KillCourse/main.go` 的目录。

## 使用流程

页面按四个步骤推进，不需要一次看完所有信息。

### 第一步：确认数据

1. 先启动主站并完成一次教务登录，保持主站运行。Smart Agent 默认通过 `http://127.0.0.1:6789` 复用主站内存中的登录会话。
2. 运行 `HDU-Smart-Course-Agent.exe`。如果工作目录或排课助手目录存在 `target-schedule.json`、`hdu-target-timetable.json` 或 `hdu-target-timetable-*.json`，页面会自动导入其中最新的有效目标课表，并显示来源和文件更新时间。`hdu-current-timetable*.json` 是当前课表导出，不会被当作目标课表自动导入。
3. 如果没有自动发现目标课表，可以使用页面中的文件选择器手动导入；手动导入会覆盖本次页面会话中的自动目标。
4. 当前个人课表会显示已有快照。点击“立即刷新”可通过主站实时获取个人课表；也可以打开“自动刷新”并设置 10–7200 秒的间隔，默认间隔为 60 秒。
5. 如果计划包含退课，确认页面显示的上次成功刷新时间；刷新失败时不会清空旧快照，页面会同时显示失败时间和原因。

### 第二步：确认计划

1. 生成执行计划。
2. 查看需要选课、需要退课和高风险提示。
3. 计划细节、保留课程、备选教学班和配置差异可按需展开查看。

### 第三步：执行准备

1. 勾选“确认写入 KillCourse/config.json”。
2. 点击“更新配置并运行安全检查”。这一步不会真实选课或退课。
3. 按页面提示输入确认短语，生成 `execution-approval.json`。
4. 点击“生成启动包”，得到：
   - `run-killcourse.bat`
   - `execution-runbook.md`
   - `execution-package.json`
5. 手动运行 `run-killcourse.bat`。

启动包会明确记录本次使用的绝对工作目录、`config.json`、启动入口和 `log_files/app.log`，解析执行结果时会从生成启动包时记录的日志偏移开始，避免混入历史运行记录。

### 第四步：结果与替代方案

1. KillCourse 完成后回到 Smart Agent，解析执行日志。
2. 若有未选上的课程，生成替代方案，查看同课程号教学班是否仍满足时间和教师偏好。
3. 替代方案只供你确认，不会自动写入或执行新的选课计划。

## 安全边界

- Smart Agent 不会自动运行 KillCourse。
- `run-killcourse.bat` 启动前会先暂停一次，防止误触。
- KillCourse 自身还会等待一次 Enter；只有你手动按 Enter 后，才会进入真实执行。
- 包含退课动作时，请再次确认风险：如果新教学班未成功选上，原课程可能已经被退掉。
- Smart Agent 不保存教务账号、密码或浏览器 Cookie。主站重启后，需重新完成登录才能刷新个人课表。
- 重写配置时会保留最新 `HDU-KillCourse-main` 支持的 `user_agent`、`ClientBodyConfigEnabled` 和 `CrossGradeEnabled` 字段，只替换本次计划的 `course` 和学年学期。
- 当前适配的 KillCourse 版本为 `v1.4.9`；Smart Agent 使用本机单独配置的 `HDU-KillCourse-main` 目录，不会把两个项目源码合并。
- 手动运行 KillCourse 并解析日志后，若检测到选课或退课成功，Smart Agent 会立即刷新个人课表；刷新失败仍保留上一次成功快照。
