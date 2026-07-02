package main

import (
	"net/http"
	"net/http/httptest"
	neturl "net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
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

func TestServeExporterStatic(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/exporter/", nil)
	rec := httptest.NewRecorder()

	serveExporterStatic(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("serveExporterStatic status = %d, want %d", rec.Code, http.StatusOK)
	}
	if body := rec.Body.String(); !strings.Contains(body, "/exporter/main.js") {
		t.Fatalf("exporter page did not use unified /exporter assets")
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
