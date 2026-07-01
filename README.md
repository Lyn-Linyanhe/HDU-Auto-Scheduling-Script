# HDU Auto Scheduling Script

HDU 课表自动化编排助手，包含两个本地工具：

- `hdu-offline-scheduler`：离线模拟排课与候选课表生成。
- `hdu-course-exporter`：课程数据导出器，用于生成 `course.json`。

## 功能

- 本地读取 `course.json` 或从 Excel 自动转换课程数据。
- 支持导入个人/班级课表作为已选底板。
- 支持课程搜索、加入、移除、锁定和冲突提示。
- 支持候选课表生成、翻页、收藏、删除和预览。
- 支持课程级、时间级、方案级约束。
- 支持导出当前课表 JSON 和课表截图。

## 本地运行

```bash
go run .
```

浏览器会自动打开离线排课助手。

## 构建

```bash
go build -buildvcs=false -o dist/hdu-offline-scheduler.exe .
go build -buildvcs=false -o dist/hdu-course-exporter.exe ./cmd/course-exporter
```

## 数据文件

`course.json`、Excel 文件和 `dist/` 下的 exe/数据默认不提交到 Git。需要分发时可以单独打包 release。
