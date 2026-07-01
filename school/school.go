package school

import (
	"encoding/json"
	"errors"
	"os"
	"strings"
	"sync"
)

type StatusResponse struct {
	Ready      bool   `json:"ready"`
	Message    string `json:"message,omitempty"`
	Count      int    `json:"count,omitempty"`
	CourseName string `json:"courseName,omitempty"`
}

type ExportRequest struct {
	Method   string `json:"method"`
	Username string `json:"username"`
	Password string `json:"password"`
	XueNian  string `json:"xueNian,omitempty"`
	XueQi    string `json:"xueQi,omitempty"`
}

type CoursePayload struct {
	Items []map[string]any `json:"items"`
}

type Service struct {
	mu     sync.RWMutex
	status StatusResponse
}

func NewService() *Service {
	return &Service{status: StatusResponse{Ready: false, Message: "idle"}}
}

func (s *Service) StartExport(req ExportRequest) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	switch strings.TrimSpace(req.Method) {
	case "password":
		s.status = StatusResponse{Ready: false, Message: "已收到账号密码，准备在后台执行学校登录并导出课程。"}
	case "qr":
		s.status = StatusResponse{Ready: false, Message: "已打开扫码登录流程，完成后将自动导出课程。"}
	default:
		s.status = StatusResponse{Ready: false, Message: "已进入导出流程，请完成登录并导出 course.json。"}
	}
	return nil
}

func (s *Service) Status() StatusResponse {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.status
}

func DecodeCoursePayload(data []byte) (*CoursePayload, error) {
	var payload CoursePayload
	if err := json.Unmarshal(data, &payload); err == nil && len(payload.Items) > 0 {
		return &payload, nil
	}

	var raw []map[string]any
	if err := json.Unmarshal(data, &raw); err == nil && len(raw) > 0 {
		return &CoursePayload{Items: raw}, nil
	}

	return nil, errors.New("course.json 解析失败或为空")
}

func ReadCourseFile(name string) (*CoursePayload, error) {
	data, err := os.ReadFile(name)
	if err != nil {
		return nil, err
	}
	return DecodeCoursePayload(data)
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
