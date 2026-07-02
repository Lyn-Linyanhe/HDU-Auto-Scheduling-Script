# 课程数据标准格式

排课助手内部统一使用 `HDU.COURSE_SCHEMA_VERSION = 1`。无论课程来自新教务接口、Excel、个人课表 JSON，还是用户导出的当前课表，都先归一化为同一结构再参与展示、冲突判断和候选生成。

## CanonicalCourse

```json
{
  "schemaVersion": 1,
  "id": "sample-data-01",
  "groupId": "(2026-2027-1)-A0003001",
  "displayCode": "(2026-2027-1)-A0003001-01",
  "rawCourseCode": "(2026-2027-1)-A0003001",
  "courseName": "数据结构",
  "sectionName": "(2026-2027-1)-A0003001-01",
  "teacher": "陈老师",
  "kind": "专业基础",
  "location": "第6教研楼303",
  "status": "已开课",
  "credits": 3.5,
  "capacity": 80,
  "enrolled": 52,
  "selected": 52,
  "timeText": "星期二第6-7节{1-17周}",
  "meetings": [
    {
      "day": 2,
      "start": 6,
      "end": 7,
      "weeks": [1, 2, 3, 4],
      "raw": "星期二第6-7节{1-17周}"
    }
  ],
  "raw": {}
}
```

## 字段含义

- `id`：教学班唯一 ID，优先使用 `jxb_id`。
- `groupId`：课程组 ID，同一课程号不同教学班共享同一个 `groupId`。
- `displayCode`：完整教学班号，通常形如 `(2026-2027-1)-A0003001-01`。
- `rawCourseCode`：原始课程号，可能来自 `kch_id` 或 `courseCode`。
- `courseName`：课程名称。
- `sectionName`：教学班名称。
- `teacher`：教师名称。
- `credits`：数字学分，支持 `0.25`。
- `timeText`：原始时间文本。
- `meetings`：结构化时间段，供冲突检测使用。
- `raw`：原始数据，便于导出和追踪。

## 为什么需要 schema

学校接口、Excel 和个人课表字段并不统一。没有标准格式时，排课逻辑会到处判断 `kcmc/courseName/name`、`jxbmc/sectionName`、`sksj/time/schedule`，后期很容易改坏。统一 schema 后，数据兼容集中在 `normalizeCourseData`，其他模块只处理标准字段。
