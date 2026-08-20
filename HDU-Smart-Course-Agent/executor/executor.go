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

		err := kcourse.HandleCourse(e.client, e.cfg, e.course, courseCode, flag)
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
	first := true
	for len(remaining) > 0 {
		if !first {
			select {
			case <-ctx.Done():
				return appendSkipped(events, remaining), ctx.Err()
			case <-done:
				return appendSkipped(events, remaining), nil
			case <-time.After(time.Duration(intervalSec) * time.Second):
			}
		}
		first = false

		for code := range remaining {
			ok, err := kcourse.GetIsCourseOk(e.client, e.cfg, e.course, code)
			if err != nil {
				ev := ExecutionEvent{
					CourseCode: code,
					Action:     "wait",
					Status:     "failed",
					Message:    "查询余量失败: " + err.Error(),
					StartedAt:  time.Now().Format(time.RFC3339),
					FinishedAt: time.Now().Format(time.RFC3339),
				}
				events = append(events, ev)
				delete(remaining, code)
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
			if err := kcourse.HandleCourse(e.client, e.cfg, e.course, code, "1"); err != nil {
				ev.Status = "failed"
				ev.Message = "选课失败: " + err.Error()
			} else {
				ev.Status = "success"
				ev.Message = "蹲课选课成功"
			}
			ev.FinishedAt = time.Now().Format(time.RFC3339)
			events = append(events, ev)
			delete(remaining, code)
		}
	}
	return events, nil
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
