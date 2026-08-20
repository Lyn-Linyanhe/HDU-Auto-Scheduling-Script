package client

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/cr4n5/HDU-KillCourse/config"
)

func TestNewClientUsesInjectedHTTPClient(t *testing.T) {
	cfg := &config.Config{UserAgent: "test-agent"}
	c := NewClient(cfg)
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("ok"))
	}))
	defer ts.Close()

	oldJW, oldSSO := BaseJWURL, BaseSSOURL
	BaseJWURL, BaseSSOURL = ts.URL, ts.URL
	defer func() { BaseJWURL, BaseSSOURL = oldJW, oldSSO }()

	c.SetHTTPClient(ts.Client())
	body, status, err := c.Get(ts.URL+"/ping", nil)
	if err != nil {
		t.Fatalf("Get error: %v", err)
	}
	if status != http.StatusOK || string(body) != "ok" {
		t.Fatalf("unexpected response status=%d body=%q", status, body)
	}
}

func TestGetStuInfoForTermUsesProvidedTerm(t *testing.T) {
	var gotQuery string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotQuery = r.URL.RawQuery
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"xsxx":{"NJDM_ID":"2026","ZYH_ID":"zyh-2026"}}`))
	}))
	defer ts.Close()

	oldJW := BaseJWURL
	BaseJWURL = ts.URL
	defer func() { BaseJWURL = oldJW }()

	cfg := &config.Config{}
	c := NewClient(cfg)
	c.SetHTTPClient(ts.Client())

	if err := c.GetStuInfoForTerm("2026", "3"); err != nil {
		t.Fatalf("GetStuInfoForTerm error: %v", err)
	}
	if !strings.Contains(gotQuery, "xnm=2026") || !strings.Contains(gotQuery, "xqm=3") {
		t.Fatalf("expected term query, got %q", gotQuery)
	}
	if c.NjdmIDXs != "2026" || c.ZyhIDXs != "zyh-2026" {
		t.Fatalf("student info not stored: NjdmIDXs=%q ZyhIDXs=%q", c.NjdmIDXs, c.ZyhIDXs)
	}
}
