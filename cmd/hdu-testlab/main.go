// Command hdu-testlab provides a local, deterministic teaching-system mock.
// It exists for acceptance testing only and never contacts HDU endpoints.
package main

import (
	"crypto/rand"
	"crypto/rsa"
	"encoding/base64"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"hdu-scheduler/school"
)

const (
	testUsername = "test-user"
	testPassword = "test-password"
)

type fixture struct {
	Items []map[string]any `json:"items"`
}

type mockServer struct {
	scenario string
	key      string
	course   fixture
	personal fixture
}

func main() {
	mode := flag.String("mode", "serve", "serve or export")
	listen := flag.String("listen", "127.0.0.1:18674", "loopback listen address")
	baseURL := flag.String("base", "", "mock server base URL for export mode")
	scenario := flag.String("scenario", "success", "mock scenario")
	fixtures := flag.String("fixtures", "testdata", "directory containing sample JSON fixtures")
	output := flag.String("output", "", "directory where export-mode files are written")
	timeout := flag.Duration("timeout", 750*time.Millisecond, "test exporter HTTP timeout")
	flag.Parse()

	switch strings.ToLower(strings.TrimSpace(*mode)) {
	case "serve":
		if err := serve(*listen, *scenario, *fixtures); err != nil {
			fmt.Fprintln(os.Stderr, "testlab server:", err)
			os.Exit(1)
		}
	case "export":
		if err := runExport(*baseURL, *output, *timeout); err != nil {
			fmt.Fprintln(os.Stderr, "testlab export:", err)
			os.Exit(1)
		}
	default:
		fmt.Fprintln(os.Stderr, "-mode must be serve or export")
		os.Exit(2)
	}
}

func serve(listen, scenario, fixtureDir string) error {
	if !isLoopbackListenAddress(listen) {
		return errors.New("test lab only permits a 127.0.0.1 or ::1 listen address")
	}
	if !knownScenario(scenario) {
		return fmt.Errorf("unknown scenario %q", scenario)
	}
	course, err := readFixture(filepath.Join(fixtureDir, "course.sample.json"))
	if err != nil {
		return err
	}
	personal, err := readFixture(filepath.Join(fixtureDir, "personal-schedule.sample.json"))
	if err != nil {
		return err
	}
	key, err := newPublicKey()
	if err != nil {
		return err
	}
	mock := &mockServer{scenario: scenario, key: key, course: course, personal: personal}
	mux := http.NewServeMux()
	mux.HandleFunc("/health", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, map[string]any{"ok": true, "scenario": scenario})
	})
	mux.HandleFunc("/cas/login", mock.handleCASLogin)
	mux.HandleFunc("/jw/login", mock.handleNewJWLogin)
	mux.HandleFunc("/jw/public-key", mock.handlePublicKey)
	mux.HandleFunc("/jw/course", mock.handleCourse)
	mux.HandleFunc("/jw/personal", mock.handlePersonal)

	listener, err := net.Listen("tcp", listen)
	if err != nil {
		return err
	}
	defer listener.Close()
	fmt.Printf("TESTLAB_READY http://%s scenario=%s\n", listener.Addr().String(), scenario)
	return http.Serve(listener, mux)
}

func runExport(baseURL, output string, timeout time.Duration) error {
	if strings.TrimSpace(baseURL) == "" {
		return errors.New("-base is required in export mode")
	}
	if strings.TrimSpace(output) == "" {
		return errors.New("-output is required in export mode")
	}
	if err := os.MkdirAll(output, 0755); err != nil {
		return err
	}
	oldDir, err := os.Getwd()
	if err != nil {
		return err
	}
	if err := os.Chdir(output); err != nil {
		return err
	}
	defer os.Chdir(oldDir)

	service := school.NewService()
	result, err := service.RunExportWithTestEndpoints(
		school.ExportRequest{Method: "password", Username: testUsername, Password: testPassword, XueNian: "2026", XueQi: "1"},
		endpointsFromBase(strings.TrimRight(baseURL, "/")),
		timeout,
	)
	response := map[string]any{"ok": err == nil, "status": service.Status()}
	if result != nil {
		response["result"] = result
	}
	if err != nil {
		response["error"] = err.Error()
	}
	writeJSON(os.Stdout, response)
	return err
}

func endpointsFromBase(base string) school.ExporterEndpoints {
	return school.ExporterEndpoints{
		CASLogin:         base + "/cas/login",
		CASService:       base + "/cas/service",
		NewJWLogin:       base + "/jw/login",
		PublicKey:        base + "/jw/public-key",
		Course:           base + "/jw/course?doType=query&gnmkdm=N1548",
		PersonalSchedule: base + "/jw/personal?gnmkdm=N2151",
	}
}

func (m *mockServer) handleCASLogin(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodGet {
		_, _ = io.WriteString(w, `<input id="login-page-flowkey" value="test-execution"><input id="login-croypto" value="MDEyMzQ1Njc4OUFCQ0RFRg==">`)
		return
	}
	writeLoginResult(w, m.scenario)
}

func (m *mockServer) handleNewJWLogin(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodGet {
		_, _ = io.WriteString(w, `<input name="csrftoken" value="test-csrf">`)
		return
	}
	if err := r.ParseForm(); err != nil {
		http.Error(w, "invalid form", http.StatusBadRequest)
		return
	}
	// The mock only recognizes its fictional account and deliberately does not
	// retain the submitted form or encrypted password.
	if r.Form.Get("yhm") != testUsername || strings.TrimSpace(r.Form.Get("mm")) == "" || m.scenario == "bad-password" {
		_, _ = io.WriteString(w, "用户名或密码错误")
		return
	}
	http.SetCookie(w, &http.Cookie{Name: "TESTLAB_SESSION", Value: "ok", Path: "/", HttpOnly: true})
	_, _ = io.WriteString(w, "login ok")
}

func writeLoginResult(w http.ResponseWriter, scenario string) {
	if scenario == "bad-password" {
		_, _ = io.WriteString(w, "用户名或密码错误")
		return
	}
	http.SetCookie(w, &http.Cookie{Name: "TESTLAB_SESSION", Value: "ok", Path: "/", HttpOnly: true})
	_, _ = io.WriteString(w, "login ok")
}

func (m *mockServer) handlePublicKey(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, map[string]string{"modulus": m.key, "exponent": "10001"})
}

func (m *mockServer) handleCourse(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodGet {
		_, _ = io.WriteString(w, "course query ready")
		return
	}
	switch m.scenario {
	case "forbidden":
		_, _ = io.WriteString(w, "无功能权限")
	case "malformed-course":
		_, _ = io.WriteString(w, "{this is not json")
	case "empty-course":
		writeJSON(w, map[string]any{"items": []any{}})
	case "timeout":
		time.Sleep(2 * time.Second)
		writeJSON(w, map[string]any{"items": m.course.Items})
	default:
		writeJSON(w, map[string]any{"items": m.course.Items})
	}
}

func (m *mockServer) handlePersonal(w http.ResponseWriter, _ *http.Request) {
	if m.scenario == "personal-failure" {
		_, _ = io.WriteString(w, "无功能权限")
		return
	}
	writeJSON(w, map[string]any{"kbList": m.personal.Items})
}

func readFixture(name string) (fixture, error) {
	data, err := os.ReadFile(name)
	if err != nil {
		return fixture{}, fmt.Errorf("read fixture %s: %w", name, err)
	}
	var value fixture
	if err := json.Unmarshal(data, &value); err != nil {
		return fixture{}, fmt.Errorf("decode fixture %s: %w", name, err)
	}
	if len(value.Items) == 0 {
		return fixture{}, fmt.Errorf("fixture %s has no items", name)
	}
	return value, nil
}

func newPublicKey() (string, error) {
	key, err := rsa.GenerateKey(rand.Reader, 1024)
	if err != nil {
		return "", err
	}
	return base64.StdEncoding.EncodeToString(key.PublicKey.N.Bytes()), nil
}

func knownScenario(value string) bool {
	switch value {
	case "success", "bad-password", "forbidden", "malformed-course", "empty-course", "timeout", "personal-failure":
		return true
	default:
		return false
	}
}

func isLoopbackListenAddress(value string) bool {
	host, _, err := net.SplitHostPort(value)
	if err != nil {
		return false
	}
	return host == "127.0.0.1" || host == "::1"
}

func writeJSON(writer io.Writer, value any) {
	_ = json.NewEncoder(writer).Encode(value)
}
