# HDU 课表自动化编排助手使用说明

## 文件说明

- `hdu-course-exporter.exe`：联网导出全校课程和个人课表。
- `hdu-offline-scheduler.exe`：离线模拟排课。
- `samples/course.sample.json`：测试用全校课程数据。
- `samples/personal-schedule.sample.json`：测试用个人课表底板。

## 推荐使用流程

1. 把两个 exe 放在同一个文件夹。
2. 运行 `hdu-course-exporter.exe`。
3. 在自动打开的浏览器页面中填写学号、密码、学年和学期。
4. 导出成功后，当前文件夹会生成：
   - `course.json`
   - `personal-schedule.json`
5. 关闭导出器窗口，运行 `hdu-offline-scheduler.exe`。
6. 排课助手会自动读取 `course.json`；如果发现 `personal-schedule.json`，会把个人课表作为已选底板导入并锁定。

## 无法联网导出时

如果学校接口暂时不可用，可以先用 Excel 或 JSON 测试：

- 把任务落实课程导出的 Excel 放到 `hdu-offline-scheduler.exe` 同目录，启动排课助手后会尝试自动转换。
- 或把 `samples/course.sample.json` 改名为 `course.json`，把 `samples/personal-schedule.sample.json` 改名为 `personal-schedule.json`，然后启动排课助手测试功能。

## 常见问题

### 浏览器没有自动打开

复制窗口里显示的本地地址，在浏览器中手动打开：

- 排课助手：`http://127.0.0.1:6789`
- 导出器：`http://127.0.0.1:6790`

### 提示端口被占用

先关闭已经打开的旧窗口。如果仍然失败，打开任务管理器结束旧的 `hdu-offline-scheduler.exe` 或 `hdu-course-exporter.exe`。

### 提示账号或密码错误

先确认学校统一身份认证能正常网页登录。如果网页能登录但导出器失败，可能是学校登录页或风控策略发生变化，需要更新导出器。

### 个人课表没有自动导入

确认 `personal-schedule.json` 和 `hdu-offline-scheduler.exe` 在同一个目录。若已经手动导入过底板，程序不会覆盖你的本地选择。

### 生成候选方案太少

检查是否锁定了过多课程、学分范围过窄、必选课程过多，或者方案级约束要求过强。

### 生成候选方案太多

增加约束条件，例如必选课程、教师偏好、最多早八天数、最多晚课天数、最少全天无课天数。
