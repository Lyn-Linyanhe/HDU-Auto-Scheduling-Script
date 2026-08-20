// Package executor wraps the vendored HDU-KillCourse client so the Smart
// Agent can run one-shot select/drop plans and a poll-based wait mode
// without shelling out to the upstream interactive binary.
package executor

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"
	"sync"
	"time"

	kcclient "github.com/cr4n5/HDU-KillCourse/client"
	kcconfig "github.com/cr4n5/HDU-KillCourse/config"
	kcourse "github.com/cr4n5/HDU-KillCourse/pkg/course"
	klogin "github.com/cr4n5/HDU-KillCourse/pkg/login"
)

// ExecutionEvent describes one course action inside an execution run.
type ExecutionEvent struct {
	CourseCode string `json:"courseCode"`
	Action     string `json:"action"` // select/drop/wait/unknown
	Status     string `json:"status"` // pending/running/success/failed/skipped
	Message    string `json:"message"`
	StartedAt  string `json:"startedAt"`
	FinishedAt string `json:"finishedAt"`
}

// Executor owns the KillCourse client lifecycle for one authorized run.
// It is safe for serial use by the HTTP layer (one run at a time).
type Executor struct {
	mu     sync.Mutex
	cfg    *kcconfig.Config
	client *kcclient.Client
	course *kcclient.GetCourseResp
}

// New logs in with cfg, refreshes student info for the configured term and
// loads the course list from coursePath. coursePath is used instead of the
// upstream hard-coded ./course.json so the Smart Agent can point at an
// exported course file without changing the working directory.
func New(cfg *kcconfig.Config, coursePath string) (*Executor, error) {
	if cfg == nil {
		return nil, errors.New("执行器：配置为空")
	}
	if (cfg.NewjwLogin.Username == "" || cfg.NewjwLogin.Password == "") &&
		(cfg.CasLogin.Username == "" || cfg.CasLogin.Password == "") {
		return nil, errors.New("执行器：未配置登录账号密码")
	}
	if cfg.Time.XueNian == "" || cfg.Time.XueQi == "" {
		return nil, errors.New("执行器：未配置学年学期")
	}

	data, err := os.ReadFile(coursePath)
	if err != nil {
		return nil, fmt.Errorf("执行器：读取课表文件失败: %w", err)
	}
	var course kcclient.GetCourseResp
	if err := json.Unmarshal(data, &course); err != nil {
		return nil, fmt.Errorf("执行器：解析课表文件失败: %w", err)
	}
	if err := kcourse.VarifyCourse(&course, cfg); err != nil {
		return nil, fmt.Errorf("执行器：课表校验失败: %w", err)
	}

	cli, err := klogin.LoginSave(cfg, false)
	if err != nil {
		return nil, fmt.Errorf("执行器：登录失败: %w", err)
	}
	xqm, err := termXqm(cfg.Time.XueQi)
	if err != nil {
		return nil, err
	}
	if err := cli.GetStuInfoForTerm(cfg.Time.XueNian, xqm); err != nil {
		return nil, fmt.Errorf("执行器：获取学生信息失败: %w", err)
	}

	return &Executor{cfg: cfg, client: cli, course: &course}, nil
}

// RunOnce executes a plan map (course code -> "1" select / "0" drop) exactly
// once and returns one event per course. Individual action failures are
// reported in the events; only setup/global errors are returned as errors.
func (e *Executor) RunOnce(ctx context.Context, plan map[string]string) ([]ExecutionEvent, error) {
	e.mu.Lock()
	defer e.mu.Unlock()

	if err := e.ensureClientBodyConfig(); err != nil {
		return nil, fmt.Errorf("执行器：获取选课配置失败: %w", err)
	}

	events := make([]ExecutionEvent, 0, len(plan))
	for courseCode, flag := range plan {
		ev := ExecutionEvent{
			CourseCode: courseCode,
			Action:     actionForFlag(flag),
			Status:     "running",
			StartedAt:  time.Now().Format(time.RFC3339),
		}
		if ev.Action == "unknown" {
			ev.Status = "failed"
			ev.Message = "计划动作未知，仅支持 1=选课 / 0=退课"
			ev.FinishedAt = time.Now().Format(time.RFC3339)
			events = append(events, ev)
			continue
		}

		err := e.execCourseAction(courseCode, flag)
		ev.FinishedAt = time.Now().Format(time.RFC3339)
		if err != nil {
			ev.Status = "failed"
			ev.Message = err.Error()
		} else {
			ev.Status = "success"
			if ev.Action == "select" {
				ev.Message = "选课成功"
			} else {
				ev.Message = "退课成功"
			}
		}
		events = append(events, ev)

		select {
		case <-ctx.Done():
			for _, remaining := range remainingKeys(plan, courseCode) {
				events = append(events, ExecutionEvent{
					CourseCode: remaining,
					Action:     actionForFlag(plan[remaining]),
					Status:     "skipped",
					Message:    "执行已被停止",
					StartedAt:  ev.FinishedAt,
					FinishedAt: ev.FinishedAt,
				})
			}
			return events, ctx.Err()
		default:
		}
	}
	return events, nil
}

// StartWait polls every intervalSec for selectable capacity and selects a
// course as soon as it becomes available. done can be used by the UI to stop
// gracefully between polls.
func (e *Executor) StartWait(ctx context.Context, plan map[string]string, intervalSec int, done <-chan struct{}) ([]ExecutionEvent, error) {
	e.mu.Lock()
	defer e.mu.Unlock()

	if err := e.ensureClientBodyConfig(); err != nil {
		return nil, fmt.Errorf("执行器：获取选课配置失败: %w", err)
	}
	if intervalSec <= 0 {
		intervalSec = 60
	}

	remaining := make(map[string]string, len(plan))
	for code, flag := range plan {
		if flag == "1" {
			remaining[code] = flag
		}
	}

	var events []ExecutionEvent
	erroredOnce := make(map[string]bool, len(remaining))
	streak := 0
	first := true
	for len(remaining) > 0 {
		if !first {
			select {
			case <-ctx.Done():
				return appendSkipped(events, remaining), ctx.Err()
			case <-done:
				return appendSkipped(events, remaining), nil
			case <-time.After(time.Duration(waitInterval(intervalSec, streak)) * time.Second):
			}
		}
		first = false

		failedChecks := 0
		for code := range remaining {
			ok, err := kcourse.GetIsCourseOk(e.client, e.cfg, e.course, code)
			if err != nil {
				if isLoginExpiredError(err) {
					if reloginErr := e.relogin(); reloginErr == nil {
						// Session was stale; try again on the next poll instead
						// of failing the course. A stale session is not a
						// transient data error, so do not grow the backoff.
						continue
					}
				}
				failedChecks++
				if !erroredOnce[code] {
					erroredOnce[code] = true
					events = append(events, ExecutionEvent{
						CourseCode: code,
						Action:     "wait",
						Status:     "failed",
						Message:    "查询余量失败: " + err.Error() + "；已保留该课程继续蹲课，轮询将按指数退避。",
						StartedAt:  time.Now().Format(time.RFC3339),
						FinishedAt: time.Now().Format(time.RFC3339),
					})
				}
				continue
			}
			if !ok {
				continue
			}

			ev := ExecutionEvent{
				CourseCode: code,
				Action:     "select",
				Status:     "running",
				Message:    "检测到余量，开始选课",
				StartedAt:  time.Now().Format(time.RFC3339),
			}
			if err := e.execCourseAction(code, "1"); err != nil {
				ev.Status = "failed"
				ev.Message = "选课失败: " + err.Error() + "；将保留该课程继续蹲课。"
			} else {
				ev.Status = "success"
				ev.Message = "蹲课选课成功"
			}
			ev.FinishedAt = time.Now().Format(time.RFC3339)
			events = append(events, ev)
			if ev.Status == "success" {
				delete(remaining, code)
				erroredOnce[code] = false
			}
		}
		if failedChecks == 0 {
			streak = 0
		} else {
			streak++
		}
	}
	return events, nil
}

// waitMaxSeconds caps the adaptive backoff in wait mode so a persistently
// failing course cannot push the poll interval to an absurd value.
const waitMaxSeconds = 600

// waitInterval doubles `base` once per consecutive failed round (up to
// waitMaxSeconds) so transient teaching-system errors do not hammer the API.
func waitInterval(base, streak int) int {
	if base <= 0 {
		base = 60
	}
	next := base << streak
	if next < base {
		next = base
	}
	if base <= waitMaxSeconds && next > waitMaxSeconds {
		next = waitMaxSeconds
	}
	return next
}

// execCourseAction runs one select/drop action. If the teaching system reports
// a stale session ("可能登录过期"/"统一身份认证"), the executor re-logs-in once
// and retries the same action so a single mid-session expiry does not fail the
// whole run.
func (e *Executor) execCourseAction(courseCode, flag string) error {
	err := kcourse.HandleCourse(e.client, e.cfg, e.course, courseCode, flag)
	if err != nil && isLoginExpiredError(err) {
		if reloginErr := e.relogin(); reloginErr != nil {
			return reloginErr
		}
		err = kcourse.HandleCourse(e.client, e.cfg, e.course, courseCode, flag)
	}
	return err
}

func isLoginExpiredError(err error) bool {
	if err == nil {
		return false
	}
	text := err.Error()
	return strings.Contains(text, "可能登录过期") || strings.Contains(text, "统一身份认证") || strings.Contains(text, "登录过期")
}

func (e *Executor) relogin() error {
	// The selection config (ccdm/xkkz/bh_id...) is student-specific and does
	// not change when the session expires, so carry it over to the new client.
	bodyConfig := e.client.ClientBodyConfig
	cli, err := klogin.LoginSave(e.cfg, false)
	if err != nil {
		return fmt.Errorf("重新登录失败: %w", err)
	}
	cli.ClientBodyConfig = bodyConfig
	e.client = cli
	xqm, xqmErr := termXqm(e.cfg.Time.XueQi)
	if xqmErr != nil {
		return xqmErr
	}
	if err := cli.GetStuInfoForTerm(e.cfg.Time.XueNian, xqm); err != nil {
		return fmt.Errorf("重新登录后获取学生信息失败: %w", err)
	}
	return nil
}

func (e *Executor) ensureClientBodyConfig() error {
	if e.client.ClientBodyConfig != nil {
		return nil
	}
	if err := kcourse.ReadClientBodyConfig(e.client); err == nil {
		return nil
	}
	return e.client.GetClientBodyConfig()
}

func actionForFlag(flag string) string {
	switch flag {
	case "1":
		return "select"
	case "0":
		return "drop"
	default:
		return "unknown"
	}
}

func termXqm(xueQi string) (string, error) {
	switch xueQi {
	case "1":
		return "3", nil
	case "2":
		return "12", nil
	default:
		return "", errors.New("执行器：学期格式错误（仅支持 1/2）")
	}
}

func remainingKeys(plan map[string]string, after string) []string {
	seen := false
	var keys []string
	for code := range plan {
		if code == after {
			seen = true
			continue
		}
		if seen {
			keys = append(keys, code)
		}
	}
	return keys
}

func appendSkipped(events []ExecutionEvent, remaining map[string]string) []ExecutionEvent {
	now := time.Now().Format(time.RFC3339)
	for code, flag := range remaining {
		events = append(events, ExecutionEvent{
			CourseCode: code,
			Action:     actionForFlag(flag),
			Status:     "skipped",
			Message:    "蹲课已停止",
			StartedAt:  now,
			FinishedAt: now,
		})
	}
	return events
}
