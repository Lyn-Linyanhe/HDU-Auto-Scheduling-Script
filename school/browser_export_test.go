package school

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"
)

func TestBrowserBridgeRejectsNonLoopbackProxyURL(t *testing.T) {
	bridge := newBrowserBridge("https://example.com/browser-proxy", nil)
	if bridge.proxyURL != defaultBrowserProxyURL {
		t.Fatalf("proxyURL = %q, want safe default %q", bridge.proxyURL, defaultBrowserProxyURL)
	}
}

func TestBrowserBridgeFetchUsesLoggedInNewJWTarget(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/targets":
			_, _ = io.WriteString(w, `[{"targetId":"target-1","type":"page","url":"https://newjw.hdu.edu.cn/jwglxt/xtgl/index_initMenu.html"}]`)
		case "/eval":
			if r.URL.Query().Get("target") != "target-1" {
				t.Fatalf("eval target = %q", r.URL.Query().Get("target"))
			}
			body, err := io.ReadAll(r.Body)
			if err != nil {
				t.Fatalf("read eval body: %v", err)
			}
			text := string(body)
			for _, want := range []string{"credentials", "https://newjw.hdu.edu.cn/jw/personal", "xnm=2026"} {
				if !strings.Contains(text, want) {
					t.Fatalf("eval body missing %q: %s", want, text)
				}
			}
			value := `{"status":200,"body":"{\"kbList\":[]}"}`
			_ = json.NewEncoder(w).Encode(map[string]any{"value": value})
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(server.Close)

	bridge := newBrowserBridge(server.URL, server.Client())
	response, err := bridge.fetch(
		"https://newjw.hdu.edu.cn/jw/personal?xnm=2026&xqm=3",
		http.MethodGet,
		map[string]string{"Accept": "application/json"},
		"",
	)
	if err != nil {
		t.Fatalf("browser fetch error = %v", err)
	}
	if response.Status != http.StatusOK || response.Body != `{"kbList":[]}` {
		t.Fatalf("browser response = %#v", response)
	}
}

func TestBrowserPersonalScheduleRejectsNullWithoutOverwriting(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/targets":
			_, _ = io.WriteString(w, `[{"targetId":"target-1","type":"page","url":"https://newjw.hdu.edu.cn/jwglxt/xtgl/index_initMenu.html"}]`)
		case "/eval":
			value, _ := json.Marshal(map[string]any{"status": http.StatusOK, "body": "null"})
			_ = json.NewEncoder(w).Encode(map[string]string{"value": string(value)})
		default:
			http.NotFound(w, r)
		}
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
	oldBytes := []byte(`{"schemaVersion":1,"items":[{"jxbmc":"old-course"}]}`)
	if err := os.WriteFile("personal-schedule.json", oldBytes, 0644); err != nil {
		t.Fatal(err)
	}

	exp := &exporter{
		browser:   newBrowserBridge(server.URL, server.Client()),
		endpoints: ExporterEndpoints{PersonalSchedule: "https://newjw.hdu.edu.cn/jwglxt/kbcx/xskbcx_cxXsgrkb.html?gnmkdm=N2151"},
	}
	if _, err := exp.exportPersonalScheduleFromBrowser(ExportRequest{}, termParams{XueNian: "2026", XueQi: "1"}); err == nil {
		t.Fatal("null browser personal schedule response unexpectedly succeeded")
	}
	got, err := os.ReadFile("personal-schedule.json")
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != string(oldBytes) {
		t.Fatalf("old personal schedule changed: got %s, want %s", got, oldBytes)
	}
}

func TestRunExportUsesAuthorizedBrowserSession(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/targets" {
			_, _ = io.WriteString(w, `[{"targetId":"target-1","type":"page","url":"https://newjw.hdu.edu.cn/jwglxt/rwlscx/rwlscx_cxRwlsIndex.html?gnmkdm=N1548&layout=default"}]`)
			return
		}
		if r.URL.Path != "/eval" {
			http.NotFound(w, r)
			return
		}
		body, _ := io.ReadAll(r.Body)
		expression := string(body)
		if strings.Contains(expression, "hasGetParam") {
			_ = json.NewEncoder(w).Encode(map[string]string{"value": `{"ready":"complete","hasGetParam":true,"href":"https://newjw.hdu.edu.cn/jwglxt/rwlscx/rwlscx_cxRwlsIndex.html?gnmkdm=N1548&layout=default"}`})
			return
		}
		course := strings.Contains(expression, "rwlscx")
		responseBody := `{"kbList":[{"kcmc":"Browser Course","xqj":"1","jc":"1-2","zcd":"1-17"}]}`
		if course {
			responseBody = `{"rows":[{"jxbmc":"(2026-2027-1)-BROWSER-01","kcmc":"Browser Course","sksj":"星期一第1-2节{1-17周}","xf":"3"}]}`
		}
		if strings.Contains(expression, "__hduCourseQueryResults") && strings.Contains(expression, "fetch(") {
			_ = json.NewEncoder(w).Encode(map[string]string{"value": "started"})
			return
		}
		if strings.Contains(expression, "__hduCourseQueryResults") {
			state, _ := json.Marshal(map[string]any{"state": "done", "status": http.StatusOK, "body": responseBody})
			_ = json.NewEncoder(w).Encode(map[string]string{"value": string(state)})
			return
		}
		value, _ := json.Marshal(map[string]any{"status": http.StatusOK, "body": responseBody})
		_ = json.NewEncoder(w).Encode(map[string]string{"value": string(value)})
	}))
	t.Cleanup(server.Close)
	oldProxy, hadProxy := os.LookupEnv("HDU_BROWSER_PROXY_URL")
	_ = os.Setenv("HDU_BROWSER_PROXY_URL", server.URL)
	t.Cleanup(func() {
		if hadProxy {
			_ = os.Setenv("HDU_BROWSER_PROXY_URL", oldProxy)
		} else {
			_ = os.Unsetenv("HDU_BROWSER_PROXY_URL")
		}
	})

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
	result, err := service.RunExport(ExportRequest{Method: "browser", XueNian: "2026", XueQi: "1"})
	if err != nil {
		t.Fatalf("RunExport() error = %v", err)
	}
	if result.Count != 1 || !result.PersonalExported || result.PersonalCount != 1 {
		t.Fatalf("unexpected browser export result: %#v", result)
	}
}

func TestRunExportUsesCachedCourseWhenBrowserCourseUnavailable(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/targets" {
			_, _ = io.WriteString(w, `[{"targetId":"target-1","type":"page","url":"https://newjw.hdu.edu.cn/jwglxt/rwlscx/rwlscx_cxRwlsIndex.html?gnmkdm=N1548&layout=default"}]`)
			return
		}
		if r.URL.Path != "/eval" {
			http.NotFound(w, r)
			return
		}
		body, _ := io.ReadAll(r.Body)
		if strings.Contains(string(body), "hasGetParam") {
			_ = json.NewEncoder(w).Encode(map[string]string{"value": `{"ready":"complete","hasGetParam":true,"href":"https://newjw.hdu.edu.cn/jwglxt/rwlscx/rwlscx_cxRwlsIndex.html?gnmkdm=N1548&layout=default"}`})
			return
		}
		if strings.Contains(string(body), "rwlscx") {
			w.WriteHeader(http.StatusInternalServerError)
			_, _ = io.WriteString(w, `{"error":"query unavailable"}`)
			return
		}
		responseBody := `{"kbList":[{"kcmc":"Browser Course","xqj":"1","jc":"1-2","zcd":"1-17"}]}`
		value, _ := json.Marshal(map[string]any{"status": http.StatusOK, "body": responseBody})
		_ = json.NewEncoder(w).Encode(map[string]string{"value": string(value)})
	}))
	t.Cleanup(server.Close)
	oldProxy, hadProxy := os.LookupEnv("HDU_BROWSER_PROXY_URL")
	_ = os.Setenv("HDU_BROWSER_PROXY_URL", server.URL)
	t.Cleanup(func() {
		if hadProxy {
			_ = os.Setenv("HDU_BROWSER_PROXY_URL", oldProxy)
		} else {
			_ = os.Unsetenv("HDU_BROWSER_PROXY_URL")
		}
	})

	outputDir := t.TempDir()
	oldWD, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(outputDir); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(oldWD) })
	if err := os.WriteFile("course.json", []byte(`{"items":[{"jxbmc":"(2026-2027-1)-CACHE-01","kcmc":"Cached Course"}]}`), 0644); err != nil {
		t.Fatal(err)
	}

	service := NewService()
	result, err := service.RunExport(ExportRequest{Method: "browser", XueNian: "2026", XueQi: "1"})
	if err != nil {
		t.Fatalf("RunExport() error = %v", err)
	}
	if result.CourseSource != "local-cache" || result.Count != 1 || !result.PersonalExported {
		t.Fatalf("unexpected cached browser export result: %#v", result)
	}
}

func TestDefaultExporterEndpointsCourseUsesFullPageEntry(t *testing.T) {
	endpoint := DefaultExporterEndpoints().Course
	for _, want := range []string{
		"gnmkdm=N1548",
		"layout=default",
	} {
		if !strings.Contains(endpoint, want) {
			t.Fatalf("course endpoint %q does not contain %q", endpoint, want)
		}
	}
	if strings.Contains(endpoint, "doType=query") {
		t.Fatalf("course endpoint should be the page entry, got query endpoint %q", endpoint)
	}
}

func TestBrowserCourseQueryUsesPageQueryContract(t *testing.T) {
	var evalBody string
	proxy := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/targets":
			_, _ = io.WriteString(w, `[{"targetId":"target-1","type":"page","url":"https://newjw.hdu.edu.cn/jwglxt/rwlscx/rwlscx_cxRwlsIndex.html?gnmkdm=N1548&layout=default"}]`)
		case "/eval":
			body, err := io.ReadAll(r.Body)
			if err != nil {
				t.Fatalf("read eval body: %v", err)
			}
			text := string(body)
			if strings.Contains(text, "hasGetParam") {
				_ = json.NewEncoder(w).Encode(map[string]string{"value": `{"ready":"complete","hasGetParam":true,"href":"https://newjw.hdu.edu.cn/jwglxt/rwlscx/rwlscx_cxRwlsIndex.html?gnmkdm=N1548&layout=default"}`})
				return
			}
			if strings.Contains(text, "fetch(") {
				evalBody = text
				_ = json.NewEncoder(w).Encode(map[string]string{"value": "started"})
				return
			}
			value := `{"state":"done","status":200,"body":"{\"total\":1,\"rows\":[{\"jxbmc\":\"(2026-2027-1)-BROWSER-01\",\"kcmc\":\"Browser Course\"}]}"}`
			_ = json.NewEncoder(w).Encode(map[string]string{"value": value})
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(proxy.Close)

	outputDir := t.TempDir()
	oldWD, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(outputDir); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(oldWD) })

	endpoints := DefaultExporterEndpoints()
	exp := newExporterWithEndpoints(endpoints, time.Minute)
	exp.browser = newBrowserBridge(proxy.URL, proxy.Client())
	result, err := exp.exportCourseFromBrowser(ExportRequest{XueNian: "2026", XueQi: "1"}, termFromRequest(ExportRequest{XueNian: "2026", XueQi: "1"}))
	if err != nil {
		t.Fatalf("exportCourseFromBrowser() error = %v", err)
	}
	if result.Count != 1 || result.CourseSource != "browser" {
		t.Fatalf("unexpected result: %#v", result)
	}
	for _, want := range []string{
		"getParam",
		`xqm:"3"`,
		"queryModel.showCount",
		"500",
		"queryModel.currentPage",
		"time",
		"Date.now",
	} {
		if !strings.Contains(evalBody, want) {
			t.Fatalf("browser query expression missing %q: %s", want, evalBody)
		}
	}
	if strings.Contains(evalBody, `"queryModel.showCount":9999`) {
		t.Fatalf("browser query expression still requests unsupported showCount=9999: %s", evalBody)
	}
	if strings.Contains(evalBody, `"queryModel.showCount":5000`) {
		t.Fatalf("browser query expression still requests unstable showCount=5000: %s", evalBody)
	}
	if strings.Contains(evalBody, `xqm:"1"`) {
		t.Fatalf("browser query expression still sends the display semester number as xqm: %s", evalBody)
	}
}

func TestBrowserCourseExportContinuesWhenTotalResultExceedsFirstPage(t *testing.T) {
	var startCount int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/targets":
			_, _ = io.WriteString(w, `[{"targetId":"target-1","type":"page","url":"https://newjw.hdu.edu.cn/jwglxt/rwlscx/rwlscx_cxRwlsIndex.html?gnmkdm=N1548&layout=default"}]`)
		case "/eval":
			body, err := io.ReadAll(r.Body)
			if err != nil {
				t.Fatalf("read eval body: %v", err)
			}
			text := string(body)
			if strings.Contains(text, "hasGetParam") {
				_ = json.NewEncoder(w).Encode(map[string]string{"value": `{"ready":"complete","hasGetParam":true,"href":"https://newjw.hdu.edu.cn/jwglxt/rwlscx/rwlscx_cxRwlsIndex.html?gnmkdm=N1548&layout=default"}`})
				return
			}
			if strings.Contains(text, "fetch(") {
				startCount++
				_ = json.NewEncoder(w).Encode(map[string]string{"value": "started"})
				return
			}
			pageItems := `[{"jxbmc":"(2026-2027-1)-BROWSER-01","kcmc":"Browser Course 1"},{"jxbmc":"(2026-2027-1)-BROWSER-02","kcmc":"Browser Course 2"},{"jxbmc":"(2026-2027-1)-BROWSER-03","kcmc":"Browser Course 3"},{"jxbmc":"(2026-2027-1)-BROWSER-04","kcmc":"Browser Course 4"},{"jxbmc":"(2026-2027-1)-BROWSER-05","kcmc":"Browser Course 5"},{"jxbmc":"(2026-2027-1)-BROWSER-06","kcmc":"Browser Course 6"},{"jxbmc":"(2026-2027-1)-BROWSER-07","kcmc":"Browser Course 7"},{"jxbmc":"(2026-2027-1)-BROWSER-08","kcmc":"Browser Course 8"},{"jxbmc":"(2026-2027-1)-BROWSER-09","kcmc":"Browser Course 9"},{"jxbmc":"(2026-2027-1)-BROWSER-10","kcmc":"Browser Course 10"}]`
			if startCount >= 2 {
				pageItems = `[{"jxbmc":"(2026-2027-1)-BROWSER-11","kcmc":"Browser Course 11"},{"jxbmc":"(2026-2027-1)-BROWSER-12","kcmc":"Browser Course 12"}]`
			}
			value, _ := json.Marshal(map[string]any{
				"state":  "done",
				"status": http.StatusOK,
				"body":   `{"totalresult":12,"items":` + pageItems + `}`,
			})
			_ = json.NewEncoder(w).Encode(map[string]string{"value": string(value)})
		default:
			http.NotFound(w, r)
		}
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

	endpoints := DefaultExporterEndpoints()
	exp := newExporterWithEndpoints(endpoints, time.Minute)
	exp.browser = newBrowserBridge(server.URL, server.Client())
	params := termFromRequest(ExportRequest{XueNian: "2026", XueQi: "1"})
	result, err := exp.exportCourseFromBrowser(ExportRequest{XueNian: "2026", XueQi: "1"}, params)
	if err != nil {
		t.Fatalf("exportCourseFromBrowser() error = %v", err)
	}
	if result.Count != 12 {
		t.Fatalf("exportCourseFromBrowser() count = %d, want 12", result.Count)
	}
	if startCount != 2 {
		t.Fatalf("course query start count = %d, want 2", startCount)
	}
}

func TestFetchCoursePageWithRetrySurvivesEmptyResponse(t *testing.T) {
	var starts int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/targets":
			_, _ = io.WriteString(w, `[{"targetId":"target-1","type":"page","url":"https://newjw.hdu.edu.cn/jwglxt/rwlscx/rwlscx_cxRwlsIndex.html?gnmkdm=N1548&layout=default"}]`)
		case "/eval":
			body, _ := io.ReadAll(r.Body)
			expression := string(body)
			if strings.Contains(expression, "hasGetParam") {
				_ = json.NewEncoder(w).Encode(map[string]string{"value": `{"ready":"complete","hasGetParam":true,"href":"https://newjw.hdu.edu.cn/jwglxt/rwlscx/rwlscx_cxRwlsIndex.html?gnmkdm=N1548&layout=default"}`})
				return
			}
			if strings.Contains(expression, "__hduCourseQueryResults") && strings.Contains(expression, "fetch(") {
				starts++
				_ = json.NewEncoder(w).Encode(map[string]string{"value": "started"})
				return
			}
			if strings.Contains(expression, "__hduCourseQueryResults") {
				bodyText := ""
				if starts >= 3 {
					bodyText = `{"rows":[{"jxbmc":"(2026-2027-1)-RETRY-01","kcmc":"Retry Course"}]}`
				}
				state, _ := json.Marshal(map[string]any{"state": "done", "status": http.StatusOK, "body": bodyText})
				_ = json.NewEncoder(w).Encode(map[string]string{"value": string(state)})
				return
			}
			http.NotFound(w, r)
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(server.Close)
	exp := newExporterWithEndpoints(DefaultExporterEndpoints(), time.Minute)
	exp.browser = newBrowserBridge(server.URL, server.Client())
	params := termFromRequest(ExportRequest{XueNian: "2026", XueQi: "1"})
	body, status, err := exp.fetchCoursePageWithRetry(params, 1)
	if err != nil {
		t.Fatalf("fetchCoursePageWithRetry() error = %v", err)
	}
	if status != http.StatusOK || !strings.Contains(body, "Retry Course") {
		t.Fatalf("unexpected body=%q status=%d", body, status)
	}
	if starts != 3 {
		t.Fatalf("start attempts = %d, want 3", starts)
	}
}

func TestBrowserBridgeFetchPrefersIndexOverCoursePage(t *testing.T) {
	var usedTarget string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/targets":
			_, _ = io.WriteString(w, `[{"targetId":"course-1","type":"page","url":"https://newjw.hdu.edu.cn/jwglxt/rwlscx/rwlscx_cxRwlsIndex.html?gnmkdm=N1548&layout=default"},{"targetId":"index-1","type":"page","url":"https://newjw.hdu.edu.cn/jwglxt/xtgl/index_initMenu.html"}]`)
		case "/eval":
			usedTarget = r.URL.Query().Get("target")
			_ = json.NewEncoder(w).Encode(map[string]string{"value": `{"status":200,"body":"{\"kbList\":[]}"}`})
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(server.Close)
	bridge := newBrowserBridge(server.URL, server.Client())
	if _, err := bridge.fetch("https://newjw.hdu.edu.cn/jw/personal", http.MethodGet, nil, ""); err != nil {
		t.Fatalf("browser fetch error = %v", err)
	}
	if usedTarget != "index-1" {
		t.Fatalf("fetch target = %q, want index-1", usedTarget)
	}
}
