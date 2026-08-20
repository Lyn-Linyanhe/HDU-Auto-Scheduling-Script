package login

import (
	"crypto/rand"
	"crypto/rsa"
	"encoding/base64"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/cr4n5/HDU-KillCourse/client"
	"github.com/cr4n5/HDU-KillCourse/config"
)

func newJWTestServer(t *testing.T, loginBody string) *httptest.Server {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 1024)
	if err != nil {
		t.Fatalf("generate rsa key: %v", err)
	}
	modulus := base64.StdEncoding.EncodeToString(key.N.Bytes())
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.Contains(r.URL.Path, "login_slogin.html") && r.Method == http.MethodGet:
			w.Write([]byte(`<input name="csrftoken" value="test-csrf">`))
		case strings.Contains(r.URL.Path, "login_getPublicKey.html"):
			w.Header().Set("Content-Type", "application/json")
			w.Write([]byte(`{"modulus":"` + modulus + `","exponent":"10001"}`))
		case strings.Contains(r.URL.Path, "login_slogin.html") && r.Method == http.MethodPost:
			w.Write([]byte(loginBody))
		default:
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
		}
	}))
}

func TestNewjwLoginDetectsSchoolFailureMessage(t *testing.T) {
	ts := newJWTestServer(t, "用户名或密码错误")
	defer ts.Close()

	oldJW := client.BaseJWURL
	client.BaseJWURL = ts.URL
	defer func() { client.BaseJWURL = oldJW }()

	cfg := &config.Config{NewjwLogin: config.NewjwLogin{Username: "test-user", Password: "secret"}}
	c := client.NewClient(cfg)
	c.SetHTTPClient(ts.Client())

	err := NewjwLogin(c, cfg)
	if err == nil {
		t.Fatal("expected login failure error, got nil")
	}
	if !strings.Contains(err.Error(), "用户名或密码不正确") {
		t.Fatalf("unexpected error: %v", err)
	}
}
