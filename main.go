package main

import (
	"embed"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	neturl "net/url"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"hdu-scheduler/school"
)

//go:embed *.html *.css *.js cmd/course-exporter/web/*
var webFS embed.FS

const addr = "127.0.0.1:6789"
const maxImportBytes = 50 << 20
const maxExportRequestBytes = 1 << 20

type appState struct {
	service *school.Service
}

func main() {
	state := &appState{service: school.NewService()}

	mux := http.NewServeMux()
	mux.HandleFunc("/", serveStatic)
	mux.HandleFunc("/exporter/", serveExporterStatic)
	mux.HandleFunc("/api/status", handleStatus)
	mux.HandleFunc("/api/course", handleCourse)
	mux.HandleFunc("/api/personal-schedule", handlePersonalSchedule)
	mux.HandleFunc("/api/bootstrap/import", handleImport)
	mux.HandleFunc("/api/export", handleExport(state))
	mux.HandleFunc("/api/export/status", handleExportStatus(state))
	mux.HandleFunc("/api/export/open-output", handleOpenOutput(state))

	if os.Getenv("HDU_NO_BROWSER") != "1" {
		go openBrowser("http://" + addr + "/")
	}

	fmt.Println("HDU Auto Scheduling Assistant running at http://" + addr)
	server := &http.Server{
		Addr:              addr,
		Handler:           withLocalCORS(mux, "6789"),
		ReadHeaderTimeout: 5 * time.Second,
	}
	if err := server.ListenAndServe(); err != nil {
		panic(err)
	}
}

func openBrowser(url string) {
	time.Sleep(700 * time.Millisecond)
	switch runtime.GOOS {
	case "windows":
		_ = exec.Command("cmd", "/c", "start", "", url).Start()
	case "darwin":
		_ = exec.Command("open", url).Start()
	default:
		_ = exec.Command("xdg-open", url).Start()
	}
}

func withLocalCORS(next http.Handler, port string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := r.Header.Get("Origin")
		if origin != "" {
			parsed, err := neturl.Parse(origin)
			if err != nil || !isAllowedLocalOrigin(parsed, port) {
				http.Error(w, "forbidden origin", http.StatusForbidden)
				return
			}
			w.Header().Set("Access-Control-Allow-Origin", origin)
			w.Header().Set("Vary", "Origin")
		}
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
		w.Header().Set("Access-Control-Allow-Methods", "GET,POST,OPTIONS")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func isAllowedLocalOrigin(origin *neturl.URL, port string) bool {
	host := strings.ToLower(origin.Hostname())
	return origin.Scheme == "http" &&
		origin.Port() == port &&
		(host == "127.0.0.1" || host == "localhost" || host == "::1")
}

func serveStatic(w http.ResponseWriter, r *http.Request) {
	name := strings.TrimPrefix(path.Clean(r.URL.Path), "/")
	if name == "" || name == "." || !strings.Contains(name, ".") {
		name = "index.html"
	}
	data, err := webFS.ReadFile(name)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Cache-Control", "no-store, no-cache, must-revalidate, max-age=0")
	w.Header().Set("Pragma", "no-cache")
	w.Header().Set("Expires", "0")
	w.Header().Set("Content-Type", contentType(name))
	_, _ = w.Write(data)
}

func serveExporterStatic(w http.ResponseWriter, r *http.Request) {
	name := strings.TrimPrefix(path.Clean(r.URL.Path), "/exporter/")
	if name == "" || name == "." || !strings.Contains(name, ".") {
		name = "index.html"
	}
	data, err := webFS.ReadFile(path.Join("cmd/course-exporter/web", name))
	if err != nil {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Cache-Control", "no-store, no-cache, must-revalidate, max-age=0")
	w.Header().Set("Pragma", "no-cache")
	w.Header().Set("Expires", "0")
	w.Header().Set("Content-Type", contentType(name))
	_, _ = w.Write(data)
}

func handleStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	payload, source, err := school.EnsureCourseFile("course.json")
	if err != nil {
		writeJSON(w, school.StatusResponse{Ready: false, Message: err.Error()})
		return
	}
	personalCount, personalErr := readPersonalScheduleCount("personal-schedule.json")
	if personalErr != nil {
		writeJSON(w, school.StatusResponse{
			Ready:               true,
			Message:             "course.json 可用，personal-schedule.json 缺失或不可解析；可先进入排课页，也可返回导出页补导个人课表。",
			Count:               len(payload.Items),
			CourseName:          school.InferCourseName(payload.Items),
			FileName:            "course.json",
			PersonalFileName:    "personal-schedule.json",
			PersonalExported:    false,
			PersonalExportError: personalErr.Error(),
		})
		return
	}
	message := ""
	if source != "json" {
		message = "已从 " + source + " 生成 course.json"
	}
	writeJSON(w, school.StatusResponse{
		Ready:            true,
		Message:          message,
		Count:            len(payload.Items),
		CourseName:       school.InferCourseName(payload.Items),
		FileName:         "course.json",
		PersonalCount:    personalCount,
		PersonalFileName: "personal-schedule.json",
		PersonalExported: true,
	})
}

func readPersonalScheduleCount(name string) (int, error) {
	data, err := os.ReadFile(name)
	if err != nil {
		return 0, err
	}
	var payload struct {
		Items []map[string]any `json:"items"`
	}
	if err := json.Unmarshal(data, &payload); err == nil {
		if payload.Items == nil {
			return 0, fmt.Errorf("personal-schedule.json 为空或缺少 items")
		}
		return len(payload.Items), nil
	}
	var list []map[string]any
	if err := json.Unmarshal(data, &list); err != nil {
		return 0, err
	}
	return len(list), nil
}

func handleCourse(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if _, _, err := school.EnsureCourseFile("course.json"); err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}
	data, err := os.ReadFile("course.json")
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	_, _ = w.Write(data)
}

func handlePersonalSchedule(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	data, err := os.ReadFile("personal-schedule.json")
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}
	var raw struct {
		Items []map[string]any `json:"items"`
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		var list []map[string]any
		if listErr := json.Unmarshal(data, &list); listErr != nil {
			http.Error(w, err.Error(), http.StatusNotFound)
			return
		}
		raw.Items = list
	}
	if raw.Items == nil {
		raw.Items = []map[string]any{}
	}
	writeJSON(w, map[string]any{"items": raw.Items})
}

func handleImport(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, maxImportBytes)
	body, err := readBody(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	payload, err := school.DecodeCoursePayload(body)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if err := os.WriteFile("course.json", body, 0644); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, map[string]any{
		"ok":         true,
		"count":      len(payload.Items),
		"courseName": school.InferCourseName(payload.Items),
	})
}

func handleExport(state *appState) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		var req school.ExportRequest
		r.Body = http.MaxBytesReader(w, r.Body, maxExportRequestBytes)
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		result, err := state.service.RunExport(req)
		if err != nil {
			writeJSON(w, map[string]any{"ok": false, "error": err.Error(), "status": state.service.Status()})
			return
		}
		writeJSON(w, map[string]any{
			"ok":                  true,
			"count":               result.Count,
			"courseName":          result.CourseName,
			"fileName":            result.FileName,
			"outputPath":          result.OutputPath,
			"personalCount":       result.PersonalCount,
			"personalFileName":    result.PersonalFileName,
			"personalOutputPath":  result.PersonalOutputPath,
			"personalExported":    result.PersonalExported,
			"personalExportError": result.PersonalExportError,
			"message":             "课程数据导出完成",
			"status":              state.service.Status(),
		})
	}
}

func handleExportStatus(state *appState) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		writeJSON(w, state.service.Status())
	}
}

func handleOpenOutput(state *appState) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		status := state.service.Status()
		outputPath := strings.TrimSpace(status.OutputPath)
		if outputPath == "" {
			outputPath, _ = filepath.Abs("course.json")
		}
		if err := openOutputPath(outputPath); err != nil {
			writeJSON(w, map[string]any{"ok": false, "error": err.Error(), "path": outputPath})
			return
		}
		writeJSON(w, map[string]any{"ok": true, "path": outputPath})
	}
}

func openOutputPath(filePath string) error {
	dir := filepath.Dir(filePath)
	switch runtime.GOOS {
	case "windows":
		if _, err := os.Stat(filePath); err == nil {
			return exec.Command("explorer.exe", "/select,", filePath).Start()
		}
		return exec.Command("explorer.exe", dir).Start()
	case "darwin":
		return exec.Command("open", dir).Start()
	default:
		return exec.Command("xdg-open", dir).Start()
	}
}

func readBody(r *http.Request) ([]byte, error) {
	return io.ReadAll(r.Body)
}

func contentType(name string) string {
	switch strings.ToLower(filepath.Ext(name)) {
	case ".html":
		return "text/html; charset=utf-8"
	case ".css":
		return "text/css; charset=utf-8"
	case ".js":
		return "application/javascript; charset=utf-8"
	case ".json":
		return "application/json; charset=utf-8"
	default:
		return "application/octet-stream"
	}
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	_ = json.NewEncoder(w).Encode(v)
}
