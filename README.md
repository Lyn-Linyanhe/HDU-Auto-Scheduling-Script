# HDU 课表自动化编排助手

这是一个面向杭电选课场景的本地工具。普通用户只需要运行一个 exe：

- `HDU-Auto-Scheduling-Script.exe`：启动后会先检查当前目录的 `course.json`。全校课表可用时直接进入离线排课页；缺少全校课表时进入课程导出页。个人课表 `personal-schedule.json` 可作为已选底板自动导入，缺失时仍可先排课，并可从页面返回导出页补导。

## 功能

- 本地读取 `course.json`，缺失时进入导出器；`course.json` 也可从任务落实 Excel 自动转换。
- 导出器会顺带下载个人课表；排课助手检测到 `personal-schedule.json` 后会自动作为已选底板导入并锁定。
- 支持手动导入个人/班级课表作为已选底板。
- 支持课程搜索、加入、移除、锁定、冲突提示和单双周展示。
- 支持课程级、时间级、方案级约束。
- 支持候选课表估算、生成、翻页预览、收藏、删除和导入显示；候选生成默认放到 Web Worker 中执行，减少页面卡顿。
- 课程数据统一归一化为 schema v1，说明见 [docs/COURSE_SCHEMA.md](docs/COURSE_SCHEMA.md)。
- 支持导出当前课表 JSON 和课表截图。

## 本地运行

运行统一助手：

```bash
go run .
```

开发调试时仍可单独运行旧导出器入口：

```bash
go run ./cmd/course-exporter
```

程序会自动打开本地浏览器页面。

## 打包 exe

直接构建统一 exe：

```bash
go build -buildvcs=false -o dist/HDU-Auto-Scheduling-Script.exe .
```

生成可分发 release 包：

```powershell
powershell -ExecutionPolicy Bypass -File scripts/build-release.ps1
```

脚本会生成 `release/HDU-Auto-Scheduling-Script-v版本号/` 和对应 zip，包含统一 exe、使用说明、课程 schema、版本文件、manifest 和测试样例数据。

## 测试数据

固定测试数据位于 `testdata/`：

- `course.sample.json`
- `personal-schedule.sample.json`

说明见 [docs/TEST_DATA.md](docs/TEST_DATA.md)。

排课 Worker 冒烟测试：

```bash
node scripts/scheduler-worker-smoke.js
```

课表截图导出冒烟测试需要先启动排课助手服务：

```bash
set HDU_NO_BROWSER=1
go run .
node scripts/screenshot-smoke.js
```

## 数据文件

`course.json`、`personal-schedule.json`、Excel 文件、`dist/` 和 `release/` 下的 exe/数据默认不提交到 Git。需要分发时建议使用 release 脚本生成 zip。
