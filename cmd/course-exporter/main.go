package main

import (
	"embed"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"hdu-scheduler/school"
)

//go:embed web/*
var webFS embed.FS

const addr = "127.0.0.1:6790"

type appState struct {
	service *school.Service
}

func main() {
	state := &appState{service: school.NewService()}

	mux := http.NewServeMux()
	mux.HandleFunc("/", serveStatic)
	mux.HandleFunc("/api/export", handleExport(state))
	mux.HandleFunc("/api/export/status", handleExportStatus(state))
	mux.HandleFunc("/api/export/open-output", handleOpenOutput(state))

	if os.Getenv("HDU_NO_BROWSER") != "1" {
		go openBrowser("http://" + addr + "/")
	}

	fmt.Println("HDU Course Exporter running at http://" + addr)
	if err := http.ListenAndServe(addr, withCORS(mux)); err != nil {
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

func withCORS(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
		w.Header().Set("Access-Control-Allow-Methods", "GET,POST,OPTIONS")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func serveStatic(w http.ResponseWriter, r *http.Request) {
	name := strings.TrimPrefix(path.Clean(r.URL.Path), "/")
	if name == "" || name == "." || !strings.Contains(name, ".") {
		name = "index.html"
	}
	data, err := webFS.ReadFile(path.Join("web", name))
	if err != nil {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Cache-Control", "no-store, no-cache, must-revalidate, max-age=0")
	w.Header().Set("Content-Type", contentType(name))
	_, _ = w.Write(data)
}

func handleExport(state *appState) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		var req school.ExportRequest
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

func contentType(name string) string {
	switch strings.ToLower(filepath.Ext(name)) {
	case ".html":
		return "text/html; charset=utf-8"
	case ".css":
		return "text/css; charset=utf-8"
	case ".js":
		return "application/javascript; charset=utf-8"
	default:
		return "application/octet-stream"
	}
}

func writeJSON(w http.ResponseWriter, value any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	_ = json.NewEncoder(w).Encode(value)
}
