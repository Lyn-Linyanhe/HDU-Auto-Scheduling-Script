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
