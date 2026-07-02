# 固定测试数据说明

`testdata` 目录里的数据不包含真实账号和真实课表，只用于本地功能验证。

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

## 手动测试步骤

1. 复制 `testdata/course.sample.json` 到项目根目录并改名为 `course.json`。
2. 复制 `testdata/personal-schedule.sample.json` 到项目根目录并改名为 `personal-schedule.json`。
3. 运行：

```powershell
go run .
```

4. 打开排课页，确认个人课表底板已自动导入并锁定。
5. 搜索 `编译原理`，确认自动识别的关联课程中包含课程实践。
6. 生成候选课表，确认单双周课程可以同时间段展示且不算冲突。

测试结束后可以删除根目录下的 `course.json` 和 `personal-schedule.json`。
