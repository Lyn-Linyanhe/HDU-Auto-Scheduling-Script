package main

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func fixturePayload(items []map[string]any) CoursePayload {
	return CoursePayload{SchemaVersion: 1, Source: "test", Term: "2026-2027-1", Items: items}
}

func TestDefaultDownloadsDirHonorsEnvironmentOverride(t *testing.T) {
	override := filepath.Join(t.TempDir(), "Downloads")
	t.Setenv("HDU_AGENT_DOWNLOADS_DIR", override)
	if got := defaultDownloadsDir(); got != override {
		t.Fatalf("defaultDownloadsDir() = %q, want %q", got, override)
	}
}

func TestSmartAgentListenAddressUsesConfiguredPort(t *testing.T) {
	t.Setenv("HDU_SMART_AGENT_PORT", "6901")
	address, port := smartAgentListenAddress()
	if address != "127.0.0.1:6901" || port != "6901" {
		t.Fatalf("smartAgentListenAddress() = (%q, %q), want (127.0.0.1:6901, 6901)", address, port)
	}
}

func TestSmartAgentListenAddressRejectsInvalidPort(t *testing.T) {
	for _, value := range []string{"", "0", "65536", "not-a-port"} {
		t.Run(value, func(t *testing.T) {
			t.Setenv("HDU_SMART_AGENT_PORT", value)
			address, port := smartAgentListenAddress()
			if address != addr || port != "6899" {
				t.Fatalf("smartAgentListenAddress() = (%q, %q), want (%q, 6899)", address, port, addr)
			}
		})
	}
}

func TestDefaultAgentSettingsUseSixtySecondRefreshInterval(t *testing.T) {
	settings := defaultAgentSettings()
	if settings.RefreshIntervalSeconds != 60 {
		t.Fatalf("default refresh interval = %d seconds, want 60", settings.RefreshIntervalSeconds)
	}
	if settings.RefreshIntervalMinutes != 1 {
		t.Fatalf("legacy refresh interval = %d minutes, want 1", settings.RefreshIntervalMinutes)
	}
}

func TestCleanAgentSettingsMigratesLegacyMinutesToSeconds(t *testing.T) {
	settings := cleanAgentSettings(AgentSettings{RefreshIntervalMinutes: 15})
	if settings.RefreshIntervalSeconds != 15*60 {
		t.Fatalf("migrated refresh interval = %d seconds, want 900", settings.RefreshIntervalSeconds)
	}
	if settings.RefreshIntervalMinutes != 15 {
		t.Fatalf("legacy refresh interval = %d minutes, want 15", settings.RefreshIntervalMinutes)
	}
}

func TestValidateAgentSettingsUsesSecondBounds(t *testing.T) {
	for _, seconds := range []int{9, 7201} {
		if err := validateAgentSettings(AgentSettings{RefreshIntervalSeconds: seconds}); err == nil {
			t.Fatalf("refresh interval %d seconds was accepted outside bounds", seconds)
		}
	}
	if err := validateAgentSettings(AgentSettings{RefreshIntervalSeconds: 3600}); err != nil {
		t.Fatalf("refresh interval 3600 seconds was rejected: %v", err)
	}
}

func TestExecutionLogNeedsRefreshOnlyForSuccessfulSelectOrDrop(t *testing.T) {
	if !executionLogNeedsRefresh(ExecutionLog{Items: []ExecutionLogItem{
		{Action: "select", Status: "success"},
	}}) {
		t.Fatal("successful select should request a personal schedule refresh")
	}
	if !executionLogNeedsRefresh(ExecutionLog{Items: []ExecutionLogItem{
		{Action: "drop", Status: "success"},
	}}) {
		t.Fatal("successful drop should request a personal schedule refresh")
	}
	if executionLogNeedsRefresh(ExecutionLog{Items: []ExecutionLogItem{
		{Action: "select", Status: "failed"},
		{Action: "drop", Status: "running"},
	}}) {
		t.Fatal("failed or running actions should not request a refresh")
	}
}

func TestHandleExecutionParseLogSignalsSuccessfulAction(t *testing.T) {
	code := "(2026-2027-1)-A0001001-01"
	p := setupAgentHTTPWorkspace(t, fixturePayload([]map[string]any{{
		"displayCode": code,
		"courseName":  "Test Course",
	}}))
	logDir := filepath.Join(p.killCourseDir, "log_files")
	if err := os.MkdirAll(logDir, 0755); err != nil {
		t.Fatal(err)
	}
	logLine := "2026/07/08 12:00:01.000000 [INFO] 正在处理课程: " + code + "\n" +
		"2026/07/08 12:00:02.000000 [INFO] 选课成功\n"
	if err := os.WriteFile(filepath.Join(logDir, "app.log"), []byte(logLine), 0644); err != nil {
		t.Fatal(err)
	}

	reqBody, err := json.Marshal(ExecutionLogRequest{
		Plan: ActionPlan{
			Term:   "2026-2027-1",
			Select: []Course{{DisplayCode: code}},
		},
		WriteExecutionLog: false,
	})
	if err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(http.MethodPost, "/api/execution/parse-log", bytes.NewReader(reqBody))
	rec := httptest.NewRecorder()
	handleExecutionParseLog(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	var response ExecutionLogResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}
	if !response.OK || !response.RefreshAfterSuccess {
		t.Fatalf("successful execution did not signal refresh: %#v", response)
	}
}

func TestDiscoverTargetSchedulePrefersFixedValidFile(t *testing.T) {
	workspace := t.TempDir()
	schedulerDir := filepath.Join(workspace, "Scheduler")
	if err := os.MkdirAll(schedulerDir, 0755); err != nil {
		t.Fatal(err)
	}
	fixed := filepath.Join(workspace, "target-schedule.json")
	legacy := filepath.Join(schedulerDir, "hdu-target-timetable-2026-08-18.json")
	mustWriteJSON(t, fixed, fixturePayload([]map[string]any{{"displayCode": "(2026-2027-1)-A0001001-01", "courseName": "Fixed"}}))
	mustWriteJSON(t, legacy, fixturePayload([]map[string]any{{"displayCode": "(2026-2027-1)-A0001001-02", "courseName": "Legacy"}}))

	candidate, warnings := discoverTargetSchedule(paths{workspace: workspace, schedulerDir: schedulerDir})
	if candidate.Path != fixed || len(candidate.Payload.Items) != 1 || candidate.Payload.Items[0]["displayCode"] != "(2026-2027-1)-A0001001-01" {
		t.Fatalf("candidate = %#v, want fixed target", candidate)
	}
	if len(warnings) != 0 {
		t.Fatalf("warnings = %v, want none", warnings)
	}
}

func TestDiscoverTargetScheduleFindsDownloadsTargetFile(t *testing.T) {
	workspace := t.TempDir()
	schedulerDir := filepath.Join(workspace, "Scheduler")
	downloadsDir := filepath.Join(workspace, "Downloads")
	for _, dir := range []string{schedulerDir, downloadsDir} {
		if err := os.MkdirAll(dir, 0755); err != nil {
			t.Fatal(err)
		}
	}
	legacy := filepath.Join(downloadsDir, "hdu-target-timetable-2026-08-18.json")
	mustWriteJSON(t, legacy, fixturePayload([]map[string]any{{"displayCode": "(2026-2027-1)-A0001001-01", "courseName": "Downloaded"}}))

	candidate, warnings := discoverTargetSchedule(paths{
		workspace:    workspace,
		schedulerDir: schedulerDir,
		downloadsDir: downloadsDir,
	})
	if candidate.Path != legacy || len(candidate.Items) != 1 || candidate.Items[0].CourseName != "Downloaded" {
		t.Fatalf("candidate = %#v, want downloaded legacy target", candidate)
	}
	if len(warnings) != 0 {
		t.Fatalf("warnings = %v, want none", warnings)
	}
}

func TestDiscoverTargetScheduleIgnoresCurrentTimetableFiles(t *testing.T) {
	workspace := t.TempDir()
	downloadsDir := filepath.Join(workspace, "Downloads")
	if err := os.MkdirAll(downloadsDir, 0755); err != nil {
		t.Fatal(err)
	}
	current := filepath.Join(downloadsDir, "hdu-current-timetable-2026-08-17-17-39-27 (1).json")
	mustWriteJSON(t, current, CoursePayload{
		SchemaVersion: 1,
		Source:        "current",
		Term:          "2026-2027-1",
		Items:         []map[string]any{{"displayCode": "(2026-2027-1)-CURRENT-01", "courseName": "Current Schedule"}},
	})

	candidate, warnings := discoverTargetSchedule(paths{workspace: workspace, downloadsDir: downloadsDir})
	if candidate.Path != "" || len(candidate.Items) != 0 {
		t.Fatalf("current timetable was auto-imported as target: candidate=%#v", candidate)
	}
	if len(warnings) != 0 {
		t.Fatalf("warnings = %v, want none for ignored current timetable", warnings)
	}
}

func TestDiscoverTargetSchedulePrefersFixedFileOverDownloadsLegacyFile(t *testing.T) {
	workspace := t.TempDir()
	schedulerDir := filepath.Join(workspace, "Scheduler")
	downloadsDir := filepath.Join(workspace, "Downloads")
	for _, dir := range []string{schedulerDir, downloadsDir} {
		if err := os.MkdirAll(dir, 0755); err != nil {
			t.Fatal(err)
		}
	}
	fixed := filepath.Join(workspace, "target-schedule.json")
	legacy := filepath.Join(downloadsDir, "hdu-target-timetable-newer.json")
	mustWriteJSON(t, fixed, fixturePayload([]map[string]any{{"displayCode": "(2026-2027-1)-A0001001-01", "courseName": "Fixed"}}))
	mustWriteJSON(t, legacy, fixturePayload([]map[string]any{{"displayCode": "(2026-2027-1)-A0001001-02", "courseName": "Downloaded"}}))

	candidate, warnings := discoverTargetSchedule(paths{
		workspace:    workspace,
		schedulerDir: schedulerDir,
		downloadsDir: downloadsDir,
	})
	if candidate.Path != fixed || candidate.Items[0].CourseName != "Fixed" {
		t.Fatalf("candidate = %#v, want fixed target", candidate)
	}
	if len(warnings) != 0 {
		t.Fatalf("warnings = %v, want none", warnings)
	}
}

func TestDiscoverTargetScheduleSkipsMalformedNewestLegacyFile(t *testing.T) {
	workspace := t.TempDir()
	schedulerDir := filepath.Join(workspace, "Scheduler")
	if err := os.MkdirAll(schedulerDir, 0755); err != nil {
		t.Fatal(err)
	}
	old := filepath.Join(schedulerDir, "hdu-target-timetable-old.json")
	newer := filepath.Join(schedulerDir, "hdu-target-timetable-new.json")
	mustWriteJSON(t, old, fixturePayload([]map[string]any{{"displayCode": "(2026-2027-1)-A0001001-01", "courseName": "Legacy"}}))
	if err := os.WriteFile(newer, []byte(`{"items":"invalid"}`), 0644); err != nil {
		t.Fatal(err)
	}
	now := time.Now()
	if err := os.Chtimes(old, now.Add(-time.Hour), now.Add(-time.Hour)); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(newer, now, now); err != nil {
		t.Fatal(err)
	}

	candidate, warnings := discoverTargetSchedule(paths{workspace: workspace, schedulerDir: schedulerDir})
	if candidate.Path != old {
		t.Fatalf("candidate path = %q, want %q", candidate.Path, old)
	}
	if len(warnings) != 1 || !strings.Contains(warnings[0], "无效") {
		t.Fatalf("warnings = %v, want malformed-file warning", warnings)
	}
}

func TestValidateMainBaseURLRequiresLoopback(t *testing.T) {
	for _, value := range []string{"https://example.com", "http://192.168.1.10:6789"} {
		if err := validateMainBaseURL(value); err == nil {
			t.Fatalf("validateMainBaseURL(%q) unexpectedly succeeded", value)
		}
	}
	for _, value := range []string{"http://127.0.0.1:6789", "http://localhost:6789", "http://[::1]:6789"} {
		if err := validateMainBaseURL(value); err != nil {
			t.Fatalf("validateMainBaseURL(%q) error = %v", value, err)
		}
	}
}

func TestLiveScheduleRefreshUsesMainExporter(t *testing.T) {
	mainServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/api/export/personal-schedule":
			w.WriteHeader(http.StatusAccepted)
			_, _ = w.Write([]byte(`{"ok":true}`))
		case r.Method == http.MethodGet && r.URL.Path == "/api/export/status":
			_, _ = w.Write([]byte(`{"phase":"success","step":"done","ready":true,"message":"个人课表刷新完成"}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer mainServer.Close()

	p := setupAgentHTTPWorkspace(t, fixturePayload([]map[string]any{{
		"displayCode": "(2026-2027-1)-A0001001-01",
		"courseName":  "Test Math",
	}}))
	settings := AgentSettings{
		SchedulerDir:           p.schedulerDir,
		KillCourseDir:          p.killCourseDir,
		MainBaseURL:            mainServer.URL,
		RefreshIntervalMinutes: 15,
	}
	autoRefresh := true
	settings.AutoRefresh = &autoRefresh
	mustWriteJSON(t, p.settingsPath, settings)
	mustWriteJSON(t, p.personalPath, fixturePayload([]map[string]any{{
		"displayCode": "(2026-2027-1)-A0001001-01",
		"courseName":  "Test Math",
	}}))

	req := httptest.NewRequest(http.MethodPost, "/api/live-schedule/refresh", nil)
	rec := httptest.NewRecorder()
	handleLiveScheduleRefresh(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	var response LiveScheduleResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if !response.OK || len(response.Items) != 1 || response.Sync.Source != "main-exporter" || response.Sync.SyncedAt == "" {
		t.Fatalf("unexpected refresh response: %#v", response)
	}
}

func TestLiveScheduleRefreshReturnsGatewayErrorStatus(t *testing.T) {
	mainServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost && r.URL.Path == "/api/export/personal-schedule" {
			w.WriteHeader(http.StatusServiceUnavailable)
			_, _ = w.Write([]byte(`{"error":"学校系统暂时不可用"}`))
			return
		}
		http.NotFound(w, r)
	}))
	t.Cleanup(mainServer.Close)

	p := setupAgentHTTPWorkspace(t, fixturePayload([]map[string]any{{
		"displayCode": "(2026-2027-1)-A0001001-01",
		"courseName":  "Test Math",
	}}))
	autoRefresh := false
	mustWriteJSON(t, p.settingsPath, AgentSettings{
		SchedulerDir:           p.schedulerDir,
		KillCourseDir:          p.killCourseDir,
		MainBaseURL:            mainServer.URL,
		AutoRefresh:            &autoRefresh,
		RefreshIntervalMinutes: 15,
	})

	req := httptest.NewRequest(http.MethodPost, "/api/live-schedule/refresh", nil)
	rec := httptest.NewRecorder()
	handleLiveScheduleRefresh(rec, req)
	if rec.Code != http.StatusBadGateway {
		t.Fatalf("status = %d, body = %s, want %d", rec.Code, rec.Body.String(), http.StatusBadGateway)
	}
	var response LiveScheduleResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if response.OK || response.Error == "" {
		t.Fatalf("unexpected gateway error response: %#v", response)
	}
}

func TestMainRefreshHTTPClientUsesContextDeadline(t *testing.T) {
	client := newMainRefreshHTTPClient()
	if client.Timeout != 0 {
		t.Fatalf("main refresh client timeout = %s, want context-controlled timeout", client.Timeout)
	}
}

func TestWriteLiveScheduleSnapshotAcceptsEmptySchedule(t *testing.T) {
	tmp := t.TempDir()
	p := paths{
		workspace:        tmp,
		personalPath:     filepath.Join(tmp, "personal-schedule.json"),
		livePersonalPath: filepath.Join(tmp, "personal-schedule-live.json"),
		liveSyncPath:     filepath.Join(tmp, "live-schedule-sync.json"),
	}
	mustWriteJSON(t, p.personalPath, fixturePayload([]map[string]any{}))
	response, err := writeLiveScheduleSnapshot(p, fixturePayload([]map[string]any{}), "test")
	if err != nil {
		t.Fatalf("writeLiveScheduleSnapshot() error = %v", err)
	}
	if !response.OK || !response.HasSnapshot || response.Sync.LiveCount != 0 || response.Sync.SyncedAt == "" {
		t.Fatalf("unexpected empty snapshot response: %#v", response)
	}
}

func TestScheduleHashIncludesCourseSemantics(t *testing.T) {
	base := Course{
		DisplayCode: "(2026-2027-1)-A0001001-01",
		GroupID:     "(2026-2027-1)-A0001001",
		CourseName:  "高等数学A",
		Teacher:     "张老师",
		TimeText:    "星期一第1-2节{1-17周}",
		Location:    "第6教研楼101",
	}
	changedTeacher := base
	changedTeacher.Teacher = "李老师"
	changedTime := base
	changedTime.TimeText = "星期二第1-2节{1-17周}"
	if scheduleHash([]Course{base}) == scheduleHash([]Course{changedTeacher}) {
		t.Fatal("schedule hash ignored teacher changes")
	}
	if scheduleHash([]Course{base}) == scheduleHash([]Course{changedTime}) {
		t.Fatal("schedule hash ignored time changes")
	}
}

func TestLiveScheduleSyncDetectsSemanticChangeForSameCourseCode(t *testing.T) {
	tmp := t.TempDir()
	p := paths{personalPath: filepath.Join(tmp, "personal-schedule.json"), livePersonalPath: filepath.Join(tmp, "personal-schedule-live.json")}
	local := Course{DisplayCode: "(2026-2027-1)-A0001001-01", CourseName: "高等数学A", Teacher: "张老师", TimeText: "星期一第1-2节"}
	live := local
	live.Teacher = "李老师"
	localPayload := CoursePayload{Items: []map[string]any{{"displayCode": local.DisplayCode, "courseName": local.CourseName, "teacher": local.Teacher, "timeText": local.TimeText}}}
	livePayload := CoursePayload{Items: []map[string]any{{"displayCode": live.DisplayCode, "courseName": live.CourseName, "teacher": live.Teacher, "timeText": live.TimeText}}}
	mustWriteJSON(t, p.personalPath, localPayload)
	mustWriteJSON(t, p.livePersonalPath, livePayload)
	sync, _ := buildLiveScheduleSyncWithSource(p, "test")
	if !sync.HasDrift || sync.LiveHash == sync.LocalHash || len(sync.Changed) != 1 {
		t.Fatalf("semantic schedule change was not detected: %#v", sync)
	}
}

func TestWriteLiveScheduleSnapshotPreservesFilesWhenSyncPreparationFails(t *testing.T) {
	tmp := t.TempDir()
	p := paths{
		workspace:        tmp,
		personalPath:     filepath.Join(tmp, "personal-schedule.json"),
		livePersonalPath: filepath.Join(tmp, "personal-schedule-live.json"),
		liveSyncPath:     filepath.Join(tmp, "blocked", "live-schedule-sync.json"),
	}
	oldPayload := fixturePayload([]map[string]any{{
		"displayCode": "(2026-2027-1)-OLD-01",
		"courseName":  "旧课程",
	}})
	newPayload := fixturePayload([]map[string]any{{
		"displayCode": "(2026-2027-1)-NEW-01",
		"courseName":  "新课程",
	}})
	mustWriteJSON(t, p.personalPath, oldPayload)
	mustWriteJSON(t, p.livePersonalPath, oldPayload)
	oldSync := LiveScheduleSync{SchemaVersion: schemaVersion, Source: "old", LiveHash: "old-hash"}
	mustWriteJSON(t, filepath.Join(tmp, "old-sync.json"), oldSync)
	if err := os.WriteFile(filepath.Join(tmp, "blocked"), []byte("not a directory"), 0644); err != nil {
		t.Fatal(err)
	}
	oldLiveBytes, err := os.ReadFile(p.livePersonalPath)
	if err != nil {
		t.Fatal(err)
	}
	response, err := writeLiveScheduleSnapshot(p, newPayload, "test")
	if err == nil {
		t.Fatalf("expected sync preparation failure, response=%#v", response)
	}
	newLiveBytes, readErr := os.ReadFile(p.livePersonalPath)
	if readErr != nil {
		t.Fatal(readErr)
	}
	if string(newLiveBytes) != string(oldLiveBytes) {
		t.Fatal("live snapshot changed even though sync file preparation failed")
	}
}

func TestRefreshFromMainUsesReportedPersonalSchedulePath(t *testing.T) {
	tmp := t.TempDir()
	reportedPath := filepath.Join(tmp, "alternate", "personal-schedule.json")
	if err := os.MkdirAll(filepath.Dir(reportedPath), 0755); err != nil {
		t.Fatal(err)
	}
	mustWriteJSON(t, reportedPath, fixturePayload([]map[string]any{{
		"displayCode": "(2026-2027-1)-REPORTED-01",
		"courseName":  "Reported Course",
	}}))
	mainServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/api/export/personal-schedule":
			w.WriteHeader(http.StatusAccepted)
			_, _ = w.Write([]byte(`{"ok":true,"status":{"phase":"exporting"}}`))
		case r.Method == http.MethodGet && r.URL.Path == "/api/export/status":
			encodedPath, _ := json.Marshal(reportedPath)
			_, _ = w.Write([]byte(`{"phase":"success","personalOutputPath":` + string(encodedPath) + `}`))
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(mainServer.Close)

	p := paths{
		workspace:        tmp,
		personalPath:     filepath.Join(tmp, "scheduler", "personal-schedule.json"),
		livePersonalPath: filepath.Join(tmp, "scheduler", "personal-schedule-live.json"),
		liveSyncPath:     filepath.Join(tmp, "live-schedule-sync.json"),
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	response, err := refreshFromMain(ctx, mainServer.URL, p)
	if err != nil {
		t.Fatal(err)
	}
	if len(response.Items) != 1 || response.Items[0].DisplayCode != "(2026-2027-1)-REPORTED-01" {
		t.Fatalf("refresh ignored reported personal path: %#v", response.Items)
	}
}

func TestNormalizeCoursePreservesCapabilitiesAndCapacity(t *testing.T) {
	selectEnabled := false
	dropEnabled := true
	course := normalizeCourse(map[string]any{
		"displayCode":   "(2026-2027-1)-A0001001-01",
		"courseCode":    "(2026-2027-1)-A0001001",
		"kcmc":          "数学",
		"selectEnabled": selectEnabled,
		"dropEnabled":   dropEnabled,
		"selectRounds":  []any{float64(1), float64(2)},
		"jxbrl":         "80",
		"jxbrs":         "52",
		"xkrs":          "51",
		"jxbzc":         "计算机科学2301",
	}, 0)
	if course.SelectEnabled == nil || *course.SelectEnabled != selectEnabled {
		t.Fatalf("select capability was not preserved: %#v", course)
	}
	if course.DropEnabled == nil || *course.DropEnabled != dropEnabled {
		t.Fatalf("drop capability was not preserved: %#v", course)
	}
	if len(course.SelectRounds) != 2 || course.SelectRounds[0] != 1 || course.SelectRounds[1] != 2 {
		t.Fatalf("selection rounds were not parsed: %#v", course.SelectRounds)
	}
	if course.Capacity == nil || *course.Capacity != 80 || course.Enrolled == nil || *course.Enrolled != 52 || course.Selected == nil || *course.Selected != 51 {
		t.Fatalf("capacity fields were not parsed: %#v", course)
	}
	if course.ClassName != "计算机科学2301" {
		t.Fatalf("class name was not preserved: %#v", course)
	}
}

func TestCourseReadOnlyEndpointsExposeOptionsClassScheduleAndCapacity(t *testing.T) {
	selectionRound := 1
	payload := fixturePayload([]map[string]any{
		{
			"displayCode":   "(2026-2027-1)-A0001001-01",
			"courseCode":    "(2026-2027-1)-A0001001",
			"kcmc":          "数学",
			"jzgxx":         "张老师",
			"sksj":          "星期一第1-2节{1-17周}",
			"selectEnabled": false,
			"dropEnabled":   true,
			"selectRounds":  []any{float64(1)},
			"jxbrl":         "80",
			"jxbrs":         "52",
			"xkrs":          "51",
			"jxbzc":         "计算机科学2301",
		},
		{
			"displayCode":   "(2026-2027-1)-A0001001-02",
			"courseCode":    "(2026-2027-1)-A0001001",
			"kcmc":          "数学",
			"jzgxx":         "李老师",
			"sksj":          "星期二第1-2节{1-17周}",
			"selectEnabled": true,
			"dropEnabled":   false,
			"selectRounds":  []any{float64(2)},
		},
	})
	payload.CurrentRound = &selectionRound
	setupAgentHTTPWorkspace(t, payload)

	optionsRec := httptest.NewRecorder()
	handleCourseOptions(optionsRec, httptest.NewRequest(http.MethodGet, "/api/course-options", nil))
	if optionsRec.Code != http.StatusOK {
		t.Fatalf("course options status=%d body=%s", optionsRec.Code, optionsRec.Body.String())
	}
	var options CourseOptionsResponse
	if err := json.Unmarshal(optionsRec.Body.Bytes(), &options); err != nil {
		t.Fatal(err)
	}
	if !options.OK || options.CurrentRound == nil || *options.CurrentRound != selectionRound || len(options.Items) != 2 {
		t.Fatalf("course options response mismatch: %#v", options)
	}

	code := "(2026-2027-1)-A0001001-01"
	scheduleRec := httptest.NewRecorder()
	handleClassSchedule(scheduleRec, httptest.NewRequest(http.MethodGet, "/api/class-schedule?displayCode="+url.QueryEscape(code), nil))
	if scheduleRec.Code != http.StatusOK {
		t.Fatalf("class schedule status=%d body=%s", scheduleRec.Code, scheduleRec.Body.String())
	}
	var schedule ClassScheduleResponse
	if err := json.Unmarshal(scheduleRec.Body.Bytes(), &schedule); err != nil {
		t.Fatal(err)
	}
	if !schedule.OK || len(schedule.Items) != 1 || schedule.Items[0].DisplayCode != code || schedule.Items[0].ClassName != "计算机科学2301" {
		t.Fatalf("class schedule response mismatch: %#v", schedule)
	}

	capacityRec := httptest.NewRecorder()
	handleCourseCapacity(capacityRec, httptest.NewRequest(http.MethodGet, "/api/course-capacity?groupId="+url.QueryEscape("(2026-2027-1)-A0001001"), nil))
	if capacityRec.Code != http.StatusOK {
		t.Fatalf("course capacity status=%d body=%s", capacityRec.Code, capacityRec.Body.String())
	}
	var capacity CourseCapacityResponse
	if err := json.Unmarshal(capacityRec.Body.Bytes(), &capacity); err != nil {
		t.Fatal(err)
	}
	if !capacity.OK || len(capacity.Items) != 2 || capacity.Items[0].Remaining == nil || *capacity.Items[0].Remaining != 28 || capacity.Items[0].Full == nil || *capacity.Items[0].Full {
		t.Fatalf("course capacity response mismatch: %#v", capacity)
	}
	if capacity.Items[1].Capacity != nil || capacity.Items[1].Remaining != nil || capacity.Items[1].Full != nil {
		t.Fatalf("missing capacity must remain unknown, got %#v", capacity.Items[1])
	}
	var capacityEnvelope map[string]any
	if err := json.Unmarshal(capacityRec.Body.Bytes(), &capacityEnvelope); err != nil {
		t.Fatal(err)
	}
	if stale, ok := capacityEnvelope["stale"].(bool); !ok || !stale {
		t.Fatalf("course.json capacity must be marked stale: %#v", capacityEnvelope)
	}
	if sourceUpdatedAt, ok := capacityEnvelope["sourceUpdatedAt"].(string); !ok || strings.TrimSpace(sourceUpdatedAt) == "" {
		t.Fatalf("capacity response must expose sourceUpdatedAt: %#v", capacityEnvelope)
	}
}

func TestBuildActionPlanBlocksUnsupportedSelectionAndRound(t *testing.T) {
	tmp := t.TempDir()
	p := paths{
		workspace:        tmp,
		schedulerDir:     tmp,
		coursePath:       filepath.Join(tmp, "course.json"),
		personalPath:     filepath.Join(tmp, "personal-schedule.json"),
		livePersonalPath: filepath.Join(tmp, "personal-schedule-live.json"),
		killConfigPath:   filepath.Join(tmp, "config.json"),
	}
	all := fixturePayload([]map[string]any{
		{
			"displayCode":   "(2026-2027-1)-A0001001-01",
			"courseCode":    "(2026-2027-1)-A0001001",
			"kcmc":          "不可选课程",
			"sksj":          "星期一第1-2节{1-17周}",
			"selectEnabled": false,
			"selectRounds":  []any{float64(2)},
		},
	})
	all.CurrentRound = func() *int { value := 1; return &value }()
	mustWriteJSON(t, p.coursePath, all)
	target := all
	plan, _, err := buildActionPlan(PlanRequest{TargetPayload: target}, p)
	if err != nil {
		t.Fatal(err)
	}
	if !hasValidationMessage(plan.Validation, "不可选") || !hasValidationMessage(plan.Validation, "选课轮次") {
		t.Fatalf("unsupported selection and round should block: %#v", plan.Validation)
	}
	if len(blockingValidationIssues(plan.Validation)) < 2 {
		t.Fatalf("expected two blocking capability issues: %#v", plan.Validation)
	}
}

func TestBuildActionPlanBlocksSelectionWhenCurrentRoundIsUnknown(t *testing.T) {
	tmp := t.TempDir()
	p := paths{
		workspace:        tmp,
		schedulerDir:     tmp,
		coursePath:       filepath.Join(tmp, "course.json"),
		personalPath:     filepath.Join(tmp, "personal-schedule.json"),
		livePersonalPath: filepath.Join(tmp, "personal-schedule-live.json"),
		killConfigPath:   filepath.Join(tmp, "config.json"),
	}
	all := fixturePayload([]map[string]any{
		{
			"displayCode":  "(2026-2027-1)-A0001001-01",
			"courseCode":   "(2026-2027-1)-A0001001",
			"kcmc":         "轮次未知课程",
			"sksj":         "星期一第1-2节{1-17周}",
			"selectRounds": []any{float64(1), float64(2)},
		},
	})
	mustWriteJSON(t, p.coursePath, all)
	plan, _, err := buildActionPlan(PlanRequest{TargetPayload: all}, p)
	if err != nil {
		t.Fatal(err)
	}
	if !hasValidationMessage(plan.Validation, "当前选课轮次未知") {
		t.Fatalf("expected unknown selection round issue: %#v", plan.Validation)
	}
	if len(blockingValidationIssues(plan.Validation)) == 0 {
		t.Fatalf("unknown selection round must block executable plan: %#v", plan.Validation)
	}
}

func TestBuildActionPlanBlocksFullSelection(t *testing.T) {
	tmp := t.TempDir()
	p := paths{
		workspace:        tmp,
		schedulerDir:     tmp,
		coursePath:       filepath.Join(tmp, "course.json"),
		personalPath:     filepath.Join(tmp, "personal-schedule.json"),
		livePersonalPath: filepath.Join(tmp, "personal-schedule-live.json"),
		killConfigPath:   filepath.Join(tmp, "config.json"),
	}
	all := fixturePayload([]map[string]any{
		{
			"displayCode":   "(2026-2027-1)-A0001001-01",
			"courseCode":    "(2026-2027-1)-A0001001",
			"kcmc":          "已满课程",
			"sksj":          "星期一第1-2节{1-17周}",
			"selectEnabled": true,
			"jxbrl":         "80",
			"jxbrs":         "80",
		},
	})
	mustWriteJSON(t, p.coursePath, all)
	plan, _, err := buildActionPlan(PlanRequest{TargetPayload: all}, p)
	if err != nil {
		t.Fatal(err)
	}
	if !hasValidationMessage(plan.Validation, "已满") {
		t.Fatalf("expected full course issue: %#v", plan.Validation)
	}
	if len(blockingValidationIssues(plan.Validation)) == 0 {
		t.Fatalf("full course must block executable selection: %#v", plan.Validation)
	}
}

func TestBuildActionPlanDiffAndRisks(t *testing.T) {
	tmp := t.TempDir()
	p := paths{
		workspace:        tmp,
		schedulerDir:     tmp,
		killCourseDir:    tmp,
		coursePath:       filepath.Join(tmp, "course.json"),
		personalPath:     filepath.Join(tmp, "personal-schedule.json"),
		livePersonalPath: filepath.Join(tmp, "personal-schedule-live.json"),
		killConfigPath:   filepath.Join(tmp, "config.json"),
		actionPlanPath:   filepath.Join(tmp, "action-plan.json"),
	}
	all := fixturePayload([]map[string]any{
		{"displayCode": "(2026-2027-1)-A0001001-01", "courseCode": "(2026-2027-1)-A0001001", "kcmc": "数学", "jzgxx": "张老师", "sksj": "星期一第1-2节{1-17周}", "xf": "3"},
		{"displayCode": "(2026-2027-1)-A0001001-02", "courseCode": "(2026-2027-1)-A0001001", "kcmc": "数学", "jzgxx": "李老师", "sksj": "星期二第1-2节{1-17周}", "xf": "3"},
		{"displayCode": "(2026-2027-1)-A0002001-01", "courseCode": "(2026-2027-1)-A0002001", "kcmc": "英语", "jzgxx": "王老师", "sksj": "星期三第1-2节{1-17周}", "xf": "2"},
	})
	current := fixturePayload([]map[string]any{
		{"displayCode": "(2026-2027-1)-A0001001-01", "courseCode": "(2026-2027-1)-A0001001", "kcmc": "数学", "jzgxx": "张老师", "sksj": "星期一第1-2节{1-17周}", "xf": "3"},
		{"displayCode": "(2026-2027-1)-A0002001-01", "courseCode": "(2026-2027-1)-A0002001", "kcmc": "英语", "jzgxx": "王老师", "sksj": "星期三第1-2节{1-17周}", "xf": "2"},
	})
	target := fixturePayload([]map[string]any{
		{"displayCode": "(2026-2027-1)-A0001001-02", "courseCode": "(2026-2027-1)-A0001001", "kcmc": "数学", "jzgxx": "李老师", "sksj": "星期二第1-2节{1-17周}", "xf": "3"},
		{"displayCode": "(2026-2027-1)-A0002001-01", "courseCode": "(2026-2027-1)-A0002001", "kcmc": "英语", "jzgxx": "王老师", "sksj": "星期三第1-2节{1-17周}", "xf": "2"},
	})
	mustWriteJSON(t, p.coursePath, all)
	mustWriteJSON(t, p.personalPath, current)
	mustWriteJSON(t, p.livePersonalPath, current)

	plan, warnings, err := buildActionPlan(PlanRequest{TargetPayload: target}, p)
	if err != nil {
		t.Fatal(err)
	}
	if len(warnings) != 0 {
		t.Fatalf("unexpected warnings: %v", warnings)
	}
	if len(plan.Keep) != 1 || plan.Keep[0].DisplayCode != "(2026-2027-1)-A0002001-01" {
		t.Fatalf("bad keep: %#v", plan.Keep)
	}
	if len(plan.Select) != 1 || plan.Select[0].DisplayCode != "(2026-2027-1)-A0001001-02" {
		t.Fatalf("bad select: %#v", plan.Select)
	}
	if len(plan.Drop) != 1 || plan.Drop[0].DisplayCode != "(2026-2027-1)-A0001001-01" {
		t.Fatalf("bad drop: %#v", plan.Drop)
	}
	if len(plan.FallbackGroups) != 1 || len(plan.FallbackGroups[0].Alternatives) != 1 {
		t.Fatalf("bad fallback groups: %#v", plan.FallbackGroups)
	}
	if len(plan.Risks) < 2 {
		t.Fatalf("expected退课风险, got %#v", plan.Risks)
	}
	if len(plan.Explanations) == 0 {
		t.Fatalf("expected plan explanations")
	}
	if len(plan.Validation) != 0 {
		t.Fatalf("unexpected validation issues: %#v", plan.Validation)
	}
}

func TestBuildActionPlanValidationIssues(t *testing.T) {
	tmp := t.TempDir()
	p := paths{
		workspace:      tmp,
		schedulerDir:   tmp,
		killCourseDir:  tmp,
		coursePath:     filepath.Join(tmp, "missing-course.json"),
		personalPath:   filepath.Join(tmp, "personal-schedule.json"),
		killConfigPath: filepath.Join(tmp, "config.json"),
		actionPlanPath: filepath.Join(tmp, "action-plan.json"),
	}
	current := fixturePayload([]map[string]any{
		{"displayCode": "(2026-2027-1)-A0003001-01", "courseCode": "(2026-2027-1)-A0003001", "kcmc": "物理", "jzgxx": "赵老师", "sksj": "星期五第1-2节{1-17周}", "xf": "2"},
	})
	target := fixturePayload([]map[string]any{
		{"displayCode": "(2026-2027-1)-A0001001-01", "courseCode": "(2026-2027-1)-A0001001", "kcmc": "数学", "jzgxx": "张老师", "sksj": "星期一第1-2节{1-17周}", "xf": "3"},
		{"displayCode": "(2026-2027-1)-A0001001-02", "courseCode": "(2026-2027-1)-A0001001", "kcmc": "数学", "jzgxx": "李老师", "sksj": "星期二第1-2节{1-17周}", "xf": "3"},
	})
	mustWriteJSON(t, p.personalPath, current)

	plan, warnings, err := buildActionPlan(PlanRequest{
		TargetPayload: target,
		LockedCodes:   []string{"(2026-2027-1)-A0003001-01"},
	}, p)
	if err != nil {
		t.Fatal(err)
	}
	if len(warnings) == 0 {
		t.Fatalf("expected missing course warning")
	}
	if len(plan.Validation) == 0 {
		t.Fatalf("expected validation issues")
	}
	if !hasValidationMessage(plan.Validation, "同一课程号") {
		t.Fatalf("expected duplicate base-course validation, got %#v", plan.Validation)
	}
	if !hasValidationMessage(plan.Validation, "锁定课程不在目标课表") {
		t.Fatalf("expected locked mismatch validation, got %#v", plan.Validation)
	}
	if len(plan.Drop) != 0 {
		t.Fatalf("locked current course should not be dropped: %#v", plan.Drop)
	}
	if len(plan.Locked) != 1 {
		t.Fatalf("expected locked course to be preserved: %#v", plan.Locked)
	}
}

func TestBuildActionPlanBlocksConflictingTargetConfig(t *testing.T) {
	tmp := t.TempDir()
	p := paths{
		workspace:      tmp,
		schedulerDir:   tmp,
		killCourseDir:  tmp,
		coursePath:     filepath.Join(tmp, "course.json"),
		personalPath:   filepath.Join(tmp, "personal-schedule.json"),
		killConfigPath: filepath.Join(tmp, "config.json"),
	}
	all := fixturePayload([]map[string]any{
		{"displayCode": "(2026-2027-1)-A0001001-01", "courseCode": "(2026-2027-1)-A0001001", "kcmc": "Course A", "sksj": "\u661f\u671f\u4e00\u7b2c1-2\u8282{1-17\u5468}"},
		{"displayCode": "(2026-2027-1)-B0001001-01", "courseCode": "(2026-2027-1)-B0001001", "kcmc": "Course B", "sksj": "\u661f\u671f\u4e00\u7b2c1-2\u8282{1-17\u5468}"},
	})
	mustWriteJSON(t, p.coursePath, all)

	plan, _, err := buildActionPlan(PlanRequest{TargetPayload: all}, p)
	if err != nil {
		t.Fatal(err)
	}
	if len(blockingValidationIssues(plan.Validation)) == 0 {
		t.Fatalf("expected target time conflict to block execution: %#v", plan.Validation)
	}
	if _, _, err := buildKillCourseConfig(plan, p.killConfigPath); err == nil {
		t.Fatal("conflicting target plan must not produce KillCourse config")
	}
}

func TestOddEvenTargetSlotsDoNotConflict(t *testing.T) {
	oddText := "\u661f\u671f\u4e00\u7b2c1-2\u8282{1-17\u5468(\u5355)}"
	evenText := "\u661f\u671f\u4e00\u7b2c1-2\u8282{2-16\u5468(\u53cc)}"
	odd, oddOK, _ := parseCourseSlots(oddText)
	even, evenOK, _ := parseCourseSlots(evenText)
	if !oddOK || !evenOK || slotsConflict(odd, even) {
		t.Fatalf("odd/even slots should parse and remain non-conflicting: odd=%#v even=%#v", odd, even)
	}
}

func TestSectionSuffixesRemainDistinctTeachingClasses(t *testing.T) {
	first := "(2026-2027-1)-A3201150-01A"
	second := "(2026-2027-1)-A3201150-01B"
	if normalizeCode(first) == normalizeCode(second) {
		t.Fatalf("teaching class suffixes must remain distinct: %q / %q", normalizeCode(first), normalizeCode(second))
	}
	if baseCourseCode(first) != baseCourseCode(second) {
		t.Fatalf("teaching class suffixes must share the same base course: %q / %q", baseCourseCode(first), baseCourseCode(second))
	}
}

func TestBuildActionPlanBlocksUnknownTargetTeachingClass(t *testing.T) {
	tmp := t.TempDir()
	p := paths{workspace: tmp, schedulerDir: tmp, coursePath: filepath.Join(tmp, "course.json"), killConfigPath: filepath.Join(tmp, "config.json")}
	mustWriteJSON(t, p.coursePath, fixturePayload([]map[string]any{
		{"displayCode": "(2026-2027-1)-A0001001-01", "courseCode": "(2026-2027-1)-A0001001", "kcmc": "Known Course", "sksj": "\u661f\u671f\u4e00\u7b2c1-2\u8282{1-17\u5468}"},
	}))
	target := fixturePayload([]map[string]any{
		{"displayCode": "(2026-2027-1)-A0001001-02", "courseCode": "(2026-2027-1)-A0001001", "kcmc": "Unknown Section", "sksj": "\u661f\u671f\u4e8c\u7b2c1-2\u8282{1-17\u5468}"},
	})
	plan, _, err := buildActionPlan(PlanRequest{TargetPayload: target}, p)
	if err != nil {
		t.Fatal(err)
	}
	if len(blockingValidationIssues(plan.Validation)) == 0 {
		t.Fatalf("unknown target teaching class should block execution: %#v", plan.Validation)
	}
}

func TestHandlePlanReturnsPreviewWhenConfigIsBlocked(t *testing.T) {
	tmp := t.TempDir()
	schedulerDir := filepath.Join(tmp, "Scheduler")
	killDir := filepath.Join(tmp, "KillCourse")
	if err := os.MkdirAll(schedulerDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(killDir, 0755); err != nil {
		t.Fatal(err)
	}
	all := fixturePayload([]map[string]any{
		{"displayCode": "(2026-2027-1)-A0001001-01", "courseCode": "(2026-2027-1)-A0001001", "kcmc": "Course A", "sksj": "\u661f\u671f\u4e00\u7b2c1-2\u8282{1-17\u5468}"},
		{"displayCode": "(2026-2027-1)-B0001001-01", "courseCode": "(2026-2027-1)-B0001001", "kcmc": "Course B", "sksj": "\u661f\u671f\u4e00\u7b2c1-2\u8282{1-17\u5468}"},
	})
	mustWriteJSON(t, filepath.Join(schedulerDir, "course.json"), all)
	mustWriteJSON(t, filepath.Join(tmp, "agent-settings.json"), AgentSettings{SchedulerDir: schedulerDir, KillCourseDir: killDir})

	oldDir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(tmp); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(oldDir) })

	body, err := json.Marshal(PlanRequest{TargetPayload: all, WriteKillCourseConfig: true})
	if err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(http.MethodPost, "/api/plan", bytes.NewReader(body))
	rec := httptest.NewRecorder()
	handlePlan(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("handlePlan status=%d body=%s", rec.Code, rec.Body.String())
	}
	var response PlanResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}
	if !response.OK || !response.ConfigBlocked || response.GeneratedConfig != nil || response.ConfigPath != "" {
		t.Fatalf("blocked plan should remain preview-only: %#v", response)
	}
	if _, err := os.Stat(filepath.Join(killDir, "config.json")); !os.IsNotExist(err) {
		t.Fatalf("blocked plan should not write config.json, err=%v", err)
	}
}

func TestBuildKillCourseConfigActions(t *testing.T) {
	plan := ActionPlan{
		Term:   "2026-2027-1",
		Select: []Course{{DisplayCode: "(2026-2027-1)-A0001001-02"}},
		Drop:   []Course{{DisplayCode: "(2026-2027-1)-A0001001-01"}},
	}
	cfg, preview, err := buildKillCourseConfig(plan, filepath.Join(t.TempDir(), "missing-config.json"))
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Course["(2026-2027-1)-A0001001-02"] != "1" {
		t.Fatalf("select action missing: %#v", cfg.Course)
	}
	if cfg.Course["(2026-2027-1)-A0001001-01"] != "0" {
		t.Fatalf("drop action missing: %#v", cfg.Course)
	}
	if cfg.Time.XueNian != "2026" || cfg.Time.XueQi != "1" {
		t.Fatalf("bad term: %#v", cfg.Time)
	}
	if preview.NewActionCount != 2 || len(preview.Actions) != 2 {
		t.Fatalf("bad config preview: %#v", preview)
	}
}

func TestBuildKillCourseConfigPreservesExistingSettings(t *testing.T) {
	tmp := t.TempDir()
	configPath := filepath.Join(tmp, "config.json")
	existing := defaultKillCourseConfig("2025-2026-2")
	existing.CasLogin.Username = "24000000"
	existing.CasLogin.Password = "secret"
	existing.Cookies.Enabled = "1"
	existing.Cookies.JSESSIONID = "old-session"
	existing.WaitCourse.Enabled = "1"
	existing.WaitCourse.Interval = 30
	existing.SMTPEmail.Enabled = "1"
	existing.UserAgent = "custom-test-agent"
	existing.ClientBodyConfigEnabled = "1"
	existing.CrossGradeEnabled = "1"
	existing.StartTime = "2026-07-20 12:00:00"
	existing.Course = map[string]string{
		"(2025-2026-2)-OLD0001-01": "1",
	}
	mustWriteJSON(t, configPath, existing)

	plan := ActionPlan{
		Term:   "2026-2027-1",
		Select: []Course{{DisplayCode: "(2026-2027-1)-A0001001-02"}},
		Drop:   []Course{{DisplayCode: "(2026-2027-1)-A0001001-01"}},
	}
	cfg, preview, err := buildKillCourseConfig(plan, configPath)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.CasLogin.Username != "24000000" || cfg.CasLogin.Password != "secret" {
		t.Fatalf("login settings were not preserved: %#v", cfg.CasLogin)
	}
	if cfg.Cookies.JSESSIONID != "old-session" || cfg.WaitCourse.Interval != 30 || cfg.SMTPEmail.Enabled != "1" {
		t.Fatalf("non-course settings were not preserved")
	}
	if cfg.UserAgent != "custom-test-agent" || cfg.ClientBodyConfigEnabled != "1" || cfg.CrossGradeEnabled != "1" {
		t.Fatalf("latest KillCourse settings were not preserved: %#v", cfg)
	}
	if _, ok := cfg.Course["(2025-2026-2)-OLD0001-01"]; ok {
		t.Fatalf("old course actions should be replaced: %#v", cfg.Course)
	}
	if !preview.ExistingConfigFound || preview.OldActionCount != 1 || preview.NewActionCount != 2 {
		t.Fatalf("bad preview counts: %#v", preview)
	}
	if !hasConfigActionStatus(preview.Actions, "(2025-2026-2)-OLD0001-01", "removed") {
		t.Fatalf("expected removed old action: %#v", preview.Actions)
	}
	if !hasConfigActionStatus(preview.Actions, "(2026-2027-1)-A0001001-02", "added") {
		t.Fatalf("expected added new action: %#v", preview.Actions)
	}
}

func TestBuildConfigPreviewRequiresPairedLoginCredentials(t *testing.T) {
	cfg := defaultKillCourseConfig("2026-2027-1")
	cfg.CasLogin.Username = "only-user"
	cfg.Course = map[string]string{"(2026-2027-1)-A0001001-01": "1"}
	preview := buildConfigPreview("config.json", true, cfg, nil)
	if preview.HasCASLogin {
		t.Fatal("partial CAS credentials must not be reported as a complete login")
	}
	cfg.CasLogin.Username = ""
	cfg.CasLogin.Password = "only-password"
	preview = buildConfigPreview("config.json", true, cfg, nil)
	if preview.HasCASLogin {
		t.Fatal("password-only CAS credentials must not be reported as a complete login")
	}
}

func TestConfigFileMatchesGeneratedDetectsNonActionDrift(t *testing.T) {
	tmp := t.TempDir()
	configPath := filepath.Join(tmp, "config.json")
	generated := defaultKillCourseConfig("2026-2027-1")
	generated.CasLogin.Username = "24000000"
	generated.Cookies.JSESSIONID = "session-a"
	generated.Cookies.Route = "route-a"
	generated.WaitCourse.Enabled = "1"
	generated.WaitCourse.Interval = 30
	generated.StartTime = "2026-07-20 12:00:00"
	generated.Course = map[string]string{
		"(2026-2027-1)-A0001001-01": "1",
	}

	cases := []struct {
		name   string
		mutate func(*KillCourseConfig)
	}{
		{name: "account", mutate: func(cfg *KillCourseConfig) { cfg.CasLogin.Username = "24000001" }},
		{name: "cookie", mutate: func(cfg *KillCourseConfig) { cfg.Cookies.JSESSIONID = "session-b" }},
		{name: "wait-course", mutate: func(cfg *KillCourseConfig) { cfg.WaitCourse.Interval = 60 }},
		{name: "start-time", mutate: func(cfg *KillCourseConfig) { cfg.StartTime = "2026-07-20 12:01:00" }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			onDisk := generated
			tc.mutate(&onDisk)
			mustWriteJSON(t, configPath, onDisk)
			exists, matches := configFileMatchesGenerated(configPath, generated)
			if !exists || matches {
				t.Fatalf("config drift must be detected: exists=%v matches=%v", exists, matches)
			}
		})
	}
}

func TestBuildActionPlanBlocksUnparseableCourseTime(t *testing.T) {
	tmp := t.TempDir()
	p := paths{
		workspace:        tmp,
		schedulerDir:     tmp,
		coursePath:       filepath.Join(tmp, "course.json"),
		personalPath:     filepath.Join(tmp, "personal-schedule.json"),
		livePersonalPath: filepath.Join(tmp, "personal-schedule-live.json"),
		killConfigPath:   filepath.Join(tmp, "config.json"),
	}
	target := fixturePayload([]map[string]any{
		{
			"displayCode": "(2026-2027-1)-A0001001-01",
			"courseCode":  "(2026-2027-1)-A0001001",
			"kcmc":        "非法时间课程",
			"sksj":        "invalid course time",
		},
	})

	plan, _, err := buildActionPlan(PlanRequest{TargetPayload: target}, p)
	if err != nil {
		t.Fatal(err)
	}
	if !hasValidationMessage(plan.Validation, "无法解析课程时间") {
		t.Fatalf("expected unparseable course time issue: %#v", plan.Validation)
	}
	if len(blockingValidationIssues(plan.Validation)) == 0 {
		t.Fatalf("unparseable course time must block execution: %#v", plan.Validation)
	}
}

func TestBuildExecutionReadiness(t *testing.T) {
	tmp := t.TempDir()
	p := paths{workspace: tmp, killCourseDir: tmp}
	plan := ActionPlan{
		Term: "2026-2027-1",
		Drop: []Course{{DisplayCode: "(2026-2027-1)-A0001001-01"}},
	}
	markPlanAsLive(t, &p, &plan, plan.Drop)
	cfg := defaultKillCourseConfig(plan.Term)
	cfg.CasLogin.Username = "24000000"
	cfg.CasLogin.Password = "secret"
	cfg.StartTime = "2026-07-20 12:00:00"
	cfg.Course = map[string]string{
		"(2026-2027-1)-A0001001-01": "0",
	}
	preview := buildConfigPreview(filepath.Join(tmp, "config.json"), true, cfg, map[string]string{})

	readiness := buildExecutionReadiness(plan, cfg, preview, p)
	if !readiness.Ready || !readiness.CanExecute {
		t.Fatalf("expected readiness despite退课 warning: %#v", readiness)
	}
	if !hasReadinessMessage(readiness.Checks, "退课动作") {
		t.Fatalf("expected drop warning: %#v", readiness.Checks)
	}
}

func TestBuildExecutionReadinessBlocksBadTerm(t *testing.T) {
	tmp := t.TempDir()
	p := paths{killCourseDir: tmp}
	plan := ActionPlan{Term: "2026-2027-1"}
	cfg := defaultKillCourseConfig(plan.Term)
	cfg.CasLogin.Username = "24000000"
	cfg.StartTime = "2026-07-20 12:00:00"
	cfg.Course = map[string]string{
		"(2025-2026-2)-A0001001-01": "1",
	}
	preview := buildConfigPreview(filepath.Join(tmp, "config.json"), true, cfg, map[string]string{})

	readiness := buildExecutionReadiness(plan, cfg, preview, p)
	if readiness.Ready || readiness.CanExecute {
		t.Fatalf("expected bad term to block readiness: %#v", readiness)
	}
	if !hasReadinessMessage(readiness.Checks, "学年学期一致") {
		t.Fatalf("expected term mismatch check: %#v", readiness.Checks)
	}
}

func TestBuildExecutionDryRun(t *testing.T) {
	tmp := t.TempDir()
	p := paths{
		workspace:        tmp,
		killCourseDir:    tmp,
		killConfigPath:   filepath.Join(tmp, "config.json"),
		livePersonalPath: filepath.Join(tmp, "personal-schedule-live.json"),
	}
	if err := os.MkdirAll(filepath.Join(tmp, "cmd", "HDU-KillCourse"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(tmp, "cmd", "HDU-KillCourse", "main.go"), []byte("package main\nfunc main(){}\n"), 0644); err != nil {
		t.Fatal(err)
	}
	cfg := defaultKillCourseConfig("2026-2027-1")
	cfg.CasLogin.Username = "24000000"
	cfg.StartTime = "2026-07-20 12:00:00"
	cfg.Course = map[string]string{
		"(2026-2027-1)-A0001001-02": "1",
		"(2026-2027-1)-A0001001-01": "0",
	}
	mustWriteJSON(t, p.killConfigPath, cfg)
	plan := ActionPlan{
		Term:   "2026-2027-1",
		Select: []Course{{DisplayCode: "(2026-2027-1)-A0001001-02"}},
		Drop:   []Course{{DisplayCode: "(2026-2027-1)-A0001001-01"}},
	}
	markPlanAsLive(t, &p, &plan, plan.Drop)

	dryRun, err := buildExecutionDryRun(DryRunRequest{Plan: plan, GeneratedConfig: &cfg}, p)
	if err != nil {
		t.Fatal(err)
	}
	if !dryRun.Ready || !dryRun.CanExecute || !dryRun.EntryFound {
		t.Fatalf("expected dry-run to be executable: %#v", dryRun)
	}
	if dryRun.ActionCounts.Select != 1 || dryRun.ActionCounts.Drop != 1 || !dryRun.HasDropRisk {
		t.Fatalf("bad action counts: %#v", dryRun.ActionCounts)
	}
	if !strings.Contains(dryRun.Command, "go run") {
		t.Fatalf("expected go run fallback command, got %q", dryRun.Command)
	}
	if !hasExecutionEvent(dryRun.Events, "没有真实执行") {
		t.Fatalf("expected safety event: %#v", dryRun.Events)
	}
}

func TestBuildExecutionDryRunBlocksDropWithoutLiveSchedule(t *testing.T) {
	tmp := t.TempDir()
	p := paths{
		workspace:      tmp,
		killCourseDir:  tmp,
		killConfigPath: filepath.Join(tmp, "config.json"),
	}
	if err := os.WriteFile(filepath.Join(tmp, "HDU-KillCourse.exe"), []byte("fake"), 0644); err != nil {
		t.Fatal(err)
	}
	cfg := defaultKillCourseConfig("2026-2027-1")
	cfg.CasLogin.Username = "24000000"
	cfg.StartTime = "2026-07-20 12:00:00"
	cfg.Course = map[string]string{
		"(2026-2027-1)-A0001001-01": "0",
	}
	mustWriteJSON(t, p.killConfigPath, cfg)
	plan := ActionPlan{
		Term: "2026-2027-1",
		Drop: []Course{{DisplayCode: "(2026-2027-1)-A0001001-01"}},
	}

	dryRun, err := buildExecutionDryRun(DryRunRequest{Plan: plan, GeneratedConfig: &cfg}, p)
	if err != nil {
		t.Fatal(err)
	}
	if dryRun.CanExecute {
		t.Fatalf("drop plan without live schedule should be blocked: %#v", dryRun)
	}
	if !hasExecutionEvent(dryRun.Events, "Drop plan blocked") {
		t.Fatalf("expected live schedule block event: %#v", dryRun.Events)
	}
}

func TestBuildExecutionDryRunBlocksMissingEntry(t *testing.T) {
	tmp := t.TempDir()
	p := paths{
		workspace:      tmp,
		killCourseDir:  filepath.Join(tmp, "missing-killcourse"),
		killConfigPath: filepath.Join(tmp, "config.json"),
	}
	cfg := defaultKillCourseConfig("2026-2027-1")
	cfg.CasLogin.Username = "24000000"
	cfg.StartTime = "2026-07-20 12:00:00"
	cfg.Course = map[string]string{
		"(2026-2027-1)-A0001001-02": "1",
	}
	mustWriteJSON(t, p.killConfigPath, cfg)
	plan := ActionPlan{
		Term:   "2026-2027-1",
		Select: []Course{{DisplayCode: "(2026-2027-1)-A0001001-02"}},
	}

	dryRun, err := buildExecutionDryRun(DryRunRequest{Plan: plan, GeneratedConfig: &cfg}, p)
	if err != nil {
		t.Fatal(err)
	}
	if dryRun.Ready || dryRun.CanExecute || dryRun.EntryFound {
		t.Fatalf("missing KillCourse entry should block dry-run: %#v", dryRun)
	}
	if !hasExecutionEvent(dryRun.Events, "未找到 KillCourse") {
		t.Fatalf("expected missing entry event: %#v", dryRun.Events)
	}
}

func TestBuildExecutionDryRunBlocksUnwrittenConfig(t *testing.T) {
	tmp := t.TempDir()
	p := paths{
		workspace:        tmp,
		killCourseDir:    tmp,
		killConfigPath:   filepath.Join(tmp, "config.json"),
		livePersonalPath: filepath.Join(tmp, "personal-schedule-live.json"),
	}
	if err := os.WriteFile(filepath.Join(tmp, "HDU-KillCourse.exe"), []byte("fake"), 0644); err != nil {
		t.Fatal(err)
	}
	cfg := defaultKillCourseConfig("2026-2027-1")
	cfg.CasLogin.Username = "24000000"
	cfg.StartTime = "2026-07-20 12:00:00"
	cfg.Course = map[string]string{
		"(2026-2027-1)-A0001001-02": "1",
	}
	plan := ActionPlan{
		Term:   "2026-2027-1",
		Select: []Course{{DisplayCode: "(2026-2027-1)-A0001001-02"}},
	}

	dryRun, err := buildExecutionDryRun(DryRunRequest{Plan: plan, GeneratedConfig: &cfg}, p)
	if err != nil {
		t.Fatal(err)
	}
	if dryRun.Ready || dryRun.CanExecute {
		t.Fatalf("unwritten config should block dry-run: %#v", dryRun)
	}
	if !hasExecutionEvent(dryRun.Events, "尚未写入磁盘") {
		t.Fatalf("expected unwritten config event: %#v", dryRun.Events)
	}
}

func TestBuildExecutionAuthorization(t *testing.T) {
	tmp := t.TempDir()
	p := paths{
		workspace:        tmp,
		killCourseDir:    tmp,
		killConfigPath:   filepath.Join(tmp, "config.json"),
		livePersonalPath: filepath.Join(tmp, "personal-schedule-live.json"),
	}
	if err := os.MkdirAll(filepath.Join(tmp, "cmd", "HDU-KillCourse"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(tmp, "cmd", "HDU-KillCourse", "main.go"), []byte("package main\nfunc main(){}\n"), 0644); err != nil {
		t.Fatal(err)
	}
	cfg := defaultKillCourseConfig("2026-2027-1")
	cfg.CasLogin.Username = "24000000"
	cfg.StartTime = "2026-07-20 12:00:00"
	cfg.Course = map[string]string{
		"(2026-2027-1)-A0001001-02": "1",
		"(2026-2027-1)-A0001001-01": "0",
	}
	mustWriteJSON(t, p.killConfigPath, cfg)
	plan := ActionPlan{
		Term:   "2026-2027-1",
		Select: []Course{{DisplayCode: "(2026-2027-1)-A0001001-02"}},
		Drop:   []Course{{DisplayCode: "(2026-2027-1)-A0001001-01"}},
	}
	markPlanAsLive(t, &p, &plan, plan.Drop)

	authorization, err := buildExecutionAuthorization(ExecutionAuthorizationRequest{
		Plan:               plan,
		GeneratedConfig:    &cfg,
		ConfirmationPhrase: "我确认退课风险并准备执行",
	}, p)
	if err != nil {
		t.Fatal(err)
	}
	if !authorization.Authorized || authorization.TicketID == "" || authorization.PlanHash == "" || authorization.ConfigHash == "" {
		t.Fatalf("bad authorization: %#v", authorization)
	}
	if !authorization.DropRiskAccepted || authorization.ActionCounts.Drop != 1 {
		t.Fatalf("drop risk should be accepted in ticket: %#v", authorization)
	}
}

func TestBuildExecutionAuthorizationRejectsWrongPhrase(t *testing.T) {
	tmp := t.TempDir()
	p := paths{
		workspace:      tmp,
		killCourseDir:  tmp,
		killConfigPath: filepath.Join(tmp, "config.json"),
	}
	if err := os.WriteFile(filepath.Join(tmp, "HDU-KillCourse.exe"), []byte("fake"), 0644); err != nil {
		t.Fatal(err)
	}
	cfg := defaultKillCourseConfig("2026-2027-1")
	cfg.CasLogin.Username = "24000000"
	cfg.StartTime = "2026-07-20 12:00:00"
	cfg.Course = map[string]string{
		"(2026-2027-1)-A0001001-02": "1",
	}
	mustWriteJSON(t, p.killConfigPath, cfg)
	plan := ActionPlan{
		Term:   "2026-2027-1",
		Select: []Course{{DisplayCode: "(2026-2027-1)-A0001001-02"}},
	}

	_, err := buildExecutionAuthorization(ExecutionAuthorizationRequest{
		Plan:               plan,
		GeneratedConfig:    &cfg,
		ConfirmationPhrase: "我确认退课风险并准备执行",
	}, p)
	if err == nil {
		t.Fatalf("expected wrong confirmation phrase to be rejected")
	}
}

func TestBuildExecutionPackage(t *testing.T) {
	tmp := t.TempDir()
	p := paths{
		workspace:      tmp,
		killCourseDir:  tmp,
		killConfigPath: filepath.Join(tmp, "config.json"),
		approvalPath:   filepath.Join(tmp, "execution-approval.json"),
		runBatchPath:   filepath.Join(tmp, "run-killcourse.bat"),
		runbookPath:    filepath.Join(tmp, "execution-runbook.md"),
		manifestPath:   filepath.Join(tmp, "execution-package.json"),
	}
	if err := os.MkdirAll(filepath.Join(tmp, "cmd", "HDU-KillCourse"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(tmp, "cmd", "HDU-KillCourse", "main.go"), []byte("package main\nfunc main(){}\n"), 0644); err != nil {
		t.Fatal(err)
	}
	cfg := defaultKillCourseConfig("2026-2027-1")
	cfg.CasLogin.Username = "24000000"
	cfg.StartTime = "2026-07-20 12:00:00"
	cfg.Course = map[string]string{
		"(2026-2027-1)-A0001001-02": "1",
	}
	mustWriteJSON(t, p.killConfigPath, cfg)
	plan := ActionPlan{
		Term:   "2026-2027-1",
		Select: []Course{{DisplayCode: "(2026-2027-1)-A0001001-02"}},
	}
	auth, err := buildExecutionAuthorization(ExecutionAuthorizationRequest{
		Plan:               plan,
		GeneratedConfig:    &cfg,
		ConfirmationPhrase: "我确认准备执行",
	}, p)
	if err != nil {
		t.Fatal(err)
	}
	mustWriteJSON(t, p.approvalPath, auth)
	pkg, err := buildExecutionPackage(ExecutionPackageRequest{
		Plan:            plan,
		GeneratedConfig: &cfg,
		Authorization:   auth,
	}, p)
	if err != nil {
		t.Fatal(err)
	}
	if !pkg.Ready || pkg.BatchPath != p.runBatchPath || pkg.RunbookPath != p.runbookPath || pkg.ManifestPath != p.manifestPath {
		t.Fatalf("bad execution package: %#v", pkg)
	}
	if err := writeExecutionPackageFiles(pkg, auth, p); err != nil {
		t.Fatal(err)
	}
	bat, err := os.ReadFile(p.runBatchPath)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(bat), "go run ./cmd/HDU-KillCourse") ||
		!strings.Contains(string(bat), "KillCourse 启动后会等待一次 Enter") ||
		!strings.Contains(string(bat), "即将启动 KillCourse") {
		t.Fatalf("bad batch content: %s", string(bat))
	}
	runbook, err := os.ReadFile(p.runbookPath)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(runbook), auth.TicketID) {
		t.Fatalf("runbook should include ticket id: %s", string(runbook))
	}
	manifest, err := os.ReadFile(p.manifestPath)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(manifest), `"manualRunOnly": true`) || !strings.Contains(string(manifest), auth.TicketID) {
		t.Fatalf("bad manifest content: %s", string(manifest))
	}
}

func TestBuildExecutionPackageRejectsUnpersistedAuthorization(t *testing.T) {
	tmp := t.TempDir()
	p := paths{
		workspace:      tmp,
		killCourseDir:  tmp,
		killConfigPath: filepath.Join(tmp, "config.json"),
		approvalPath:   filepath.Join(tmp, "execution-approval.json"),
	}
	if err := os.WriteFile(filepath.Join(tmp, "HDU-KillCourse.exe"), []byte("fake"), 0644); err != nil {
		t.Fatal(err)
	}
	cfg := defaultKillCourseConfig("2026-2027-1")
	cfg.CasLogin.Username = "24000000"
	cfg.StartTime = "2026-07-20 12:00:00"
	cfg.Course = map[string]string{
		"(2026-2027-1)-A0001001-02": "1",
	}
	mustWriteJSON(t, p.killConfigPath, cfg)
	plan := ActionPlan{
		Term:   "2026-2027-1",
		Select: []Course{{DisplayCode: "(2026-2027-1)-A0001001-02"}},
	}
	auth, err := buildExecutionAuthorization(ExecutionAuthorizationRequest{
		Plan:               plan,
		GeneratedConfig:    &cfg,
		ConfirmationPhrase: "我确认准备执行",
	}, p)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := buildExecutionPackage(ExecutionPackageRequest{
		Plan:            plan,
		GeneratedConfig: &cfg,
		Authorization:   auth,
	}, p); err == nil {
		t.Fatal("unpersisted authorization must not generate an execution package")
	}
}

func TestBuildExecutionPackageRejectsExpiredAuthorization(t *testing.T) {
	tmp := t.TempDir()
	p := paths{
		workspace:      tmp,
		killCourseDir:  tmp,
		killConfigPath: filepath.Join(tmp, "config.json"),
		approvalPath:   filepath.Join(tmp, "execution-approval.json"),
	}
	if err := os.WriteFile(filepath.Join(tmp, "HDU-KillCourse.exe"), []byte("fake"), 0644); err != nil {
		t.Fatal(err)
	}
	cfg := defaultKillCourseConfig("2026-2027-1")
	cfg.CasLogin.Username = "24000000"
	cfg.StartTime = "2026-07-20 12:00:00"
	cfg.Course = map[string]string{
		"(2026-2027-1)-A0001001-02": "1",
	}
	mustWriteJSON(t, p.killConfigPath, cfg)
	plan := ActionPlan{
		Term:   "2026-2027-1",
		Select: []Course{{DisplayCode: "(2026-2027-1)-A0001001-02"}},
	}
	auth, err := buildExecutionAuthorization(ExecutionAuthorizationRequest{
		Plan:               plan,
		GeneratedConfig:    &cfg,
		ConfirmationPhrase: "我确认准备执行",
	}, p)
	if err != nil {
		t.Fatal(err)
	}
	mustWriteJSON(t, p.approvalPath, auth)
	auth.ExpiresAt = time.Now().Add(-time.Minute).Format(time.RFC3339)
	_, err = buildExecutionPackage(ExecutionPackageRequest{
		Plan:            plan,
		GeneratedConfig: &cfg,
		Authorization:   auth,
	}, p)
	if err == nil {
		t.Fatalf("expected expired authorization to be rejected")
	}
}

func TestBuildExecutionLogParsesKillCourseAppLog(t *testing.T) {
	tmp := t.TempDir()
	logDir := filepath.Join(tmp, "log_files")
	if err := os.MkdirAll(logDir, 0755); err != nil {
		t.Fatal(err)
	}
	appLog := strings.Join([]string{
		"2026/07/08 12:00:00.000000 [INFO] 正在处理课程: (2026-2027-1)-A0001001-02",
		"2026/07/08 12:00:00.100000 [INFO] 课程名称: 数学",
		"2026/07/08 12:00:00.200000 [INFO] 上课时间: 星期一第1-2节",
		"2026/07/08 12:00:01.000000 [INFO] 选课成功",
		"2026/07/08 12:00:02.000000 [INFO] 正在处理课程: (2026-2027-1)-A0002001-01",
		"2026/07/08 12:00:03.000000 [ERROR] 选课失败: 人数可能已满",
		"2026/07/08 12:00:04.000000 [INFO] 正在处理课程: (2026-2027-1)-A0003001-01",
		"2026/07/08 12:00:05.000000 [INFO] 退课成功(可能？)",
		"",
	}, "\n")
	if err := os.WriteFile(filepath.Join(logDir, "app.log"), []byte(appLog), 0644); err != nil {
		t.Fatal(err)
	}
	cfg := defaultKillCourseConfig("2026-2027-1")
	cfg.Course = map[string]string{
		"(2026-2027-1)-A0001001-02": "1",
		"(2026-2027-1)-A0002001-01": "1",
		"(2026-2027-1)-A0003001-01": "0",
	}
	plan := ActionPlan{
		Term: "2026-2027-1",
		Select: []Course{
			{DisplayCode: "(2026-2027-1)-A0001001-02"},
			{DisplayCode: "(2026-2027-1)-A0002001-01"},
		},
		Drop: []Course{{DisplayCode: "(2026-2027-1)-A0003001-01"}},
	}

	executionLog, err := buildExecutionLog(ExecutionLogRequest{Plan: plan, GeneratedConfig: &cfg}, paths{killCourseDir: tmp})
	if err != nil {
		t.Fatal(err)
	}
	if executionLog.Summary.Total != 3 || executionLog.Summary.Success != 2 || executionLog.Summary.Failed != 1 {
		t.Fatalf("bad summary: %#v", executionLog.Summary)
	}
	if executionLog.Items[0].Action != "select" || executionLog.Items[0].Status != "success" || executionLog.Items[0].CourseName != "数学" {
		t.Fatalf("bad first item: %#v", executionLog.Items[0])
	}
	if executionLog.Items[1].FailureType != "full" {
		t.Fatalf("expected full failure: %#v", executionLog.Items[1])
	}
	if executionLog.Items[2].Action != "drop" || executionLog.Items[2].Status != "success" {
		t.Fatalf("bad drop item: %#v", executionLog.Items[2])
	}
}

func TestBuildExecutionLogKeepsRunningItem(t *testing.T) {
	tmp := t.TempDir()
	logDir := filepath.Join(tmp, "log_files")
	if err := os.MkdirAll(logDir, 0755); err != nil {
		t.Fatal(err)
	}
	appLog := "2026/07/08 12:00:00.000000 [INFO] 正在处理课程: (2026-2027-1)-A0001001-02\n"
	if err := os.WriteFile(filepath.Join(logDir, "app.log"), []byte(appLog), 0644); err != nil {
		t.Fatal(err)
	}
	executionLog, err := buildExecutionLog(ExecutionLogRequest{}, paths{killCourseDir: tmp})
	if err != nil {
		t.Fatal(err)
	}
	if executionLog.Summary.Pending != 1 || executionLog.Items[0].Status != "running" {
		t.Fatalf("expected running item: %#v", executionLog)
	}
}

func TestBuildFallbackRecommendationsRanksCompatibleAlternatives(t *testing.T) {
	tmp := t.TempDir()
	p := paths{coursePath: filepath.Join(tmp, "course.json")}
	all := fixturePayload([]map[string]any{
		{"displayCode": "(2026-2027-1)-A0001001-01", "courseCode": "(2026-2027-1)-A0001001", "courseName": "Math", "teacher": "T1", "timeText": "星期一第1-2节{1-17周}", "credits": "3"},
		{"displayCode": "(2026-2027-1)-A0001001-02", "courseCode": "(2026-2027-1)-A0001001", "courseName": "Math", "teacher": "T2", "timeText": "星期一第3-4节{1-17周}", "credits": "3"},
		{"displayCode": "(2026-2027-1)-A0001001-03", "courseCode": "(2026-2027-1)-A0001001", "courseName": "Math", "teacher": "T1", "timeText": "星期二第3-4节{1-17周}", "credits": "3"},
		{"displayCode": "(2026-2027-1)-B0002001-01", "courseCode": "(2026-2027-1)-B0002001", "courseName": "English", "teacher": "T3", "timeText": "星期一第3-4节{1-17周}", "credits": "2"},
	})
	mustWriteJSON(t, p.coursePath, all)
	plan := ActionPlan{
		Term: "2026-2027-1",
		Target: []Course{
			{DisplayCode: "(2026-2027-1)-A0001001-01", GroupID: "(2026-2027-1)-A0001001", CourseName: "Math", Teacher: "T1", TimeText: "星期一第1-2节{1-17周}"},
			{DisplayCode: "(2026-2027-1)-B0002001-01", GroupID: "(2026-2027-1)-B0002001", CourseName: "English", Teacher: "T3", TimeText: "星期一第3-4节{1-17周}"},
		},
		Select: []Course{{DisplayCode: "(2026-2027-1)-A0001001-01", GroupID: "(2026-2027-1)-A0001001", CourseName: "Math", Teacher: "T1", TimeText: "星期一第1-2节{1-17周}"}},
		Keep:   []Course{{DisplayCode: "(2026-2027-1)-B0002001-01", GroupID: "(2026-2027-1)-B0002001", CourseName: "English", Teacher: "T3", TimeText: "星期一第3-4节{1-17周}"}},
	}
	log := ExecutionLog{Items: []ExecutionLogItem{{
		CourseCode:  "(2026-2027-1)-A0001001-01",
		CourseName:  "Math",
		Action:      "select",
		Status:      "failed",
		FailureType: "full",
		Message:     "full",
	}}}

	recs, err := buildFallbackRecommendations(FallbackRecommendationRequest{Plan: plan, ExecutionLog: log}, p)
	if err != nil {
		t.Fatal(err)
	}
	if recs.Summary.FailedSelectCount != 1 || recs.Summary.WithOptions != 1 || len(recs.Items) != 1 {
		t.Fatalf("bad summary: %#v", recs)
	}
	options := recs.Items[0].Options
	if len(options) != 2 {
		t.Fatalf("expected two alternatives: %#v", options)
	}
	if options[0].Course.DisplayCode != "(2026-2027-1)-A0001001-03" || !options[0].TimeCompatible || !options[0].SameTeacher {
		t.Fatalf("expected compatible same-teacher option first: %#v", options[0])
	}
	if options[1].Course.DisplayCode != "(2026-2027-1)-A0001001-02" || options[1].TimeCompatible || len(options[1].Conflicts) != 1 {
		t.Fatalf("expected conflicting option second: %#v", options[1])
	}
}

func TestBuildFallbackRecommendationsReportsNoAlternatives(t *testing.T) {
	tmp := t.TempDir()
	p := paths{coursePath: filepath.Join(tmp, "course.json")}
	all := fixturePayload([]map[string]any{
		{"displayCode": "(2026-2027-1)-A0001001-01", "courseCode": "(2026-2027-1)-A0001001", "courseName": "Math", "teacher": "T1", "timeText": "星期一第1-2节{1-17周}"},
	})
	mustWriteJSON(t, p.coursePath, all)
	plan := ActionPlan{
		Term:   "2026-2027-1",
		Target: []Course{{DisplayCode: "(2026-2027-1)-A0001001-01", GroupID: "(2026-2027-1)-A0001001", CourseName: "Math", Teacher: "T1", TimeText: "星期一第1-2节{1-17周}"}},
		Select: []Course{{DisplayCode: "(2026-2027-1)-A0001001-01", GroupID: "(2026-2027-1)-A0001001", CourseName: "Math", Teacher: "T1", TimeText: "星期一第1-2节{1-17周}"}},
	}
	log := ExecutionLog{Items: []ExecutionLogItem{{
		CourseCode:  "(2026-2027-1)-A0001001-01",
		CourseName:  "Math",
		Action:      "select",
		Status:      "failed",
		FailureType: "full",
	}}}

	recs, err := buildFallbackRecommendations(FallbackRecommendationRequest{Plan: plan, ExecutionLog: log}, p)
	if err != nil {
		t.Fatal(err)
	}
	if recs.Summary.WithoutOptions != 1 || len(recs.Items) != 1 || len(recs.Items[0].Options) != 0 {
		t.Fatalf("expected no alternatives: %#v", recs)
	}
}

func TestDiscoverPathsUsesSettingsOverrides(t *testing.T) {
	tmp := t.TempDir()
	workspace := filepath.Join(tmp, "HDU-Smart-Course-Agent")
	schedulerDir := filepath.Join(tmp, "custom-scheduler")
	killDir := filepath.Join(tmp, "custom-killcourse")
	for _, dir := range []string{workspace, schedulerDir, killDir} {
		if err := os.MkdirAll(dir, 0755); err != nil {
			t.Fatal(err)
		}
	}
	oldWD, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = os.Chdir(oldWD)
	})
	if err := os.Chdir(workspace); err != nil {
		t.Fatal(err)
	}
	mustWriteJSON(t, filepath.Join(workspace, "agent-settings.json"), AgentSettings{
		SchedulerDir:  schedulerDir,
		KillCourseDir: killDir,
	})

	p := discoverPaths()
	if p.schedulerDir != schedulerDir {
		t.Fatalf("expected scheduler override %q, got %q", schedulerDir, p.schedulerDir)
	}
	if p.killCourseDir != killDir {
		t.Fatalf("expected killcourse override %q, got %q", killDir, p.killCourseDir)
	}
	if p.coursePath != filepath.Join(schedulerDir, "course.json") {
		t.Fatalf("bad course path: %q", p.coursePath)
	}
	if p.killConfigPath != filepath.Join(killDir, "config.json") {
		t.Fatalf("bad config path: %q", p.killConfigPath)
	}
}

func TestDiscoverPathsFindsGrandparentSiblingKillCourse(t *testing.T) {
	tmp := t.TempDir()
	schedulerDir := filepath.Join(tmp, "HDU-Auto-Scheduling-Script")
	workspace := filepath.Join(schedulerDir, "HDU-Smart-Course-Agent")
	killDir := filepath.Join(tmp, "HDU-KillCourse-main", "HDU-KillCourse-main")
	for _, dir := range []string{workspace, schedulerDir, killDir} {
		if err := os.MkdirAll(dir, 0755); err != nil {
			t.Fatal(err)
		}
	}
	mustWriteJSON(t, filepath.Join(schedulerDir, "course.json"), fixturePayload([]map[string]any{
		{"displayCode": "(2026-2027-1)-A0001001-01", "courseCode": "(2026-2027-1)-A0001001", "kcmc": "数学"},
	}))
	if err := os.WriteFile(filepath.Join(killDir, "config.example.json"), []byte("{}\n"), 0644); err != nil {
		t.Fatal(err)
	}
	oldWD, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = os.Chdir(oldWD)
	})
	if err := os.Chdir(workspace); err != nil {
		t.Fatal(err)
	}

	p := discoverPaths()
	if p.schedulerDir != schedulerDir {
		t.Fatalf("expected scheduler dir %q, got %q", schedulerDir, p.schedulerDir)
	}
	if p.killCourseDir != killDir {
		t.Fatalf("expected grandparent KillCourse dir %q, got %q", killDir, p.killCourseDir)
	}
}

func TestValidateAgentSettingsRejectsMissingDir(t *testing.T) {
	err := validateAgentSettings(AgentSettings{
		SchedulerDir: filepath.Join(t.TempDir(), "missing"),
	})
	if err == nil {
		t.Fatalf("expected missing scheduler dir to be rejected")
	}
}

func TestValidateAgentSettingsRejectsUnknownKillCourseDir(t *testing.T) {
	tmp := t.TempDir()
	err := validateAgentSettings(AgentSettings{KillCourseDir: tmp})
	if err == nil {
		t.Fatalf("expected unknown KillCourse dir to be rejected")
	}
}

func TestValidateAgentSettingsAcceptsKillCourseEntry(t *testing.T) {
	tmp := t.TempDir()
	entry := filepath.Join(tmp, "cmd", "HDU-KillCourse")
	if err := os.MkdirAll(entry, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(entry, "main.go"), []byte("package main\nfunc main(){}\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := validateAgentSettings(AgentSettings{KillCourseDir: tmp}); err != nil {
		t.Fatalf("expected valid KillCourse dir: %v", err)
	}
}

func mustWriteJSON(t *testing.T, file string, value any) {
	t.Helper()
	if err := writeJSONFile(file, value); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(file); err != nil {
		t.Fatal(err)
	}
}

func setupAgentHTTPWorkspace(t *testing.T, payload CoursePayload) paths {
	t.Helper()
	tmp := t.TempDir()
	schedulerDir := filepath.Join(tmp, "Scheduler")
	killDir := filepath.Join(tmp, "KillCourse")
	if err := os.MkdirAll(schedulerDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(killDir, 0755); err != nil {
		t.Fatal(err)
	}
	mustWriteJSON(t, filepath.Join(schedulerDir, "course.json"), payload)
	mustWriteJSON(t, filepath.Join(tmp, "agent-settings.json"), AgentSettings{SchedulerDir: schedulerDir, KillCourseDir: killDir})
	oldDir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(tmp); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(oldDir) })
	return discoverPaths()
}

func markPlanAsLive(t *testing.T, p *paths, plan *ActionPlan, current []Course) {
	t.Helper()
	if p.livePersonalPath == "" {
		p.livePersonalPath = filepath.Join(p.workspace, "personal-schedule-live.json")
	}
	payload := CoursePayload{
		SchemaVersion: schemaVersion,
		Source:        "test-live-schedule",
		Term:          plan.Term,
		Items:         make([]map[string]any, 0, len(current)),
	}
	for _, course := range current {
		payload.Items = append(payload.Items, map[string]any{
			"displayCode": course.DisplayCode,
			"courseCode":  course.RawCourseCode,
			"groupId":     course.GroupID,
			"courseName":  firstNonEmpty(course.CourseName, "Test Course"),
			"teacher":     course.Teacher,
			"timeText":    course.TimeText,
			"credits":     course.Credits,
		})
	}
	mustWriteJSON(t, p.livePersonalPath, payload)
	normalized, _, err := loadCourses(p.livePersonalPath)
	if err != nil {
		t.Fatal(err)
	}
	plan.CurrentSource = "live"
	plan.CurrentHash = scheduleHash(normalized)
}

func hasValidationMessage(items []ValidationIssue, part string) bool {
	for _, item := range items {
		if strings.Contains(item.Message, part) {
			return true
		}
	}
	return false
}

func hasConfigActionStatus(items []ConfigActionPreview, code string, status string) bool {
	for _, item := range items {
		if item.Code == code && item.Status == status {
			return true
		}
	}
	return false
}

func hasReadinessMessage(items []ReadinessCheck, part string) bool {
	for _, item := range items {
		if strings.Contains(item.Message, part) {
			return true
		}
	}
	return false
}

func hasExecutionEvent(items []ExecutionEvent, part string) bool {
	for _, item := range items {
		if strings.Contains(item.Message, part) {
			return true
		}
	}
	return false
}

func TestNormalizePayloadMergesSameTeachingClassRows(t *testing.T) {
	payload := CoursePayload{Items: []map[string]any{
		{"jxb_id": "class-1", "kcmc": "操作系统", "jxbmc": "(2026-2027-1)-A0503030-04", "sksj": "星期二第3-4节{1-17周}", "jxdd": "第3教研楼215"},
		{"jxb_id": "class-1", "kcmc": "操作系统", "jxbmc": "(2026-2027-1)-A0503030-04", "sksj": "星期五第1-2节{1-17周}", "jxdd": "第7教研楼北308"},
	}}
	courses, normalized, err := normalizePayload(payload)
	if err != nil {
		t.Fatalf("normalizePayload() error = %v", err)
	}
	if len(courses) != 1 || len(normalized.Items) != 1 {
		t.Fatalf("courses=%d items=%d, want 1 and 1", len(courses), len(normalized.Items))
	}
	if !strings.Contains(courses[0].TimeText, "星期二") || !strings.Contains(courses[0].TimeText, "星期五") {
		t.Fatalf("TimeText not combined: %q", courses[0].TimeText)
	}
}
