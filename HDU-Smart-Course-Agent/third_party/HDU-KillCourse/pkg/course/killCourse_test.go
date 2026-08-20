package course

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/cr4n5/HDU-KillCourse/client"
	"github.com/cr4n5/HDU-KillCourse/config"
)

func TestSelectCourseReturnsErrorOnFlagZero(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.Contains(r.URL.Path, "xkBcZyZzxkYzb") {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"flag":"0","msg":"人数已满"}`))
	}))
	defer ts.Close()

	oldJW := client.BaseJWURL
	client.BaseJWURL = ts.URL
	defer func() { client.BaseJWURL = oldJW }()

	cfg := &config.Config{CrossGradeEnabled: "0"}
	c := client.NewClient(cfg)
	c.SetHTTPClient(ts.Client())
	c.ClientBodyConfig = &client.ClientBodyConfig{XkkzId: map[string]string{"01": "xkkz-1"}}
	c.NjdmIDXs = "2026"
	c.ZyhIDXs = "zyh-1"

	err := SelectCourse(c, "jxb-1", "kch-1", "01", "2026", cfg)
	if err == nil {
		t.Fatal("expected error when school reports flag=0, got nil")
	}
	if !strings.Contains(err.Error(), "人数已满") {
		t.Fatalf("expected failure message in error, got: %v", err)
	}
}

func TestCancelCourseReturnsErrorOnFailure(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.Contains(r.URL.Path, "tuikBcZzxkYzb") {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`"0"`))
	}))
	defer ts.Close()

	oldJW := client.BaseJWURL
	client.BaseJWURL = ts.URL
	defer func() { client.BaseJWURL = oldJW }()

	cfg := &config.Config{}
	c := client.NewClient(cfg)
	c.SetHTTPClient(ts.Client())

	err := CancelCourse(c, "jxb-1", "kch-1", "2026", "1")
	if err == nil {
		t.Fatal("expected error when school returns failure for drop, got nil")
	}
}

func TestHandleCourseNilClientBodyConfigReturnsError(t *testing.T) {
	cfg := &config.Config{Time: config.Time{XueNian: "2026", XueQi: "1"}}
	c := client.NewClient(cfg)
	var course client.GetCourseResp
	raw := `{"items":[{"jxbmc":"(2026-2027-1)-A0001001-01","kcmc":"高等数学A","sksj":"星期一第1-2节{1-17周}","kklxmc":"主修课程","kch_id":"kch-1","jxb_id":"jxb-1","jxbzc":"2026"}]}`
	if err := json.Unmarshal([]byte(raw), &course); err != nil {
		t.Fatalf("unmarshal fixture: %v", err)
	}
	err := HandleCourse(c, cfg, &course, "(2026-2027-1)-A0001001-01", "1")
	if err == nil {
		t.Fatal("expected error for nil ClientBodyConfig, got nil")
	}
}
