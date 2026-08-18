package school

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestDecodePersonalScheduleBodyAcceptsTopLevelArray(t *testing.T) {
	raw, items, err := decodePersonalScheduleBody([]byte(`[{"jxbmc":"(2026-2027-1)-A0001001-01","kcmc":"Test Math"}]`))
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 1 || len(mapSlice(raw["items"])) != 1 {
		t.Fatalf("decoded raw=%#v items=%#v", raw, items)
	}
}

func TestRefreshPersonalScheduleRequiresSession(t *testing.T) {
	service := NewService()
	_, err := service.RefreshPersonalSchedule()
	if err == nil || !strings.Contains(err.Error(), "请先完成登录") {
		t.Fatalf("RefreshPersonalSchedule() error = %v", err)
	}
}

func TestValidateExportRequestAllowsExplicitBrowserSession(t *testing.T) {
	if err := ValidateExportRequest(ExportRequest{Method: "browser", XueNian: "2026", XueQi: "1"}); err != nil {
		t.Fatalf("browser session request rejected: %v", err)
	}
	if err := ValidateExportRequest(ExportRequest{Method: "password", Username: "user", Password: "pass", XueNian: "2026", XueQi: "1"}); err != nil {
		t.Fatalf("password request rejected: %v", err)
	}
}

func TestOutputFilePathUsesConfiguredDirectoryAcrossWorkingDirectories(t *testing.T) {
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

	path, err := OutputFilePath("course.json")
	if err != nil {
		t.Fatalf("OutputFilePath() error = %v", err)
	}
	want := filepath.Join(outputDir, "course.json")
	if path != want {
		t.Fatalf("OutputFilePath() = %q, want %q", path, want)
	}
}

func TestExtractPersonalScheduleItems(t *testing.T) {
	raw := map[string]any{
		"kbList": []any{
			map[string]any{
				"kcmc": "计算机网络",
				"xqj":  "1",
				"jc":   "3-4",
				"zcd":  "1-17周",
				"xm":   "测试教师",
				"cdmc": "第6教研楼101",
			},
		},
	}

	items := extractPersonalScheduleItems(raw)
	if len(items) != 1 {
		t.Fatalf("len(items) = %d, want 1", len(items))
	}
	item := items[0]
	if item["kcmc"] != "计算机网络" {
		t.Fatalf("kcmc = %v", item["kcmc"])
	}
	if item["jzgxx"] != "测试教师" {
		t.Fatalf("jzgxx = %v", item["jzgxx"])
	}
	timeText, _ := item["sksj"].(string)
	for _, want := range []string{"星期一", "第3-4节", "1-17周"} {
		if !strings.Contains(timeText, want) {
			t.Fatalf("sksj = %q, missing %q", timeText, want)
		}
	}
}

func TestExtractCourseItemsShapes(t *testing.T) {
	tests := []struct {
		name string
		body string
	}{
		{
			name: "items",
			body: `{"items":[{"kcmc":"编译原理"}]}`,
		},
		{
			name: "rows",
			body: `{"rows":[{"kcmc":"软件工程"}]}`,
		},
		{
			name: "nested data rows",
			body: `{"data":{"rows":[{"kcmc":"计算机网络"}]}}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			items, err := extractCourseItems([]byte(tt.body))
			if err != nil {
				t.Fatalf("extractCourseItems() error = %v", err)
			}
			if len(items) != 1 {
				t.Fatalf("len(items) = %d, want 1", len(items))
			}
			if items[0]["kcmc"] == "" {
				t.Fatalf("kcmc missing in %#v", items[0])
			}
		})
	}
}

func TestTermFromRequest(t *testing.T) {
	first := termFromRequest(ExportRequest{XueNian: "2026", XueQi: "1"})
	if first.XueNian != "2026" || first.XueQi != "1" || first.Xqm != "3" {
		t.Fatalf("first term = %#v, want xnm=2026 xqm=3", first)
	}

	second := termFromRequest(ExportRequest{XueNian: "2026", XueQi: "2"})
	if second.XueNian != "2026" || second.XueQi != "2" || second.Xqm != "12" {
		t.Fatalf("second term = %#v, want xnm=2026 xqm=12", second)
	}
}

func TestWriteCourseDiagnosis(t *testing.T) {
	dir := t.TempDir()
	oldWD, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	defer func() {
		_ = os.Chdir(oldWD)
	}()

	path := writeCourseDiagnosis(
		termParams{XueNian: "2026", XueQi: "1", Xqm: "3"},
		200,
		[]byte(`{"items":[],"data":{"rows":[]}}`),
	)
	if path == "" {
		t.Fatal("writeCourseDiagnosis returned empty path")
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read diagnosis: %v", err)
	}
	text := string(data)
	for _, want := range []string{`"term": "2026-2027-1"`, `"items"`, `"$.data.rows": 0`} {
		if !strings.Contains(text, want) {
			t.Fatalf("diagnosis missing %q:\n%s", want, text)
		}
	}
}

func TestValidateTestExporterEndpointsOnlyAllowsLoopbackHTTP(t *testing.T) {
	valid := ExporterEndpoints{
		CASLogin:         "http://127.0.0.1:18674/cas/login",
		CASService:       "http://localhost:18674/cas/service",
		NewJWLogin:       "http://127.0.0.1:18674/jw/login",
		PublicKey:        "http://127.0.0.1:18674/jw/public-key",
		Course:           "http://127.0.0.1:18674/jw/course?doType=query",
		PersonalSchedule: "http://127.0.0.1:18674/jw/personal?gnmkdm=N2151",
	}
	if err := ValidateTestExporterEndpoints(valid); err != nil {
		t.Fatalf("valid loopback endpoints rejected: %v", err)
	}

	invalids := []ExporterEndpoints{
		func() ExporterEndpoints { next := valid; next.Course = "https://127.0.0.1/course"; return next }(),
		func() ExporterEndpoints { next := valid; next.Course = "http://newjw.hdu.edu.cn/course"; return next }(),
		func() ExporterEndpoints {
			next := valid
			next.Course = "http://user:pass@127.0.0.1/course"
			return next
		}(),
	}
	for _, endpoints := range invalids {
		if err := ValidateTestExporterEndpoints(endpoints); err == nil {
			t.Fatalf("unsafe endpoints unexpectedly accepted: %#v", endpoints)
		}
	}
}

func TestRunExportWithTestEndpointsRejectsUnsafeURLBeforeRequest(t *testing.T) {
	service := NewService()
	_, err := service.RunExportWithTestEndpoints(
		ExportRequest{Method: "password", Username: "test-user", Password: "test-password"},
		ExporterEndpoints{Course: "http://example.com/course"},
		time.Millisecond,
	)
	if err == nil || !strings.Contains(err.Error(), "loopback") {
		t.Fatalf("unsafe endpoint error = %v, want loopback validation failure", err)
	}
}

func TestWriteCourseFileKeepsConcurrentReadersOnCompleteJSON(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "course.json")
	payloads := [][]byte{
		[]byte(`{"items":[{"kcmc":"Course A","xf":"0.25"}]}`),
		[]byte(`{"items":[{"kcmc":"Course B","xf":"3"},{"kcmc":"Course C","xf":"1.5"}]}`),
	}
	if err := os.WriteFile(path, payloads[0], 0644); err != nil {
		t.Fatal(err)
	}

	var group sync.WaitGroup
	errors := make(chan error, 64)
	for writer := 0; writer < 8; writer++ {
		group.Add(1)
		go func(offset int) {
			defer group.Done()
			for iteration := 0; iteration < 100; iteration++ {
				if err := WriteCourseFile(path, payloads[(offset+iteration)%len(payloads)]); err != nil {
					errors <- err
					return
				}
			}
		}(writer)
	}
	for reader := 0; reader < 8; reader++ {
		group.Add(1)
		go func() {
			defer group.Done()
			for iteration := 0; iteration < 100; iteration++ {
				payload, err := ReadCourseFile(path)
				if err != nil {
					errors <- err
					return
				}
				if payload == nil || len(payload.Items) == 0 {
					errors <- fmt.Errorf("read an empty course payload")
					return
				}
			}
		}()
	}
	group.Wait()
	close(errors)
	for err := range errors {
		t.Errorf("concurrent course file access failed: %v", err)
	}
}

func TestSafePreviewRedactsStructuredSecrets(t *testing.T) {
	body := []byte(`{"password":"json-secret","token":"json-token","next":"keep"} https://example.test/login?cookie=url-secret&x=1 <input name="csrftoken" value="html-secret">`)
	preview := safePreview(body, 1000)
	for _, secret := range []string{"json-secret", "json-token", "url-secret", "html-secret"} {
		if strings.Contains(preview, secret) {
			t.Fatalf("safePreview leaked %q: %s", secret, preview)
		}
	}
	if !strings.Contains(preview, "***") {
		t.Fatalf("safePreview did not mark redacted values: %s", preview)
	}
}

func TestReadCourseFileBytesReturnsValidatedOriginalJSON(t *testing.T) {
	path := filepath.Join(t.TempDir(), "course.json")
	data := []byte(`{"schemaVersion":1,"items":[{"kcmc":"Course A","xf":"0.25"}]}`)
	if err := os.WriteFile(path, data, 0644); err != nil {
		t.Fatal(err)
	}
	got, err := ReadCourseFileBytes(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != string(data) {
		t.Fatalf("ReadCourseFileBytes changed raw JSON: got=%s want=%s", got, data)
	}
}

func TestOutputFilePathDefaultUsesPackagedExecutableDirectory(t *testing.T) {
	t.Setenv("HDU_OUTPUT_DIR", "")
	releaseRoot := filepath.Join(string(filepath.Separator), "HDU", "release")
	exe := filepath.Join(releaseRoot, "HDU-Auto-Scheduling-Script.exe")
	got, err := outputDirectoryForExecutable(exe)
	if err != nil {
		t.Fatalf("outputDirectoryForExecutable() error = %v", err)
	}
	want, absErr := filepath.Abs(releaseRoot)
	if absErr != nil {
		t.Fatalf("filepath.Abs() error = %v", absErr)
	}
	if got != want {
		t.Fatalf("outputDirectoryForExecutable() = %q, want %q", got, want)
	}
}

func TestOutputFilePathDefaultUsesWorkingDirectoryForDevExecutable(t *testing.T) {
	wd := t.TempDir()
	oldWD, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(wd); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(oldWD) })
	t.Setenv("HDU_OUTPUT_DIR", "")
	exe := filepath.Join(wd, "HDU-Auto-Scheduling-Script.test.exe")
	got, err := outputDirectoryForExecutable(exe)
	if err != nil {
		t.Fatalf("outputDirectoryForExecutable() error = %v", err)
	}
	if got != wd {
		t.Fatalf("outputDirectoryForExecutable() = %q, want %q", got, wd)
	}
}

func TestMergePersonalScheduleItemsCombinesSameTeachingClass(t *testing.T) {
	items := []map[string]any{
		{"jxb_id": "class-1", "kcmc": "操作系统", "sksj": "星期二第3-4节{1-17周}", "jxdd": "第3教研楼215"},
		{"jxb_id": "class-1", "kcmc": "操作系统", "sksj": "星期五第1-2节{1-17周}", "jxdd": "第7教研楼北308"},
		{"jxb_id": "class-2", "kcmc": "计算机网络", "sksj": "星期四第6-7节{1-17周}", "jxdd": "第3教研楼315"},
	}
	merged := MergePersonalScheduleItems(items)
	if len(merged) != 2 {
		t.Fatalf("len(merged) = %d, want 2", len(merged))
	}
	first := merged[0]
	if !strings.Contains(textAny(first["sksj"]), "星期二") || !strings.Contains(textAny(first["sksj"]), "星期五") {
		t.Fatalf("first sksj not combined: %v", first["sksj"])
	}
	if !strings.Contains(textAny(first["jxdd"]), "第3教研楼215") || !strings.Contains(textAny(first["jxdd"]), "第7教研楼北308") {
		t.Fatalf("first jxdd not combined: %v", first["jxdd"])
	}
}

func TestLoadLoginCredentialsReadsConfiguredFile(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.json")
	content := `{"newjw_login":{"username":"u12345678","password":"secret-pass"},"cas_login":{"username":"u12345678","password":"secret-pass"}}`
	if err := os.WriteFile(cfgPath, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("HDU_LOGIN_CONFIG", cfgPath)
	username, password, err := LoadLoginCredentials()
	if err != nil {
		t.Fatalf("LoadLoginCredentials() error = %v", err)
	}
	if username != "u12345678" || password != "secret-pass" {
		t.Fatalf("LoadLoginCredentials() = %q/%q", username, password)
	}
}
