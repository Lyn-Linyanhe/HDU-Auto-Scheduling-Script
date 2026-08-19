package main

import (
	"archive/zip"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	neturl "net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"hdu-scheduler/school"
)

func TestAllowedLocalOrigin(t *testing.T) {
	tests := []struct {
		origin string
		port   string
		want   bool
	}{
		{"http://127.0.0.1:6789", "6789", true},
		{"http://localhost:6789", "6789", true},
		{"http://[::1]:6789", "6789", true},
		{"http://127.0.0.1:6790", "6789", false},
		{"https://127.0.0.1:6789", "6789", false},
		{"http://example.com:6789", "6789", false},
	}

	for _, tt := range tests {
		parsed, err := neturl.Parse(tt.origin)
		if err != nil {
			t.Fatalf("parse origin %q: %v", tt.origin, err)
		}
		if got := isAllowedLocalOrigin(parsed, tt.port); got != tt.want {
			t.Fatalf("isAllowedLocalOrigin(%q, %q) = %v, want %v", tt.origin, tt.port, got, tt.want)
		}
	}
}

func TestMainListenAddressUsesConfiguredPort(t *testing.T) {
	t.Setenv("HDU_MAIN_PORT", "6791")
	address, port := mainListenAddress()
	if address != "127.0.0.1:6791" || port != "6791" {
		t.Fatalf("mainListenAddress() = (%q, %q), want (127.0.0.1:6791, 6791)", address, port)
	}
}

func TestMainListenAddressRejectsInvalidPort(t *testing.T) {
	for _, value := range []string{"", "0", "65536", "not-a-port"} {
		t.Run(value, func(t *testing.T) {
			t.Setenv("HDU_MAIN_PORT", value)
			address, port := mainListenAddress()
			if address != addr || port != "6789" {
				t.Fatalf("mainListenAddress() = (%q, %q), want (%q, 6789)", address, port, addr)
			}
		})
	}
}

func TestServeExporterStatic(t *testing.T) {
	tests := []struct {
		path        string
		contentType string
		marker      string
	}{
		{"/exporter/", "text/html; charset=utf-8", "/exporter/main.js"},
		{"/exporter/style.css", "text/css; charset=utf-8", "--bg:"},
		{"/exporter/main.js", "application/javascript; charset=utf-8", "const els"},
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, tt.path, nil)
			rec := httptest.NewRecorder()

			serveExporterStatic(rec, req)

			if rec.Code != http.StatusOK {
				t.Fatalf("serveExporterStatic(%q) status = %d, want %d", tt.path, rec.Code, http.StatusOK)
			}
			if got := rec.Header().Get("Content-Type"); got != tt.contentType {
				t.Fatalf("serveExporterStatic(%q) content type = %q, want %q", tt.path, got, tt.contentType)
			}
			if body := rec.Body.String(); !strings.Contains(body, tt.marker) {
				t.Fatalf("serveExporterStatic(%q) body does not contain %q", tt.path, tt.marker)
			}
		})
	}
}

func TestServeStaticPreservesPublicURLs(t *testing.T) {
	tests := []struct {
		path        string
		contentType string
		marker      string
	}{
		{"/", "text/html; charset=utf-8", "<title>HDU 课表自动化编排助手</title>"},
		{"/bootstrap.html", "text/html; charset=utf-8", "<title>HDU 模拟排课助手 - 导入</title>"},
		{"/scheduler.html", "text/html; charset=utf-8", "<title>HDU 课表自动化编排助手</title>"},
		{"/styles.css", "text/css; charset=utf-8", ".bootstrap-layout"},
		{"/shared.js", "application/javascript; charset=utf-8", "globalThis.HDU"},
		{"/bootstrap.js", "application/javascript; charset=utf-8", "/api/bootstrap/import"},
		{"/scheduler.js", "application/javascript; charset=utf-8", "/api/export/timetable"},
		{"/scheduler-worker.js", "application/javascript; charset=utf-8", "self.onmessage"},
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, tt.path, nil)
			rec := httptest.NewRecorder()

			serveStatic(rec, req)

			if rec.Code != http.StatusOK {
				t.Fatalf("serveStatic(%q) status = %d, want %d", tt.path, rec.Code, http.StatusOK)
			}
			if got := rec.Header().Get("Content-Type"); got != tt.contentType {
				t.Fatalf("serveStatic(%q) content type = %q, want %q", tt.path, got, tt.contentType)
			}
			if body := rec.Body.String(); !strings.Contains(body, tt.marker) {
				t.Fatalf("serveStatic(%q) body does not contain %q", tt.path, tt.marker)
			}
		})
	}
}

func TestHandlePersonalScheduleRefreshReportsMissingLoginConfig(t *testing.T) {
	t.Setenv("HDU_LOGIN_CONFIG", filepath.Join(t.TempDir(), "missing-config.json"))
	state := &appState{service: school.NewService()}
	req := httptest.NewRequest(http.MethodPost, "/api/export/personal-schedule", nil)
	rec := httptest.NewRecorder()
	handlePersonalScheduleRefresh(state)(rec, req)
	if rec.Code != http.StatusConflict {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusConflict)
	}
	if !strings.Contains(rec.Body.String(), "自动登录未启用") {
		t.Fatalf("response should explain auto-login is unavailable: %s", rec.Body.String())
	}
}

func TestHandleScheduleExportWritesTargetToProjectDirectory(t *testing.T) {
	oldDir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	tmp := t.TempDir()
	if err := os.Chdir(tmp); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(oldDir) })

	req := httptest.NewRequest(http.MethodPost, "/api/export/timetable", strings.NewReader(`{
		"kind":"target",
		"payload":{"schemaVersion":1,"source":"candidate","items":[{"jxbmc":"(2026-2027-1)-A0001001-01","kcmc":"Test Course"}]}
	}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	handleScheduleExport(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	var response struct {
		OK     bool   `json:"ok"`
		Path   string `json:"path"`
		Count  int    `json:"count"`
		Source string `json:"source"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
		t.Fatalf("response JSON: %v", err)
	}
	if !response.OK || response.Count != 1 || response.Source != "candidate" {
		t.Fatalf("unexpected response: %#v", response)
	}
	wantPath, err := filepath.Abs("target-schedule.json")
	if err != nil {
		t.Fatal(err)
	}
	if response.Path != wantPath {
		t.Fatalf("path = %q, want %q", response.Path, wantPath)
	}
	data, err := os.ReadFile(wantPath)
	if err != nil {
		t.Fatalf("target schedule was not written: %v", err)
	}
	var payload school.CoursePayload
	if err := json.Unmarshal(data, &payload); err != nil || len(payload.Items) != 1 {
		t.Fatalf("written target payload invalid: items=%d error=%v", len(payload.Items), err)
	}
}

func TestHandleScheduleExportUsesConfiguredOutputDirectory(t *testing.T) {
	outputDir := t.TempDir()
	workingDir := t.TempDir()
	t.Setenv("HDU_OUTPUT_DIR", outputDir)

	oldDir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(workingDir); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(oldDir) })

	req := httptest.NewRequest(http.MethodPost, "/api/export/timetable", strings.NewReader(`{
		"kind":"target",
		"payload":{"schemaVersion":1,"source":"candidate","items":[{"jxbmc":"(2026-2027-1)-A0001001-01","kcmc":"Configured Output Course"}]}
	}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	handleScheduleExport(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}

	var response struct {
		Path string `json:"path"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
		t.Fatalf("response JSON: %v", err)
	}
	wantPath := filepath.Join(outputDir, "target-schedule.json")
	if response.Path != wantPath {
		t.Fatalf("path = %q, want %q", response.Path, wantPath)
	}
	if _, err := os.Stat(wantPath); err != nil {
		t.Fatalf("configured output file missing: %v", err)
	}
	if _, err := os.Stat(filepath.Join(workingDir, "target-schedule.json")); !os.IsNotExist(err) {
		t.Fatalf("working directory unexpectedly contains target schedule, stat error = %v", err)
	}
}

func TestHandleStatusReadsConfiguredOutputDirectory(t *testing.T) {
	outputDir := t.TempDir()
	workingDir := t.TempDir()
	t.Setenv("HDU_OUTPUT_DIR", outputDir)

	oldDir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(workingDir); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(oldDir) })

	if err := school.WriteCourseFile(filepath.Join(outputDir, "course.json"), []byte(`{"items":[{"kcmc":"Configured Course","xf":"3"}]}`)); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(outputDir, "personal-schedule.json"), []byte(`{"items":[{"kcmc":"Configured Personal"}]}`), 0644); err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/status", nil)
	rec := httptest.NewRecorder()
	handleStatus(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	var response school.StatusResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}
	if !response.Ready || response.Count != 1 || response.PersonalCount != 1 || !response.PersonalExported {
		t.Fatalf("unexpected configured status: %#v", response)
	}
}

func TestSchedulerExportUsesProjectWriter(t *testing.T) {
	data, err := os.ReadFile("web/scheduler.js")
	if err != nil {
		t.Fatal(err)
	}
	text := string(data)
	if !strings.Contains(text, "'/api/export/timetable'") && !strings.Contains(text, "\"/api/export/timetable\"") {
		t.Fatal("scheduler export does not call the project timetable writer")
	}
	if strings.Contains(text, "URL.createObjectURL(blob)") {
		t.Fatal("scheduler export still downloads the timetable through the browser")
	}
}

func TestHandleStatusAllowsCourseOnlyAndReportsPersonalSchedule(t *testing.T) {
	oldDir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	tmp := t.TempDir()
	if err := os.Chdir(tmp); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = os.Chdir(oldDir)
	})

	courseData, err := os.ReadFile(filepath.Join(oldDir, "testdata", "course.sample.json"))
	if err != nil {
		t.Fatal(err)
	}
	personalData, err := os.ReadFile(filepath.Join(oldDir, "testdata", "personal-schedule.sample.json"))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile("course.json", courseData, 0644); err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/status", nil)
	rec := httptest.NewRecorder()
	handleStatus(rec, req)
	if body := rec.Body.String(); !strings.Contains(body, `"ready":true`) || !strings.Contains(body, `personal-schedule.json`) {
		t.Fatalf("status with course only should be ready and report missing personal schedule: %s", body)
	}

	if err := os.WriteFile("personal-schedule.json", personalData, 0644); err != nil {
		t.Fatal(err)
	}
	rec = httptest.NewRecorder()
	handleStatus(rec, req)
	if body := rec.Body.String(); !strings.Contains(body, `"ready":true`) || !strings.Contains(body, `"personalExported":true`) {
		t.Fatalf("status with both files should be ready: %s", body)
	}
}

func TestHandleStatusDoesNotRepairCourseFileOnGet(t *testing.T) {
	oldDir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	tmp := t.TempDir()
	if err := os.Chdir(tmp); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(oldDir) })

	data, err := json.Marshal(map[string]any{
		"items": []map[string]any{{
			"jxbmc": "(2026-2027-1)-A0001001-01",
			"kcmc":  "Test Course",
		}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile("course.json", data, 0644); err != nil {
		t.Fatal(err)
	}
	if err := writeTestCourseWorkbook("course-data.xlsx"); err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/status", nil)
	rec := httptest.NewRecorder()
	handleStatus(rec, req)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"ready":true`) {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	got, err := os.ReadFile("course.json")
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != string(data) {
		t.Fatalf("GET /api/status rewrote course.json: before=%d bytes after=%d bytes", len(data), len(got))
	}
	backups, err := filepath.Glob("course.incomplete-*.json")
	if err != nil {
		t.Fatal(err)
	}
	if len(backups) != 0 {
		t.Fatalf("GET /api/status created backups: %v", backups)
	}
}

func TestHandleCourseDoesNotRepairCourseFileOnGet(t *testing.T) {
	oldDir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	tmp := t.TempDir()
	if err := os.Chdir(tmp); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(oldDir) })

	data := []byte(`{"items":[{"jxbmc":"(2026-2027-1)-A0001001-01","kcmc":"Test Course"}]}`)
	if err := os.WriteFile("course.json", data, 0644); err != nil {
		t.Fatal(err)
	}
	if err := writeTestCourseWorkbook("course-data.xlsx"); err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/course", nil)
	rec := httptest.NewRecorder()
	handleCourse(rec, req)
	if rec.Code != http.StatusOK || rec.Body.String() != string(data) {
		t.Fatalf("course = %d %s", rec.Code, rec.Body.String())
	}
	backups, err := filepath.Glob("course.incomplete-*.json")
	if err != nil {
		t.Fatal(err)
	}
	if len(backups) != 0 {
		t.Fatalf("GET /api/course created backups: %v", backups)
	}
}

func TestHandleStatusDoesNotTouchCreditCapableJSONOnGet(t *testing.T) {
	oldDir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	tmp := t.TempDir()
	if err := os.Chdir(tmp); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(oldDir) })

	data := []byte(`{"items":[{"jxbmc":"(2026-2027-1)-A0001001-01","kcmc":"Test Course","xf":"3"}]}`)
	if err := os.WriteFile("course.json", data, 0644); err != nil {
		t.Fatal(err)
	}
	if err := writeTestCourseWorkbook("course-data.xlsx"); err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/status", nil)
	rec := httptest.NewRecorder()
	handleStatus(rec, req)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"ready":true`) {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	got, err := os.ReadFile("course.json")
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != string(data) {
		t.Fatalf("credit-capable GET changed course.json: before=%d bytes after=%d bytes", len(data), len(got))
	}
	backups, err := filepath.Glob("course.incomplete-*.json")
	if err != nil {
		t.Fatal(err)
	}
	if len(backups) != 0 {
		t.Fatalf("credit-capable GET created backups: %v", backups)
	}
}

func writeTestCourseWorkbook(name string) error {
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
		{"教学班名称", "课程名称", "学分"},
		{"(2026-2027-1)-A0001001-01", "Test Course", "0.25"},
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
