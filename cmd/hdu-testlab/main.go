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
	scenario     string
	key          string
	course       fixture
	personal     fixture
	killCourseKC fixture
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
	killCourseKC, err := loadKillCourseFixture(fixtureDir)
	if err != nil {
		return err
	}
	key, err := newPublicKey()
	if err != nil {
		return err
	}
	mock := &mockServer{scenario: scenario, key: key, course: course, personal: personal, killCourseKC: killCourseKC}
	mux := http.NewServeMux()
	mux.HandleFunc("/health", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, map[string]any{"ok": true, "scenario": scenario})
	})
	mux.HandleFunc("/cas/login", mock.handleCASLogin)
	mux.HandleFunc("/jw/login", mock.handleNewJWLogin)
	mux.HandleFunc("/jw/public-key", mock.handlePublicKey)
	mux.HandleFunc("/jw/course", mock.handleCourse)
	mux.HandleFunc("/jw/personal", mock.handlePersonal)
	// KillCourse protocol routes (used by the executor via kcclient.BaseJWURL).
	mux.HandleFunc("/xtgl/login_slogin.html", mock.handleKillCourseLogin)
	mux.HandleFunc("/xtgl/login_getPublicKey.html", mock.handlePublicKey)
	mux.HandleFunc("/kbcx/xskbcx_cxXsgrkb.html", mock.handleStuInfo)
	mux.HandleFunc("/xsxk/zzxkyzb_cxZzxkYzbIndex.html", mock.handleSelectIndex)
	mux.HandleFunc("/xsxk/zzxkyzbjk_cxJxbWithKchZzxkYzb.html", mock.handleDoJxbId)
	mux.HandleFunc("/xsxk/zzxkyzbjk_xkBcZyZzxkYzb.html", mock.handleKillSelect)
	mux.HandleFunc("/xsxk/zzxkyzb_tuikBcZzxkYzb.html", mock.handleKillDrop)
	mux.HandleFunc("/xsxk/zzxkyzb_cxZzxkYzbPartDisplay.html", mock.handlePartDisplay)

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
	case "course-shape-drift":
		writeJSON(w, map[string]any{"items": []map[string]any{
			{"jxmc": "(2026-2027-1)-A0001001-01", "kcmc": "高等数学A", "sksj": "星期一第1-2节{1-17周}"},
			{"jxmc": "(2026-2027-1)-A0001001-02", "kcmc": "高等数学A", "sksj": "星期二第3-4节{1-17周}"},
		}})
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

// handleKillCourseLogin serves the newjw login page (GET csrftoken) and the
// password login POST that the vendored KillCourse client uses.
func (m *mockServer) handleKillCourseLogin(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodGet {
		_, _ = io.WriteString(w, `<input name="csrftoken" value="test-csrf">`)
		return
	}
	if err := r.ParseForm(); err != nil {
		http.Error(w, "invalid form", http.StatusBadRequest)
		return
	}
	if r.Form.Get("yhm") != testUsername || strings.TrimSpace(r.Form.Get("mm")) == "" {
		_, _ = io.WriteString(w, "用户名或密码错误")
		return
	}
	http.SetCookie(w, &http.Cookie{Name: "JSESSIONID", Value: "mock-session", Path: "/", HttpOnly: true})
	http.SetCookie(w, &http.Cookie{Name: "route", Value: "mock-route", Path: "/", HttpOnly: true})
	_, _ = io.WriteString(w, "login ok")
}

// handleStuInfo returns the student profile the course executor caches.
func (m *mockServer) handleStuInfo(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, map[string]any{"xsxx": map[string]any{"NJDM_ID": "2026", "ZYH_ID": "zyh-2026"}})
}

// handleSelectIndex returns the course-selection configuration page parsed by
// GetClientBodyConfig. XkkzId values deliberately avoid hyphens because the
// upstream parser extracts them with a \w+ pattern.
func (m *mockServer) handleSelectIndex(w http.ResponseWriter, _ *http.Request) {
	const page = `<input name="ccdm" value="ccdm-1">` +
		`<input name="bh_id" value="2026">` +
		`<input name="jg_id_1" value="jg-1">` +
		`<input name="xsbj" value="xsbj-1">` +
		`<input name="xz" value="4">` +
		`<input name="mzm" value="mzm-1">` +
		`<input name="xslbdm" value="xslbdm-1">` +
		`<input name="xbm" value="xbm-1">` +
		`<input name="zyfx_id" value="zyfx-1">` +
		`<input name="xqh_id" value="xqh-1">` +
		`<a role="tab" onclick="queryCourse(this,'01','xkkz01')">主修</a>` +
		`<a role="tab" onclick="queryCourse(this,'10','xkkz10')">通识选修</a>`
	_, _ = io.WriteString(w, page)
}

// handleDoJxbId returns the do_jxb_id mapping for every fixture item.
func (m *mockServer) handleDoJxbId(w http.ResponseWriter, _ *http.Request) {
	items := make([]map[string]any, 0, len(m.killCourseKC.Items))
	for _, item := range m.killCourseKC.Items {
		jxbID, _ := item["jxb_id"].(string)
		if jxbID == "" {
			continue
		}
		items = append(items, map[string]any{"jxb_id": jxbID, "do_jxb_id": "do-" + jxbID})
	}
	writeJSON(w, items)
}

// handleKillSelect returns the course-selection outcome for the scenario.
func (m *mockServer) handleKillSelect(w http.ResponseWriter, _ *http.Request) {
	if m.scenario == "killcourse-fail" {
		writeJSON(w, map[string]any{"flag": "0", "msg": "人数已满"})
		return
	}
	writeJSON(w, map[string]any{"flag": "1", "msg": "选课成功"})
}

// handleKillDrop reports a successful drop for every scenario.
func (m *mockServer) handleKillDrop(w http.ResponseWriter, _ *http.Request) {
	_, _ = io.WriteString(w, `"1"`)
}

// handlePartDisplay reports capacity for wait mode (tmpList non-empty).
func (m *mockServer) handlePartDisplay(w http.ResponseWriter, _ *http.Request) {
	items := make([]map[string]any, 0, len(m.killCourseKC.Items))
	for _, item := range m.killCourseKC.Items {
		jxbmc, _ := item["jxbmc"].(string)
		if jxbmc == "" {
			continue
		}
		items = append(items, map[string]any{"jxbmc": jxbmc})
	}
	writeJSON(w, map[string]any{"tmpList": items})
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

func loadKillCourseFixture(fixtureDir string) (fixture, error) {
	path := filepath.Join(fixtureDir, "killcourse.course.sample.json")
	if _, err := os.Stat(path); err != nil {
		return fixture{}, nil
	}
	return readFixture(path)
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
	case "success", "bad-password", "forbidden", "malformed-course", "empty-course", "course-shape-drift", "timeout", "personal-failure", "killcourse", "killcourse-fail":
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
