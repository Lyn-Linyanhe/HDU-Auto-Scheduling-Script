package school

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"
)

type StatusResponse struct {
	Ready               bool   `json:"ready"`
	Phase               string `json:"phase"`
	Step                string `json:"step,omitempty"`
	Message             string `json:"message,omitempty"`
	Error               string `json:"error,omitempty"`
	Count               int    `json:"count,omitempty"`
	CourseName          string `json:"courseName,omitempty"`
	CourseSource        string `json:"courseSource,omitempty"`
	FileName            string `json:"fileName,omitempty"`
	OutputPath          string `json:"outputPath,omitempty"`
	PersonalCount       int    `json:"personalCount,omitempty"`
	PersonalFileName    string `json:"personalFileName,omitempty"`
	PersonalOutputPath  string `json:"personalOutputPath,omitempty"`
	PersonalExported    bool   `json:"personalExported"`
	PersonalExportError string `json:"personalExportError,omitempty"`
	UpdatedAt           string `json:"updatedAt,omitempty"`
}

type ExportRequest struct {
	Method   string `json:"method"`
	Username string `json:"username"`
	Password string `json:"password"`
	XueNian  string `json:"xueNian,omitempty"`
	XueQi    string `json:"xueQi,omitempty"`
}

type CoursePayload struct {
	SchemaVersion int              `json:"schemaVersion,omitempty"`
	Items         []map[string]any `json:"items"`
}

const CourseSchemaVersion = 1

var courseFileMu sync.RWMutex

type Service struct {
	mu            sync.RWMutex
	status        StatusResponse
	running       bool
	authenticated *exporter
	loginRequest  ExportRequest
	launch        func(func())
}

func NewService() *Service {
	return &Service{status: newStatus("idle", "idle", "等待导出课程数据", false)}
}

func (s *Service) StartExport(req ExportRequest) error {
	switch normalizeExportMethod(req.Method) {
	case "password", "":
		s.setStatus("queued", "password", "已收到账号密码，准备登录新教务并导出课程。", false, nil)
	case "browser":
		s.setStatus("queued", "browser", "准备复用已授权浏览器中的新教务登录会话。", false, nil)
	case "qr":
		s.setStatus("queued", "qr", "扫码登录暂未实现，请先使用账号密码导出。", false, nil)
	default:
		s.setStatus("queued", "export", "已进入导出流程，请完成登录并导出 course.json。", false, nil)
	}
	return nil
}

func (s *Service) Status() StatusResponse {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.status
}

func (s *Service) beginRun() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.running {
		return false
	}
	s.running = true
	return true
}

func (s *Service) endRun() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.running = false
}

func (s *Service) setStatus(phase, step, message string, ready bool, result *ExportResult) {
	s.mu.Lock()
	defer s.mu.Unlock()
	status := newStatus(phase, step, message, ready)
	if result != nil {
		status.Count = result.Count
		status.CourseName = result.CourseName
		status.CourseSource = result.CourseSource
		status.FileName = result.FileName
		status.OutputPath = result.OutputPath
		status.PersonalCount = result.PersonalCount
		status.PersonalFileName = result.PersonalFileName
		status.PersonalOutputPath = result.PersonalOutputPath
		status.PersonalExported = result.PersonalExported
		status.PersonalExportError = result.PersonalExportError
	}
	s.status = status
}

func (s *Service) setError(step string, err error) {
	message := "导出失败"
	if err != nil {
		message = err.Error()
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	status := newStatus("error", step, message, false)
	status.Error = message
	s.status = status
}

func newStatus(phase, step, message string, ready bool) StatusResponse {
	return StatusResponse{
		Ready:     ready,
		Phase:     phase,
		Step:      step,
		Message:   message,
		UpdatedAt: time.Now().Format(time.RFC3339),
	}
}

func ValidateExportRequest(req ExportRequest) error {
	method := normalizeExportMethod(req.Method)
	if method != "password" && method != "browser" && method != "qr" {
		return errors.New("登录方式无效，请选择账号密码登录")
	}
	if method == "qr" {
		return errors.New("扫码登录暂未实现，请先使用账号密码登录")
	}
	if method == "browser" {
		return validateTermRequest(req)
	}
	if strings.TrimSpace(req.Username) == "" {
		return errors.New("请填写学号或工号")
	}
	if strings.TrimSpace(req.Password) == "" {
		return errors.New("请填写密码")
	}
	return validateTermRequest(req)
}

func normalizeExportMethod(method string) string {
	method = strings.ToLower(strings.TrimSpace(method))
	if method == "" {
		return "password"
	}
	return method
}

func validateTermRequest(req ExportRequest) error {
	xueNian := strings.TrimSpace(req.XueNian)
	if xueNian != "" && !regexp.MustCompile(`^\d{4}$`).MatchString(xueNian) {
		return errors.New("学年格式应为 4 位年份，例如 2026")
	}
	xueQi := strings.TrimSpace(req.XueQi)
	if xueQi != "" && xueQi != "1" && xueQi != "2" {
		return errors.New("学期只能选择第一学期或第二学期")
	}
	return nil
}

func DecodeCoursePayload(data []byte) (*CoursePayload, error) {
	var payload CoursePayload
	if err := json.Unmarshal(data, &payload); err == nil && len(payload.Items) > 0 {
		if payload.SchemaVersion == 0 {
			payload.SchemaVersion = CourseSchemaVersion
		}
		return &payload, nil
	}

	var raw []map[string]any
	if err := json.Unmarshal(data, &raw); err == nil && len(raw) > 0 {
		return &CoursePayload{SchemaVersion: CourseSchemaVersion, Items: raw}, nil
	}

	return nil, errors.New("course.json 解析失败或为空")
}

func ReadCourseFile(name string) (*CoursePayload, error) {
	courseFileMu.RLock()
	defer courseFileMu.RUnlock()

	data, err := readCourseFileBytes(name)
	if err != nil {
		return nil, err
	}
	return DecodeCoursePayload(data)
}

func ReadCourseFileBytes(name string) ([]byte, error) {
	courseFileMu.RLock()
	defer courseFileMu.RUnlock()

	data, err := readCourseFileBytes(name)
	if err != nil {
		return nil, err
	}
	if _, err := DecodeCoursePayload(data); err != nil {
		return nil, err
	}
	return data, nil
}

func readCourseFileBytes(name string) ([]byte, error) {
	return os.ReadFile(name)
}

func WriteCourseFile(name string, data []byte) error {
	if _, err := DecodeCoursePayload(data); err != nil {
		return err
	}

	courseFileMu.Lock()
	defer courseFileMu.Unlock()
	return writeFileAtomic(name, data, 0644)
}

func writeFileAtomic(name string, data []byte, mode os.FileMode) error {
	directory := filepath.Dir(name)
	temporary, err := os.CreateTemp(directory, "."+filepath.Base(name)+".tmp-*")
	if err != nil {
		return err
	}
	temporaryName := temporary.Name()
	removeTemporary := true
	defer func() {
		_ = temporary.Close()
		if removeTemporary {
			_ = os.Remove(temporaryName)
		}
	}()

	if _, err := temporary.Write(data); err != nil {
		return err
	}
	if err := temporary.Chmod(mode); err != nil {
		return err
	}
	if err := temporary.Sync(); err != nil {
		return err
	}
	if err := temporary.Close(); err != nil {
		return err
	}
	if err := os.Rename(temporaryName, name); err != nil {
		return err
	}
	removeTemporary = false
	return nil
}

func InferCourseName(items []map[string]any) string {
	for _, item := range items {
		for _, key := range []string{"kcmc", "jxbmc", "courseName", "name"} {
			if v, ok := item[key].(string); ok && strings.TrimSpace(v) != "" {
				return strings.TrimSpace(v)
			}
		}
	}
	return "course.json"
}
