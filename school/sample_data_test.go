package school

import (
	"archive/zip"
	"encoding/json"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

func TestSampleCourseDataCanDecode(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("..", "testdata", "course.sample.json"))
	if err != nil {
		t.Fatalf("read sample course data: %v", err)
	}
	payload, err := DecodeCoursePayload(data)
	if err != nil {
		t.Fatalf("decode sample course data: %v", err)
	}
	if len(payload.Items) < 8 {
		t.Fatalf("sample course count = %d, want at least 8", len(payload.Items))
	}
}

func TestSamplePersonalScheduleCanDecode(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("..", "testdata", "personal-schedule.sample.json"))
	if err != nil {
		t.Fatalf("read sample personal schedule: %v", err)
	}
	payload, err := DecodeCoursePayload(data)
	if err != nil {
		t.Fatalf("decode sample personal schedule: %v", err)
	}
	if len(payload.Items) != 2 {
		t.Fatalf("sample personal schedule count = %d, want 2", len(payload.Items))
	}
}

func TestDistCourseWorkbookPreservesRealCourseFields(t *testing.T) {
	path := filepath.Join("..", "dist", "2026-2027_1_任务落实情况课程导出.xlsx")
	if _, err := os.Stat(path); err != nil {
		t.Skipf("real dist workbook is unavailable: %v", err)
	}
	payload, err := ReadCourseExcel(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(payload.Items) != 5155 {
		t.Fatalf("real dist workbook course rows = %d, want 5155", len(payload.Items))
	}

	var target map[string]any
	quarterCredits := 0
	period10 := 0
	period13 := 0
	oddEven := 0
	for _, item := range payload.Items {
		if strings.Contains(textValue(item["jxbmc"]), "A0512150-04") || strings.Contains(textValue(item["displayCode"]), "A0512150-04") {
			target = item
		}
		credit, _ := strconv.ParseFloat(textValue(item["xf"]), 64)
		if credit == 0.25 {
			quarterCredits++
		}
		timeText := textValue(item["sksj"])
		if strings.Contains(timeText, "第10") {
			period10++
		}
		if strings.Contains(timeText, "第13") {
			period13++
		}
		if strings.Contains(timeText, "(单)") || strings.Contains(timeText, "(双)") {
			oddEven++
		}
	}
	t.Logf("course rows=%d, quarterCredits=%d, period10=%d, period13=%d, oddEven=%d", len(payload.Items), quarterCredits, period10, period13, oddEven)
	if target == nil {
		t.Fatal("real dist workbook did not preserve display code A0512150-04")
	}
	for key, want := range map[string]string{
		"displayCode": "(2026-2027-1)-A0512150-04",
		"courseCode":  "(2026-2027-1)-A0512150",
		"kcmc":        "软件工程",
		"xf":          "2.0",
	} {
		if got := textValue(target[key]); got != want {
			t.Fatalf("A0512150-04 %s = %q, want %q; row=%#v", key, got, want, target)
		}
	}
	if !strings.Contains(textValue(target["sksj"]), "星期三第1-2节") {
		t.Fatalf("A0512150-04 schedule = %q, want Wednesday periods 1-2", target["sksj"])
	}
	if quarterCredits == 0 {
		t.Fatal("real dist workbook did not preserve any 0.25 credit value")
	}
	if period10 == 0 {
		t.Fatal("real dist workbook did not preserve section 10 scheduling data")
	}
	if oddEven == 0 {
		t.Fatal("real dist workbook did not preserve odd/even week scheduling data")
	}
}

func TestEnsureCourseFileRecoversCreditlessJSONFromExcel(t *testing.T) {
	oldDir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	tmp := t.TempDir()
	if err := os.Chdir(tmp); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(oldDir) })

	creditless := &CoursePayload{Items: []map[string]any{{
		"jxbmc": "(2026-2027-1)-A0001001-01",
		"kcmc":  "Test Course",
		"sksj":  "Monday",
	}}}
	data, err := json.Marshal(creditless)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile("course.json", data, 0644); err != nil {
		t.Fatal(err)
	}
	if err := writeMinimalCourseExcel("course-data.xlsx"); err != nil {
		t.Fatal(err)
	}

	payload, source, err := EnsureCourseFile("course.json")
	if err != nil {
		t.Fatal(err)
	}
	if source == "json" || !HasCourseCredits(payload) {
		t.Fatalf("expected Excel recovery with credits, source=%q payload=%#v", source, payload)
	}
	backups, err := filepath.Glob("course.incomplete-*.json")
	if err != nil || len(backups) != 1 {
		t.Fatalf("expected one incomplete JSON backup, got %v, err=%v", backups, err)
	}
	reloaded, err := ReadCourseFile("course.json")
	if err != nil || !HasCourseCredits(reloaded) {
		t.Fatalf("recovered course.json has no usable credits: %#v, err=%v", reloaded, err)
	}
}

func TestEnsureCourseFileKeepsCreditCapableJSON(t *testing.T) {
	oldDir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	tmp := t.TempDir()
	if err := os.Chdir(tmp); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(oldDir) })

	payload := &CoursePayload{Items: []map[string]any{{"kcmc": "Test Course", "xf": "0.25"}}}
	data, err := json.Marshal(payload)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile("course.json", data, 0644); err != nil {
		t.Fatal(err)
	}

	loaded, source, err := EnsureCourseFile("course.json")
	if err != nil || source != "json" || !HasCourseCredits(loaded) {
		t.Fatalf("credit-capable JSON should remain unchanged: source=%q payload=%#v err=%v", source, loaded, err)
	}
}

func writeMinimalCourseExcel(name string) error {
	file, err := os.Create(name)
	if err != nil {
		return err
	}
	defer file.Close()
	writer := zip.NewWriter(file)
	sheet, err := writer.Create("xl/worksheets/sheet1.xml")
	if err != nil {
		return err
	}
	rows := [][]string{
		{"\u6559\u5b66\u73ed\u540d\u79f0", "\u8bfe\u7a0b\u540d\u79f0", "\u4e0a\u8bfe\u65f6\u95f4", "\u5b66\u5206"},
		{"(2026-2027-1)-A0001001-01", "Test Course", "\u661f\u671f\u4e00\u7b2c1-2\u8282{1-17\u5468}", "0.25"},
	}
	var builder strings.Builder
	builder.WriteString(`<?xml version="1.0" encoding="UTF-8"?><worksheet><sheetData>`)
	for rowIndex, row := range rows {
		builder.WriteString(`<row r="`)
		builder.WriteString(string(rune('1' + rowIndex)))
		builder.WriteString(`">`)
		for columnIndex, value := range row {
			builder.WriteString(`<c r="`)
			builder.WriteString(string(rune('A' + columnIndex)))
			builder.WriteString(string(rune('1' + rowIndex)))
			builder.WriteString(`" t="inlineStr"><is><t>`)
			builder.WriteString(value)
			builder.WriteString(`</t></is></c>`)
		}
		builder.WriteString(`</row>`)
	}
	builder.WriteString(`</sheetData></worksheet>`)
	if _, err := sheet.Write([]byte(builder.String())); err != nil {
		return err
	}
	return writer.Close()
}
