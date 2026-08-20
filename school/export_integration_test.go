package school

import (
	"crypto/rand"
	"crypto/rsa"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

type exporterTestScenario struct {
	courseBody     string
	personalBody   string
	courseStatus   int
	personalStatus int
	loginCount     *int
	personalCount  *int
}

func TestRunExportWithTestEndpointsSuccess(t *testing.T) {
	result, status, outputDir, err := runMockExporter(t, exporterTestScenario{
		courseBody:   `{"rows":[{"jxbmc":"(2026-2027-1)-A0001001-01","kcmc":"Test Math","sksj":"星期一第1-2节{1-17周}","xf":"3.00"}]}`,
		personalBody: `{"kbList":[{"jxbmc":"(2026-2027-1)-A0001001-01","kcmc":"Test Math","xqj":"1","jc":"1-2","zcd":"1-17周"}]}`,
	})
	if err != nil {
		t.Fatalf("RunExportWithTestEndpoints() error = %v", err)
	}
	if result.Count != 1 || !result.PersonalExported || result.PersonalCount != 1 {
		t.Fatalf("unexpected result: %#v", result)
	}
	if !status.Ready || status.Phase != "success" {
		t.Fatalf("unexpected final status: %#v", status)
	}
	for _, name := range []string{"course.json", "personal-schedule.json"} {
		data, readErr := os.ReadFile(outputDir + string(os.PathSeparator) + name)
		if readErr != nil {
			t.Fatalf("read %s: %v", name, readErr)
		}
		var payload CoursePayload
		if unmarshalErr := json.Unmarshal(data, &payload); unmarshalErr != nil || len(payload.Items) != 1 {
			t.Fatalf("invalid %s: items=%d error=%v", name, len(payload.Items), unmarshalErr)
		}
	}
}

func TestRunExportWithTestEndpointsWritesDiagnosisForMalformedCourse(t *testing.T) {
	_, status, outputDir, err := runMockExporter(t, exporterTestScenario{
		courseBody:   `{not valid json`,
		personalBody: `{"kbList":[]}`,
	})
	if err == nil || status.Step != "query" {
		t.Fatalf("malformed course response error=%v status=%#v", err, status)
	}
	data, readErr := os.ReadFile(outputDir + string(os.PathSeparator) + "course-export-diagnosis.json")
	if readErr != nil {
		t.Fatalf("diagnostic file was not written: %v", readErr)
	}
	if !strings.Contains(string(data), `"bodyBytes"`) || !strings.Contains(string(data), `"term": "2026-2027-1"`) {
		t.Fatalf("diagnostic payload is incomplete: %s", data)
	}
}

func TestRunExportWithTestEndpointsReportsCourseForbidden(t *testing.T) {
	_, status, _, err := runMockExporter(t, exporterTestScenario{
		courseBody:   `{"message":"Forbidden"}`,
		courseStatus: http.StatusForbidden,
		personalBody: `{"kbList":[]}`,
	})
	if err == nil || !strings.Contains(err.Error(), "HTTP 403") || status.Step != "query" {
		t.Fatalf("course forbidden error=%v status=%#v", err, status)
	}
	if strings.Contains(err.Error(), "没有拿到课程数据") {
		t.Fatalf("HTTP 403 should not be reported as a term mismatch: %v", err)
	}
}

func TestRunExportWithTestEndpointsRejectsCourseShapeDrift(t *testing.T) {
	_, status, outputDir, err := runMockExporter(t, exporterTestScenario{
		courseBody:   `{"items":[{"jxmc":"(2026-2027-1)-A0001001-01","kcmc":"高等数学A"},{"jxmc":"(2026-2027-1)-A0001001-02","kcmc":"高等数学A"}]}`,
		personalBody: `{"kbList":[]}`,
	})
	if err == nil || !strings.Contains(err.Error(), "疑似改版") || status.Step != "query" {
		t.Fatalf("shape drift error=%v status=%#v", err, status)
	}
	data, readErr := os.ReadFile(outputDir + string(os.PathSeparator) + "course-export-diagnosis.json")
	if readErr != nil {
		t.Fatalf("diagnosis file was not written for shape drift: %v", readErr)
	}
	if !strings.Contains(string(data), `"shapeDrift"`) {
		t.Fatalf("diagnosis is missing shapeDrift: %s", data)
	}
	if _, statErr := os.Stat(outputDir + string(os.PathSeparator) + "course.json"); statErr == nil {
		t.Fatalf("course.json must not be written when the response shape drifted")
	}
}

func TestFinishCASLoginRejectsReturnedLoginPage(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("service") == "" {
			t.Fatalf("CAS service parameter missing from callback request")
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write([]byte(`<html><form action="/login"><input name="username"><input name="password"></form></html>`))
	}))
	t.Cleanup(server.Close)

	exp := newExporterWithEndpoints(ExporterEndpoints{
		CASLogin:   server.URL + "/cas/login",
		CASService: server.URL + "/cas/service",
	}, time.Second)
	if err := exp.finishCASLogin(); err == nil || !strings.Contains(err.Error(), "CAS") {
		t.Fatalf("finishCASLogin() error = %v, want a CAS authentication error", err)
	}
}

func TestFinishCASLoginLabelsForbiddenAsCAS(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusForbidden)
		_, _ = w.Write([]byte(`Forbidden`))
	}))
	t.Cleanup(server.Close)

	exp := newExporterWithEndpoints(ExporterEndpoints{
		CASLogin:   server.URL + "/cas/login",
		CASService: server.URL + "/cas/service",
	}, time.Second)
	if err := exp.finishCASLogin(); err == nil || !strings.Contains(err.Error(), "CAS") || strings.Contains(err.Error(), "个人课表") {
		t.Fatalf("finishCASLogin() error = %v, want a CAS-specific forbidden error", err)
	}
}

func TestLoginCASBindsServiceOnInitialAndSubmit(t *testing.T) {
	service := "http://127.0.0.1/service"
	key := base64.StdEncoding.EncodeToString([]byte("0123456789abcdef"))
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/cas/login":
			if r.URL.Query().Get("service") != service {
				t.Fatalf("CAS login service query = %q, want %q", r.URL.Query().Get("service"), service)
			}
			if r.Method == http.MethodGet {
				_, _ = fmt.Fprintf(w, `<input id="login-page-flowkey" value="flow"><input id="login-croypto" value="%s">`, key)
				return
			}
			if r.Header.Get("Referer") == "" || !strings.Contains(r.Header.Get("Accept"), "text/html") {
				t.Fatalf("CAS login POST is missing browser page headers")
			}
			if err := r.ParseForm(); err != nil {
				t.Fatalf("ParseForm() error = %v", err)
			}
			if r.Form.Get("username") != "test-user" || r.Form.Get("password") == "" {
				t.Fatalf("CAS credentials were not submitted")
			}
			w.Header().Set("Location", "/cas/service?ticket=test-ticket")
			w.WriteHeader(http.StatusFound)
		case "/cas/service":
			if r.URL.Query().Get("ticket") != "test-ticket" {
				t.Fatalf("CAS ticket missing from callback: %q", r.URL.RawQuery)
			}
			_, _ = w.Write([]byte("logged in"))
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(server.Close)
	service = server.URL + "/cas/service"

	exp := newExporterWithEndpoints(ExporterEndpoints{
		CASLogin:   server.URL + "/cas/login",
		CASService: service,
	}, time.Second)
	if err := exp.loginCAS("password", "test-user", "test-password"); err != nil {
		t.Fatalf("loginCAS() error = %v", err)
	}
}

func TestDefaultExporterEndpointsUseSecureCASService(t *testing.T) {
	if !strings.HasPrefix(DefaultExporterEndpoints().CASService, "https://") {
		t.Fatalf("CASService = %q, want an HTTPS callback", DefaultExporterEndpoints().CASService)
	}
}

func TestLoginPreservesDirectAndCASFailureReasons(t *testing.T) {
	server := httptest.NewServer(http.NotFoundHandler())
	t.Cleanup(server.Close)
	exp := newExporterWithEndpoints(ExporterEndpoints{
		CASLogin:   server.URL + "/cas/login",
		CASService: server.URL + "/cas/service",
		NewJWLogin: server.URL + "/jw/login",
		PublicKey:  server.URL + "/jw/public-key",
	}, time.Second)
	err := exp.login("password", "test-user", "test-password")
	if err == nil || !strings.Contains(err.Error(), "直登") || !strings.Contains(err.Error(), "CAS") {
		t.Fatalf("login() error = %v, want both direct and CAS failure reasons", err)
	}
}

func TestExportPersonalScheduleSendsBrowserPageHeaders(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("User-Agent") != userAgent ||
			r.Header.Get("Referer") == "" ||
			!strings.Contains(r.Header.Get("Accept"), "text/html") {
			w.WriteHeader(http.StatusForbidden)
			_, _ = w.Write([]byte(`{"message":"Forbidden"}`))
			return
		}
		_, _ = w.Write([]byte(`{"kbList":[{"jxbmc":"test-course","kcmc":"Test Course","xqj":"1","jc":"1-2","zcd":"1-17"}]}`))
	}))
	t.Cleanup(server.Close)

	outputDir := t.TempDir()
	oldWD, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(outputDir); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(oldWD) })

	exp := newExporterWithEndpoints(ExporterEndpoints{
		PersonalSchedule: server.URL + "/jw/personal?gnmkdm=N2151",
	}, time.Second)
	result, err := exp.exportPersonalSchedule(ExportRequest{XueNian: "2026", XueQi: "1"})
	if err != nil {
		t.Fatalf("exportPersonalSchedule() error = %v", err)
	}
	if result.PersonalCount != 1 || !result.PersonalExported {
		t.Fatalf("unexpected personal export result: %#v", result)
	}
}

func TestRunExportWithTestEndpointsKeepsCourseWhenPersonalExportFails(t *testing.T) {
	result, status, outputDir, err := runMockExporter(t, exporterTestScenario{
		courseBody:   `{"items":[{"jxbmc":"(2026-2027-1)-A0001001-01","kcmc":"Test Math","sksj":"星期一第1-2节{1-17周}","xf":"3.00"}]}`,
		personalBody: "无功能权限",
	})
	if err != nil {
		t.Fatalf("course export should remain successful when personal export fails: %v", err)
	}
	if result.PersonalExported || result.PersonalExportError == "" || !status.Ready {
		t.Fatalf("personal failure was not retained in result/status: result=%#v status=%#v", result, status)
	}
	if _, readErr := os.Stat(outputDir + string(os.PathSeparator) + "course.json"); readErr != nil {
		t.Fatalf("course.json missing after personal export failure: %v", readErr)
	}
	if _, readErr := os.Stat(outputDir + string(os.PathSeparator) + "personal-schedule.json"); !os.IsNotExist(readErr) {
		t.Fatalf("personal-schedule.json should not exist, stat error=%v", readErr)
	}
}

func TestRunExportWithTestEndpointsRejectsForbiddenPersonalSchedule(t *testing.T) {
	result, status, outputDir, err := runMockExporter(t, exporterTestScenario{
		courseBody:     `{"items":[{"jxbmc":"(2026-2027-1)-A0001001-01","kcmc":"Test Math","sksj":"星期一第1-2节{1-17周}","xf":"3.00"}]}`,
		personalBody:   `{"message":"Forbidden"}`,
		personalStatus: http.StatusForbidden,
	})
	if err != nil {
		t.Fatalf("course export should remain successful when personal endpoint is forbidden: %v", err)
	}
	if result == nil || result.PersonalExported || !strings.Contains(result.PersonalExportError, "HTTP 403") ||
		!strings.Contains(result.PersonalExportError, "浏览器") || !status.Ready {
		t.Fatalf("personal forbidden response was not retained: result=%#v status=%#v", result, status)
	}
	if _, readErr := os.Stat(outputDir + string(os.PathSeparator) + "personal-schedule.json"); !os.IsNotExist(readErr) {
		t.Fatalf("personal-schedule.json should not be written for HTTP 403, stat error=%v", readErr)
	}
}

func TestRunExportThenRefreshPersonalScheduleReusesSession(t *testing.T) {
	loginCount := 0
	personalCount := 0
	service, _, _, err := runMockExporterService(t, exporterTestScenario{
		courseBody:    `{"items":[{"jxbmc":"(2026-2027-1)-A0001001-01","kcmc":"Test Math","sksj":"星期一第1-2节{1-17周}","xf":"3.00"}]}`,
		personalBody:  `{"kbList":[{"jxbmc":"(2026-2027-1)-A0001001-01","kcmc":"Test Math","xqj":"1","jc":"1-2","zcd":"1-17周"}]}`,
		loginCount:    &loginCount,
		personalCount: &personalCount,
	})
	if err != nil {
		t.Fatalf("RunExportWithTestEndpoints() error = %v", err)
	}
	initialPersonalCount := personalCount
	if loginCount != 1 || initialPersonalCount < 1 {
		t.Fatalf("initial request counts = login %d personal %d", loginCount, personalCount)
	}

	result, err := service.RefreshPersonalSchedule()
	if err != nil {
		t.Fatalf("RefreshPersonalSchedule() error = %v", err)
	}
	if !result.PersonalExported || result.PersonalCount != 1 {
		t.Fatalf("unexpected refresh result: %#v", result)
	}
	if loginCount != 1 || personalCount != initialPersonalCount+1 {
		t.Fatalf("refresh request counts = login %d personal %d", loginCount, personalCount)
	}
}

func TestStartPersonalScheduleRefreshPublishesExportingBeforeLaunch(t *testing.T) {
	service := NewService()
	service.authenticated = &exporter{}
	service.loginRequest = ExportRequest{Method: "browser", XueNian: "2026", XueQi: "1"}
	service.setStatus("success", "done", "previous refresh", true, &ExportResult{PersonalExported: true})

	var launched func()
	service.launch = func(task func()) {
		launched = task
	}
	if err := service.StartPersonalScheduleRefresh(); err != nil {
		t.Fatalf("StartPersonalScheduleRefresh() error = %v", err)
	}
	t.Cleanup(service.endRun)
	if launched == nil {
		t.Fatal("refresh worker was not handed to launch hook")
	}
	if status := service.Status(); status.Phase != "exporting" || status.Step != "personal" {
		t.Fatalf("status after scheduling = %#v, want exporting/personal", status)
	}
}

func TestRefreshPersonalScheduleClearsSessionOnForbidden(t *testing.T) {
	service, _, _, err := runMockExporterService(t, exporterTestScenario{
		courseBody:     `{"items":[{"jxbmc":"(2026-2027-1)-A0001001-01","kcmc":"Test Math","sksj":"星期一第1-2节{1-17周}","xf":"3.00"}]}`,
		personalBody:   `{"message":"Forbidden"}`,
		personalStatus: http.StatusForbidden,
	})
	if err != nil {
		t.Fatalf("RunExportWithTestEndpoints() error = %v", err)
	}
	if _, err := service.RefreshPersonalSchedule(); err == nil {
		t.Fatal("RefreshPersonalSchedule() unexpectedly succeeded")
	}
	if _, _, err := service.authenticatedSession(); err == nil {
		t.Fatal("authenticated session was retained after HTTP 403")
	}
}

func TestExportPersonalScheduleRejectsNullWithoutOverwriting(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = io.WriteString(w, "null")
	}))
	t.Cleanup(server.Close)

	outputDir := t.TempDir()
	oldBytes := []byte(`{"schemaVersion":1,"items":[{"jxbmc":"old-course"}]}`)
	outputPath := filepath.Join(outputDir, "personal-schedule.json")
	if err := os.WriteFile(outputPath, oldBytes, 0644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("HDU_OUTPUT_DIR", outputDir)

	exp := newExporterWithEndpoints(ExporterEndpoints{PersonalSchedule: server.URL + "/personal"}, time.Second)
	if _, err := exp.exportPersonalSchedule(ExportRequest{XueNian: "2026", XueQi: "1"}); err == nil {
		t.Fatal("null personal schedule response unexpectedly succeeded")
	}
	got, err := os.ReadFile(outputPath)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != string(oldBytes) {
		t.Fatalf("old personal schedule changed: got %s, want %s", got, oldBytes)
	}
}

func runMockExporter(t *testing.T, scenario exporterTestScenario) (*ExportResult, StatusResponse, string, error) {
	service, outputDir, result, err := runMockExporterService(t, scenario)
	return result, service.Status(), outputDir, err
}

func runMockExporterService(t *testing.T, scenario exporterTestScenario) (*Service, string, *ExportResult, error) {
	t.Helper()
	publicKey, err := mockPublicKey()
	if err != nil {
		t.Fatal(err)
	}
	mux := http.NewServeMux()
	mux.HandleFunc("/jw/login", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			_, _ = w.Write([]byte(`<input name="csrftoken" value="test-csrf">`))
			return
		}
		if err := r.ParseForm(); err != nil || r.Form.Get("yhm") != "test-user" || r.Form.Get("mm") == "" {
			_, _ = w.Write([]byte("用户名或密码错误"))
			return
		}
		if scenario.loginCount != nil {
			(*scenario.loginCount)++
		}
		http.SetCookie(w, &http.Cookie{Name: "TEST_SESSION", Value: "ok", Path: "/"})
		_, _ = w.Write([]byte("ok"))
	})
	mux.HandleFunc("/jw/public-key", func(w http.ResponseWriter, request *http.Request) {
		if request.Header.Get("User-Agent") != userAgent ||
			request.Header.Get("Referer") == "" ||
			!strings.Contains(request.Header.Get("Accept"), "application/json") {
			w.WriteHeader(http.StatusForbidden)
			return
		}
		_ = json.NewEncoder(w).Encode(publicKeyPayload{Modulus: publicKey, Exponent: "10001"})
	})
	mux.HandleFunc("/jw/course", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			_, _ = w.Write([]byte("ready"))
			return
		}
		if scenario.courseStatus != 0 {
			w.WriteHeader(scenario.courseStatus)
		}
		_, _ = w.Write([]byte(scenario.courseBody))
	})
	personalRequests := 0
	mux.HandleFunc("/jw/personal", func(w http.ResponseWriter, _ *http.Request) {
		personalRequests++
		if scenario.personalCount != nil {
			(*scenario.personalCount)++
		}
		if scenario.personalStatus != 0 && personalRequests > 1 {
			w.WriteHeader(scenario.personalStatus)
		}
		_, _ = w.Write([]byte(scenario.personalBody))
	})
	server := httptest.NewServer(mux)
	t.Cleanup(server.Close)

	outputDir := t.TempDir()
	oldDir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(outputDir); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(oldDir) })

	base := server.URL
	endpoints := ExporterEndpoints{
		CASLogin:         base + "/cas/login",
		CASService:       base + "/cas/service",
		NewJWLogin:       base + "/jw/login",
		PublicKey:        base + "/jw/public-key",
		Course:           base + "/jw/course?doType=query",
		PersonalSchedule: base + "/jw/personal?gnmkdm=N2151",
	}
	service := NewService()
	result, runErr := service.RunExportWithTestEndpoints(
		ExportRequest{Method: "password", Username: "test-user", Password: "test-password", XueNian: "2026", XueQi: "1"},
		endpoints,
		time.Second,
	)
	return service, outputDir, result, runErr
}

func mockPublicKey() (string, error) {
	key, err := rsa.GenerateKey(rand.Reader, 1024)
	if err != nil {
		return "", err
	}
	return base64.StdEncoding.EncodeToString(key.PublicKey.N.Bytes()), nil
}

func TestRunExportFallsBackToBrowserLoginWhenPasswordBlocked(t *testing.T) {
	var loginSubmitted bool
	var loginEvalBody string
	proxy := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/targets":
			_, _ = io.WriteString(w, `[{"targetId":"target-1","type":"page","url":"https://newjw.hdu.edu.cn/jwglxt/xtgl/login_slogin.html"}]`)
		case "/new":
			_, _ = io.WriteString(w, `{"targetId":"target-1"}`)
		case "/eval":
			body, err := io.ReadAll(r.Body)
			if err != nil {
				t.Fatalf("read eval body: %v", err)
			}
			expression := string(body)
			if strings.Contains(expression, "return 'submitted'") {
				loginEvalBody = expression
				loginSubmitted = true
				_ = json.NewEncoder(w).Encode(map[string]string{"value": "submitted"})
				return
			}
			if strings.Contains(expression, "hasGetParam") {
				_ = json.NewEncoder(w).Encode(map[string]string{"value": `{"ready":"complete","hasGetParam":true,"href":"https://newjw.hdu.edu.cn/jwglxt/rwlscx/rwlscx_cxRwlsIndex.html?gnmkdm=N1548&layout=default"}`})
				return
			}
			if strings.Contains(expression, "__hduCourseQueryResults") && strings.Contains(expression, "fetch(") {
				_ = json.NewEncoder(w).Encode(map[string]string{"value": "started"})
				return
			}
			if strings.Contains(expression, "__hduCourseQueryResults") {
				state := map[string]any{
					"state":  "done",
					"status": http.StatusOK,
					"body":   `{"rows":[{"jxbmc":"(2026-2027-1)-A0001001-01","kcmc":"Test Math","sksj":"星期一第1-2节{1-17周}","xf":"3.00"}]}`,
				}
				value, _ := json.Marshal(state)
				_ = json.NewEncoder(w).Encode(map[string]string{"value": string(value)})
				return
			}
			if strings.Contains(expression, "authed") {
				href := "https://newjw.hdu.edu.cn/jwglxt/xtgl/login_slogin.html"
				if loginSubmitted {
					href = "https://newjw.hdu.edu.cn/jwglxt/xtgl/index_initMenu.html"
				}
				state := map[string]any{
					"href":         href,
					"ready":        "complete",
					"hasYhm":       true,
					"hasMm":        true,
					"hasDl":        true,
					"hasCasUser":   false,
					"hasCasPass":   false,
					"hasCasSubmit": false,
					"authed":       loginSubmitted,
				}
				value, _ := json.Marshal(state)
				_ = json.NewEncoder(w).Encode(map[string]string{"value": string(value)})
				return
			}
			_ = json.NewEncoder(w).Encode(map[string]string{"value": `{"status":200,"body":"{\"kbList\":[{\"kcmc\":\"Test Math\",\"xqj\":\"1\",\"jc\":\"1-2\",\"zcd\":\"1-17周\"}]}"}`})
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(proxy.Close)

	oldProxy, hadProxy := os.LookupEnv("HDU_BROWSER_PROXY_URL")
	_ = os.Setenv("HDU_BROWSER_PROXY_URL", proxy.URL)
	t.Cleanup(func() {
		if hadProxy {
			_ = os.Setenv("HDU_BROWSER_PROXY_URL", oldProxy)
		} else {
			_ = os.Unsetenv("HDU_BROWSER_PROXY_URL")
		}
	})

	mux := http.NewServeMux()
	mux.HandleFunc("/jw/login", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`<html>login page without token</html>`))
	})
	mux.HandleFunc("/cas/login", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`<html>cas page without flowkey</html>`))
	})
	schoolServer := httptest.NewServer(mux)
	t.Cleanup(schoolServer.Close)

	outputDir := t.TempDir()
	oldWD, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(outputDir); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(oldWD) })

	base := schoolServer.URL
	endpoints := ExporterEndpoints{
		CASLogin:         base + "/cas/login",
		CASService:       base + "/cas/service",
		NewJWLogin:       base + "/jw/login",
		PublicKey:        base + "/jw/public-key",
		Course:           base + "/jw/course?doType=query",
		PersonalSchedule: base + "/jw/personal?gnmkdm=N2151",
	}
	service := NewService()
	result, err := service.RunExportWithTestEndpoints(
		ExportRequest{Method: "password", Username: "test-user", Password: "test-password", XueNian: "2026", XueQi: "1"},
		endpoints,
		30*time.Second,
	)
	if err != nil {
		t.Fatalf("RunExportWithTestEndpoints() error = %v", err)
	}
	if result.Count != 1 || !result.PersonalExported || result.PersonalCount != 1 {
		t.Fatalf("unexpected fallback export result: %#v", result)
	}
	if !loginSubmitted {
		t.Fatal("browser login form was never submitted")
	}
	if !strings.Contains(loginEvalBody, "test-user") || !strings.Contains(loginEvalBody, "test-password") {
		t.Fatalf("browser login expression did not carry submitted credentials: %s", loginEvalBody)
	}
	status := service.Status()
	if !status.Ready || status.Phase != "success" {
		t.Fatalf("unexpected final status: %#v", status)
	}
}

func TestRefreshPersonalScheduleAutoLogsInWithoutSession(t *testing.T) {
	proxy := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/targets":
			_, _ = io.WriteString(w, `[{"targetId":"target-1","type":"page","url":"https://newjw.hdu.edu.cn/jwglxt/xtgl/index_initMenu.html"}]`)
		case "/new":
			_, _ = io.WriteString(w, `{"targetId":"target-1"}`)
		case "/eval":
			body, _ := io.ReadAll(r.Body)
			expression := string(body)
			if strings.Contains(expression, "authed") {
				state := map[string]any{
					"href":         "https://newjw.hdu.edu.cn/jwglxt/xtgl/index_initMenu.html",
					"ready":        "complete",
					"hasYhm":       false,
					"hasMm":        false,
					"hasDl":        false,
					"hasCasUser":   false,
					"hasCasPass":   false,
					"hasCasSubmit": false,
					"authed":       true,
				}
				value, _ := json.Marshal(state)
				_ = json.NewEncoder(w).Encode(map[string]string{"value": string(value)})
				return
			}
			_ = json.NewEncoder(w).Encode(map[string]string{"value": `{"status":200,"body":"{\"kbList\":[{\"kcmc\":\"Auto Course\",\"xqj\":\"1\",\"jc\":\"1-2\",\"zcd\":\"1-17\"}]}"}`})
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(proxy.Close)

	oldProxy, hadProxy := os.LookupEnv("HDU_BROWSER_PROXY_URL")
	_ = os.Setenv("HDU_BROWSER_PROXY_URL", proxy.URL)
	t.Cleanup(func() {
		if hadProxy {
			_ = os.Setenv("HDU_BROWSER_PROXY_URL", oldProxy)
		} else {
			_ = os.Unsetenv("HDU_BROWSER_PROXY_URL")
		}
	})

	cfgDir := t.TempDir()
	cfgPath := filepath.Join(cfgDir, "config.json")
	if err := os.WriteFile(cfgPath, []byte(`{"newjw_login":{"username":"u12345678","password":"secret-pass"}}`), 0644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("HDU_LOGIN_CONFIG", cfgPath)

	outputDir := t.TempDir()
	oldWD, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(outputDir); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(oldWD) })

	service := NewService()
	if err := service.StartPersonalScheduleRefresh(); err != nil {
		t.Fatalf("StartPersonalScheduleRefresh() error = %v", err)
	}
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		status := service.Status()
		if status.Phase == "success" {
			break
		}
		if status.Phase == "error" {
			t.Fatalf("refresh failed: %s", status.Message)
		}
		time.Sleep(100 * time.Millisecond)
	}
	status := service.Status()
	if status.Phase != "success" || !status.PersonalExported || status.PersonalCount != 1 {
		t.Fatalf("unexpected status: %#v", status)
	}
}
