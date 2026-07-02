package school

import (
	"os"
	"path/filepath"
	"testing"
)

func TestSampleCourseDataCanDecode(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("..", "testdata", "course.sample.json"))
	if err != nil {
		t.Fatalf("read sample course data: %v", err)
	}
	payload, err := DecodeCoursePayload(data)
	if err != nil {
		t.Fatalf("decode sample course data: %v", err)
	}
	if len(payload.Items) < 8 {
		t.Fatalf("sample course count = %d, want at least 8", len(payload.Items))
	}
}

func TestSamplePersonalScheduleCanDecode(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("..", "testdata", "personal-schedule.sample.json"))
	if err != nil {
		t.Fatalf("read sample personal schedule: %v", err)
	}
	payload, err := DecodeCoursePayload(data)
	if err != nil {
		t.Fatalf("decode sample personal schedule: %v", err)
	}
	if len(payload.Items) != 2 {
		t.Fatalf("sample personal schedule count = %d, want 2", len(payload.Items))
	}
}
