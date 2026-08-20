package executor

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"encoding/base64"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync/atomic"
	"testing"

	kcclient "github.com/cr4n5/HDU-KillCourse/client"
	kcconfig "github.com/cr4n5/HDU-KillCourse/config"
)

func newMockTeachingServer(t *testing.T, selectBody string) *httptest.Server {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 1024)
	if err != nil {
		t.Fatalf("generate rsa key: %v", err)
	}
	modulus := base64.StdEncoding.EncodeToString(key.N.Bytes())
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/xtgl/login_slogin.html" && r.Method == http.MethodGet:
			w.Write([]byte(`<input name="csrftoken" value="test-csrf">`))
		case r.URL.Path == "/xtgl/login_getPublicKey.html":
			w.Header().Set("Content-Type", "application/json")
			w.Write([]byte(`{"modulus":"` + modulus + `","exponent":"10001"}`))
		case r.URL.Path == "/xtgl/login_slogin.html" && r.Method == http.MethodPost:
			http.SetCookie(w, &http.Cookie{Name: "JSESSIONID", Value: "mock-session", Path: "/"})
			http.SetCookie(w, &http.Cookie{Name: "route", Value: "mock-route", Path: "/"})
			w.Write([]byte("login ok"))
		case r.URL.Path == "/kbcx/xskbcx_cxXsgrkb.html":
			w.Header().Set("Content-Type", "application/json")
			w.Write([]byte(`{"xsxx":{"NJDM_ID":"2026","ZYH_ID":"zyh-2026"}}`))
		case r.URL.Path == "/xsxk/zzxkyzb_cxZzxkYzbIndex.html":
			w.Write([]byte(`<input name="ccdm" value="ccdm-1"><input name="bh_id" value="2026"><input name="jg_id_1" value="jg-1"><input name="xsbj" value="xsbj-1"><input name="xz" value="4"><input name="mzm" value="mzm-1"><input name="xslbdm" value="xslbdm-1"><input name="xbm" value="xbm-1"><input name="zyfx_id" value="zyfx-1"><input name="xqh_id" value="xqh-1"><a role="tab" onclick="queryCourse(this,'01','xkkz01')">主修</a><a role="tab" onclick="queryCourse(this,'10','xkkz10')">通识选修</a>`))
		case r.URL.Path == "/xsxk/zzxkyzbjk_cxJxbWithKchZzxkYzb.html":
			w.Header().Set("Content-Type", "application/json")
			w.Write([]byte(`[{"jxb_id":"kill-jxb-01","do_jxb_id":"do-kill-jxb-01"}]`))
		case r.URL.Path == "/xsxk/zzxkyzbjk_xkBcZyZzxkYzb.html":
			w.Header().Set("Content-Type", "application/json")
			w.Write([]byte(selectBody))
		case r.URL.Path == "/xsxk/zzxkyzb_cxZzxkYzbPartDisplay.html":
			w.Header().Set("Content-Type", "application/json")
			w.Write([]byte(`{"tmpList":[{"jxbmc":"(2026-2027-1)-A0001001-01"}]}`))
		case r.URL.Path == "/xsxk/zzxkyzb_tuikBcZzxkYzb.html":
			w.Write([]byte(`"1"`))
		default:
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
		}
	}))
}

func newReloginMockServer(t *testing.T, selectBodies []string, loginCount *int32, partDisplayBodies []string) *httptest.Server {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 1024)
	if err != nil {
		t.Fatalf("generate rsa key: %v", err)
	}
	modulus := base64.StdEncoding.EncodeToString(key.N.Bytes())
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/xtgl/login_slogin.html" && r.Method == http.MethodGet:
			w.Write([]byte(`<input name="csrftoken" value="test-csrf">`))
		case r.URL.Path == "/xtgl/login_getPublicKey.html":
			w.Header().Set("Content-Type", "application/json")
			w.Write([]byte(`{"modulus":"` + modulus + `","exponent":"10001"}`))
		case r.URL.Path == "/xtgl/login_slogin.html" && r.Method == http.MethodPost:
			if loginCount != nil {
				atomic.AddInt32(loginCount, 1)
			}
			http.SetCookie(w, &http.Cookie{Name: "JSESSIONID", Value: "mock-session", Path: "/"})
			http.SetCookie(w, &http.Cookie{Name: "route", Value: "mock-route", Path: "/"})
			w.Write([]byte("login ok"))
		case r.URL.Path == "/kbcx/xskbcx_cxXsgrkb.html":
			w.Header().Set("Content-Type", "application/json")
			w.Write([]byte(`{"xsxx":{"NJDM_ID":"2026","ZYH_ID":"zyh-2026"}}`))
		case r.URL.Path == "/xsxk/zzxkyzb_cxZzxkYzbIndex.html":
			w.Write([]byte(`<input name="ccdm" value="ccdm-1"><input name="bh_id" value="2026"><input name="jg_id_1" value="jg-1"><input name="xsbj" value="xsbj-1"><input name="xz" value="4"><input name="mzm" value="mzm-1"><input name="xslbdm" value="xslbdm-1"><input name="xbm" value="xbm-1"><input name="zyfx_id" value="zyfx-1"><input name="xqh_id" value="xqh-1"><a role="tab" onclick="queryCourse(this,'01','xkkz01')">主修</a><a role="tab" onclick="queryCourse(this,'10','xkkz10')">通识选修</a>`))
		case r.URL.Path == "/xsxk/zzxkyzbjk_cxJxbWithKchZzxkYzb.html":
			w.Header().Set("Content-Type", "application/json")
			w.Write([]byte(`[{"jxb_id":"kill-jxb-01","do_jxb_id":"do-kill-jxb-01"}]`))
		case r.URL.Path == "/xsxk/zzxkyzbjk_xkBcZyZzxkYzb.html":
			body := selectBodies[0]
			if len(selectBodies) > 1 {
				selectBodies = selectBodies[1:]
			}
			w.Header().Set("Content-Type", "application/json")
			w.Write([]byte(body))
		case r.URL.Path == "/xsxk/zzxkyzb_cxZzxkYzbPartDisplay.html":
			body := `{"tmpList":[{"jxbmc":"(2026-2027-1)-A0001001-01"}]}`
			if len(partDisplayBodies) > 0 {
				body = partDisplayBodies[0]
				partDisplayBodies = partDisplayBodies[1:]
			}
			w.Header().Set("Content-Type", "application/json")
			w.Write([]byte(body))
		case r.URL.Path == "/xsxk/zzxkyzb_tuikBcZzxkYzb.html":
			w.Write([]byte(`"1"`))
		default:
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
		}
	}))
}

func testConfig() *kcconfig.Config {
	return &kcconfig.Config{
		NewjwLogin: kcconfig.NewjwLogin{Username: "test-user", Password: "test-password", Level: "0"},
		CasLogin:   kcconfig.CasLogin{Username: "test-user", Password: "test-password", Level: "1"},
		Time:       kcconfig.Time{XueNian: "2026", XueQi: "1"},
		Cookies:    kcconfig.Cookies{Enabled: "1"},
	}
}

func setupExecutor(t *testing.T, server *httptest.Server, selectBody string) *Executor {
	t.Helper()
	tmp := t.TempDir()
	oldDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	if err := os.Chdir(tmp); err != nil {
		t.Fatalf("chdir: %v", err)
	}
	t.Cleanup(func() { _ = os.Chdir(oldDir) })

	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatalf("resolve test file path")
	}
	fixture := filepath.Join(filepath.Dir(thisFile), "..", "..", "testdata", "killcourse.course.sample.json")
	data, err := os.ReadFile(fixture)
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}
	coursePath := filepath.Join(tmp, "course.json")
	if err := os.WriteFile(coursePath, data, 0o644); err != nil {
		t.Fatalf("write course.json: %v", err)
	}

	oldJW := kcclient.BaseJWURL
	oldSSO := kcclient.BaseSSOURL
	kcclient.BaseJWURL = server.URL
	kcclient.BaseSSOURL = server.URL
	t.Cleanup(func() { kcclient.BaseJWURL, kcclient.BaseSSOURL = oldJW, oldSSO })

	cfg := testConfig()
	ex, err := New(cfg, coursePath)
	if err != nil {
		t.Fatalf("New executor: %v", err)
	}
	if _, err := os.Stat(filepath.Join(tmp, "config.json")); err == nil {
		t.Fatalf("New must not persist config.json into the working directory")
	}
	return ex
}

func TestRunOnceSelectSuccess(t *testing.T) {
	server := newMockTeachingServer(t, `{"flag":"1","msg":"选课成功"}`)
	defer server.Close()
	ex := setupExecutor(t, server, "")
	events, err := ex.RunOnce(context.Background(), map[string]string{"(2026-2027-1)-A0001001-01": "1"})
	if err != nil {
		t.Fatalf("RunOnce error: %v", err)
	}
	if len(events) != 1 || events[0].Status != "success" || events[0].Action != "select" {
		t.Fatalf("unexpected events: %+v", events)
	}
}

func TestRunOnceSelectFailure(t *testing.T) {
	server := newMockTeachingServer(t, `{"flag":"0","msg":"人数已满"}`)
	defer server.Close()
	ex := setupExecutor(t, server, "")
	events, err := ex.RunOnce(context.Background(), map[string]string{"(2026-2027-1)-A0001001-01": "1"})
	if err != nil {
		t.Fatalf("RunOnce error: %v", err)
	}
	if len(events) != 1 || events[0].Status != "failed" {
		t.Fatalf("expected failed event, got %+v", events)
	}
	if !strings.Contains(events[0].Message, "人数已满") {
		t.Fatalf("expected failure message, got %q", events[0].Message)
	}
}

func TestRunOnceDropSuccess(t *testing.T) {
	server := newMockTeachingServer(t, "")
	defer server.Close()
	ex := setupExecutor(t, server, "")
	events, err := ex.RunOnce(context.Background(), map[string]string{"(2026-2027-1)-A0001001-01": "0"})
	if err != nil {
		t.Fatalf("RunOnce error: %v", err)
	}
	if len(events) != 1 || events[0].Status != "success" || events[0].Action != "drop" {
		t.Fatalf("unexpected events: %+v", events)
	}
}

func TestStartWaitSelectSuccess(t *testing.T) {
	server := newMockTeachingServer(t, `{"flag":"1","msg":"选课成功"}`)
	defer server.Close()
	ex := setupExecutor(t, server, "")
	events, err := ex.StartWait(context.Background(), map[string]string{"(2026-2027-1)-A0001001-01": "1"}, 1, make(chan struct{}))
	if err != nil {
		t.Fatalf("StartWait error: %v", err)
	}
	if len(events) != 1 || events[0].Status != "success" || events[0].Action != "select" {
		t.Fatalf("unexpected events: %+v", events)
	}
}

func TestRunOnceReloginOnLoginExpired(t *testing.T) {
	var loginCount int32
	server := newReloginMockServer(t, []string{"统一身份认证", `{"flag":"1","msg":"选课成功"}`}, &loginCount, nil)
	defer server.Close()
	ex := setupExecutor(t, server, "")
	events, err := ex.RunOnce(context.Background(), map[string]string{"(2026-2027-1)-A0001001-01": "1"})
	if err != nil {
		t.Fatalf("RunOnce error: %v", err)
	}
	if len(events) != 1 || events[0].Status != "success" {
		t.Fatalf("expected success after re-login, got %+v", events)
	}
	if got := atomic.LoadInt32(&loginCount); got < 2 {
		t.Fatalf("expected at least 2 logins (initial + re-login), got %d", got)
	}
}

func TestRunOnceReloginStillFails(t *testing.T) {
	var loginCount int32
	server := newReloginMockServer(t, []string{"统一身份认证", "统一身份认证"}, &loginCount, nil)
	defer server.Close()
	ex := setupExecutor(t, server, "")
	events, err := ex.RunOnce(context.Background(), map[string]string{"(2026-2027-1)-A0001001-01": "1"})
	if err != nil {
		t.Fatalf("RunOnce error: %v", err)
	}
	if len(events) != 1 || events[0].Status != "failed" || !strings.Contains(events[0].Message, "登录过期") {
		t.Fatalf("expected persistent login-expired failure, got %+v", events)
	}
	if got := atomic.LoadInt32(&loginCount); got < 2 {
		t.Fatalf("expected re-login attempt, got %d logins", got)
	}
}

func TestStartWaitReloginOnPartDisplayExpired(t *testing.T) {
	var loginCount int32
	server := newReloginMockServer(t, []string{`{"flag":"1","msg":"选课成功"}`}, &loginCount, []string{"统一身份认证"})
	defer server.Close()
	ex := setupExecutor(t, server, "")
	events, err := ex.StartWait(context.Background(), map[string]string{"(2026-2027-1)-A0001001-01": "1"}, 1, make(chan struct{}))
	if err != nil {
		t.Fatalf("StartWait error: %v", err)
	}
	if len(events) != 1 || events[0].Status != "success" || events[0].Action != "select" {
		t.Fatalf("expected select success after re-login, got %+v", events)
	}
	if got := atomic.LoadInt32(&loginCount); got < 2 {
		t.Fatalf("expected at least 2 logins (initial + re-login), got %d", got)
	}
}

func TestWaitIntervalBacksOffAndCaps(t *testing.T) {
	cases := []struct {
		base, streak, want int
	}{
		{0, 0, 60},
		{1, 0, 1},
		{1, 1, 2},
		{10, 3, 80},
		{10, 7, 600},    // capped at waitMaxSeconds
		{60, 10, 600},   // capped
		{600, 1, 600},   // base at cap stays at cap
		{1200, 1, 2400}, // base above cap keeps growing (user override)
	}
	for _, c := range cases {
		got := waitInterval(c.base, c.streak)
		if got != c.want {
			t.Fatalf("waitInterval(%d, %d) = %d, want %d", c.base, c.streak, got, c.want)
		}
	}
}

func TestStartWaitTransientFailureKeepsWaiting(t *testing.T) {
	var loginCount int32
	server := newReloginMockServer(t, []string{`{"flag":"1","msg":"选课成功"}`}, &loginCount, []string{"无功能权限"})
	defer server.Close()
	ex := setupExecutor(t, server, "")
	events, err := ex.StartWait(context.Background(), map[string]string{"(2026-2027-1)-A0001001-01": "1"}, 1, make(chan struct{}))
	if err != nil {
		t.Fatalf("StartWait error: %v", err)
	}
	if got := atomic.LoadInt32(&loginCount); got != 1 {
		t.Fatalf("expected no re-login for a transient data error, got %d logins", got)
	}
	successSeen := false
	for _, ev := range events {
		if ev.Status == "success" && ev.CourseCode == "(2026-2027-1)-A0001001-01" {
			successSeen = true
		}
	}
	if !successSeen {
		t.Fatalf("course should still be selected after a transient failure, events=%+v", events)
	}
}

func TestRunOnceEmitsLiveEvents(t *testing.T) {
	server := newMockTeachingServer(t, `{"flag":"1","msg":"选课成功"}`)
	defer server.Close()
	ex := setupExecutor(t, server, "")
	var seen []ExecutionEvent
	ex.SetOnEvent(func(ev ExecutionEvent) { seen = append(seen, ev) })

	events, err := ex.RunOnce(context.Background(), map[string]string{"(2026-2027-1)-A0001001-01": "1"})
	if err != nil {
		t.Fatalf("RunOnce error: %v", err)
	}
	if len(events) != 1 || events[0].Status != "success" {
		t.Fatalf("unexpected final events: %+v", events)
	}
	if len(seen) < 2 {
		t.Fatalf("expected running + final live events, got %d: %+v", len(seen), seen)
	}
	if seen[0].Status != "running" || seen[len(seen)-1].Status != "success" {
		t.Fatalf("unexpected live event order: %+v", seen)
	}
}
