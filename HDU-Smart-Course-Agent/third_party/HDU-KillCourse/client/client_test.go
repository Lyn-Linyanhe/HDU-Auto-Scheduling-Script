package client

import (
	"net/http"
	"net/http/httptest"
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
