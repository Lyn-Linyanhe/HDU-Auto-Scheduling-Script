package school

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
)

var classCodeRe = regexp.MustCompile(`\(\d{4}-\d{4}-\d\)-[A-Za-z0-9]+-\d{1,3}`)

type xlsxCell struct {
	Ref  string `xml:"r,attr"`
	Type string `xml:"t,attr"`
	V    string `xml:"v"`
	IS   struct {
		T []string `xml:"t"`
		R []struct {
			T string `xml:"t"`
		} `xml:"r"`
	} `xml:"is"`
}

type xlsxRow struct {
	Cells []xlsxCell `xml:"c"`
}

type xlsxSheetData struct {
	Rows []xlsxRow `xml:"sheetData>row"`
}

type xlsxSharedStrings struct {
	Items []struct {
		T []string `xml:"t"`
		R []struct {
			T string `xml:"t"`
		} `xml:"r"`
	} `xml:"si"`
}

func EnsureCourseFile(jsonName string) (*CoursePayload, string, error) {
	if payload, err := ReadCourseFile(jsonName); err == nil {
		return payload, "json", nil
	}

	xlsxName, err := findCourseExcel(".")
	if err != nil {
		return nil, "", err
	}
	payload, err := ReadCourseExcel(xlsxName)
	if err != nil {
		return nil, "", err
	}
	if len(payload.Items) == 0 {
		return nil, "", errors.New("Excel 中没有识别到课程数据")
	}

	data, err := json.MarshalIndent(payload, "", "  ")
	if err != nil {
		return nil, "", err
	}
	if err := os.WriteFile(jsonName, data, 0644); err != nil {
		return nil, "", err
	}
	return payload, filepath.Base(xlsxName), nil
}

func ReadCourseExcel(name string) (*CoursePayload, error) {
	reader, err := zip.OpenReader(name)
	if err != nil {
		return nil, err
	}
	defer reader.Close()

	shared, err := readSharedStrings(&reader.Reader)
	if err != nil {
		return nil, err
	}

	var best []map[string]any
	var bestName string
	for _, file := range reader.File {
		if !strings.HasPrefix(file.Name, "xl/worksheets/") || !strings.HasSuffix(file.Name, ".xml") {
			continue
		}
		rows, err := readSheetRows(file, shared)
		if err != nil {
			continue
		}
		items := rowsToCourses(rows)
		if len(items) > len(best) {
			best = items
			bestName = file.Name
		}
	}
	if len(best) == 0 {
		return nil, fmt.Errorf("未能从 %s 识别课程信息表", filepath.Base(name))
	}
	_ = bestName
	return &CoursePayload{SchemaVersion: CourseSchemaVersion, Items: best}, nil
}

func findCourseExcel(dir string) (string, error) {
	matches, _ := filepath.Glob(filepath.Join(dir, "*任务落实情况课程导出.xlsx"))
	if len(matches) == 0 {
		matches, _ = filepath.Glob(filepath.Join(dir, "*.xlsx"))
	}
	if len(matches) == 0 {
		return "", errors.New("当前目录没有可用的 course.json，也没有找到课程导出 Excel")
	}

	var newest string
	var newestTime int64
	for _, match := range matches {
		info, err := os.Stat(match)
		if err != nil || info.IsDir() {
			continue
		}
		if newest == "" || info.ModTime().UnixNano() > newestTime {
			newest = match
			newestTime = info.ModTime().UnixNano()
		}
	}
	if newest == "" {
		return "", errors.New("没有找到可读取的课程导出 Excel")
	}
	return newest, nil
}

func readSharedStrings(reader *zip.Reader) ([]string, error) {
	file := zipEntry(reader, "xl/sharedStrings.xml")
	if file == nil {
		return nil, nil
	}
	data, err := readZipFile(file)
	if err != nil {
		return nil, err
	}
	var parsed xlsxSharedStrings
	if err := xml.Unmarshal(data, &parsed); err != nil {
		return nil, err
	}
	out := make([]string, 0, len(parsed.Items))
	for _, item := range parsed.Items {
		out = append(out, strings.TrimSpace(strings.Join(richText(item.T, item.R), "")))
	}
	return out, nil
}

func readSheetRows(file *zip.File, shared []string) ([][]string, error) {
	data, err := readZipFile(file)
	if err != nil {
		return nil, err
	}
	var parsed xlsxSheetData
	if err := xml.Unmarshal(data, &parsed); err != nil {
		return nil, err
	}

	rows := make([][]string, 0, len(parsed.Rows))
	for _, row := range parsed.Rows {
		values := []string{}
		for _, cell := range row.Cells {
			col := cellColumn(cell.Ref)
			if col <= 0 {
				col = len(values) + 1
			}
			for len(values) < col {
				values = append(values, "")
			}
			values[col-1] = cellValue(cell, shared)
		}
		if hasAnyValue(values) {
			rows = append(rows, values)
		}
	}
	return rows, nil
}

func rowsToCourses(rows [][]string) []map[string]any {
	for rowIndex, row := range rows {
		header := headerMap(row)
		if !hasCourseHeaders(header) {
			continue
		}
		items := make([]map[string]any, 0, len(rows)-rowIndex-1)
		for _, dataRow := range rows[rowIndex+1:] {
			item := map[string]any{}
			for index, value := range dataRow {
				value = strings.TrimSpace(value)
				if value == "" {
					continue
				}
				if key := header[index]; key != "" {
					item[key] = value
				}
			}
			if normalizeExcelCourse(item) {
				items = append(items, item)
			}
		}
		return items
	}
	return nil
}

func normalizeExcelCourse(item map[string]any) bool {
	jxbmc := textValue(item["jxbmc"])
	kcmc := textValue(item["kcmc"])
	sksj := textValue(item["sksj"])
	if jxbmc == "" && kcmc == "" {
		return false
	}

	if code := classCodeRe.FindString(jxbmc); code != "" {
		item["displayCode"] = code
		item["sectionName"] = code
		if textValue(item["kch_id"]) == "" || looksLikeInternalID(textValue(item["kch_id"])) {
			item["courseCode"] = strings.TrimRight(regexp.MustCompile(`-\d{1,3}$`).ReplaceAllString(code, ""), "-")
		}
	}
	if textValue(item["courseCode"]) == "" {
		item["courseCode"] = textValue(item["kch_id"])
	}
	if textValue(item["sectionId"]) == "" && jxbmc != "" {
		item["sectionId"] = jxbmc
	}
	if textValue(item["jxb_id"]) == "" && jxbmc != "" {
		item["jxb_id"] = jxbmc
	}
	if sksj == "" {
		item["sksj"] = textValue(item["time"])
	}
	return true
}

func headerMap(row []string) map[int]string {
	out := map[int]string{}
	for index, value := range row {
		switch strings.TrimSpace(value) {
		case "教学班名称":
			out[index] = "jxbmc"
		case "课程号":
			out[index] = "kch_id"
		case "课程名称":
			out[index] = "kcmc"
		case "是否开课":
			out[index] = "kkztmc"
		case "是否排课":
			out[index] = "bpkbj"
		case "选课标记":
			out[index] = "xkbjmc"
		case "上课时间":
			out[index] = "sksj"
		case "上课地点":
			out[index] = "jxdd"
		case "场地名称":
			out[index] = "cdlbmc"
		case "场地具体名称":
			out[index] = "cdejlbmc"
		case "教职工信息":
			out[index] = "jzgxx"
		case "教师出生日期":
			out[index] = "jscsrq"
		case "教师性别":
			out[index] = "jsxbdm"
		case "开课部门":
			out[index] = "kkbmmc"
		case "学分":
			out[index] = "xf"
		case "授课学院":
			out[index] = "skxy"
		case "教学班容量":
			out[index] = "jxbrl"
		case "教学班人数":
			out[index] = "jxbrs"
		case "选课人数":
			out[index] = "xkrs"
		case "面向对象":
			out[index] = "mxdx"
		case "授课班级":
			out[index] = "skbj"
		case "授课详情":
			out[index] = "skxq"
		case "开课类型":
			out[index] = "kklxmc"
		case "课程归属":
			out[index] = "kcgs"
		case "讲课学时":
			out[index] = "jkxs"
		case "考核方式":
			out[index] = "khfsmc"
		case "学科备注":
			out[index] = "xkbz"
		}
	}
	return out
}

func hasCourseHeaders(header map[int]string) bool {
	seen := map[string]bool{}
	for _, key := range header {
		seen[key] = true
	}
	return seen["jxbmc"] && seen["kcmc"]
}

func cellValue(cell xlsxCell, shared []string) string {
	switch cell.Type {
	case "s":
		index, err := strconv.Atoi(strings.TrimSpace(cell.V))
		if err == nil && index >= 0 && index < len(shared) {
			return shared[index]
		}
	case "inlineStr":
		return strings.Join(richText(cell.IS.T, cell.IS.R), "")
	}
	return strings.TrimSpace(cell.V)
}

func richText(texts []string, runs []struct {
	T string `xml:"t"`
}) []string {
	parts := append([]string{}, texts...)
	for _, run := range runs {
		parts = append(parts, run.T)
	}
	return parts
}

func cellColumn(ref string) int {
	total := 0
	for _, ch := range ref {
		if ch < 'A' || ch > 'Z' {
			break
		}
		total = total*26 + int(ch-'A'+1)
	}
	return total
}

func zipEntry(reader *zip.Reader, name string) *zip.File {
	for _, file := range reader.File {
		if file.Name == name {
			return file
		}
	}
	return nil
}

func readZipFile(file *zip.File) ([]byte, error) {
	closer, err := file.Open()
	if err != nil {
		return nil, err
	}
	defer closer.Close()
	var buf bytes.Buffer
	if _, err := io.Copy(&buf, closer); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func hasAnyValue(values []string) bool {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return true
		}
	}
	return false
}

func textValue(value any) string {
	if value == nil {
		return ""
	}
	return strings.TrimSpace(fmt.Sprint(value))
}

func looksLikeInternalID(value string) bool {
	if len(value) < 20 {
		return false
	}
	for _, ch := range value {
		if !((ch >= '0' && ch <= '9') || (ch >= 'A' && ch <= 'F') || (ch >= 'a' && ch <= 'f')) {
			return false
		}
	}
	return true
}
