# HDU 课表自动化编排助手

这是一个面向杭电选课场景的本地工具集合，当前拆成两个独立可打包程序：

- `hdu-course-exporter`：课程数据导出器，负责联网登录新教务，并生成 `course.json` 与 `personal-schedule.json`。
- `hdu-offline-scheduler`：离线排课助手，读取全校课表后在浏览器里完成模拟排课、约束筛选和候选方案浏览。

## 功能

- 本地读取 `course.json`，缺失时可从任务落实 Excel 自动转换。
- 导出器会顺带下载个人课表；排课助手检测到 `personal-schedule.json` 后会自动作为已选底板导入并锁定。
- 支持手动导入个人/班级课表作为已选底板。
- 支持课程搜索、加入、移除、锁定、冲突提示和单双周展示。
- 支持课程级、时间级、方案级约束。
- 支持候选课表估算、生成、翻页预览、收藏、删除和导入显示。
- 支持导出当前课表 JSON 和课表截图。

## 本地运行

运行离线排课助手：

```bash
go run .
```

运行课程数据导出器：

```bash
go run ./cmd/course-exporter
```

两个程序都会自动打开本地浏览器页面。

## 打包

```bash
go build -buildvcs=false -o dist/hdu-offline-scheduler.exe .
go build -buildvcs=false -o dist/hdu-course-exporter.exe ./cmd/course-exporter
```

## 数据文件

`course.json`、`personal-schedule.json`、Excel 文件和 `dist/` 下的 exe/数据默认不提交到 Git。需要分发时建议单独打包 release。
