package school

import (
	"strings"
	"testing"
)

func TestExtractPersonalScheduleItems(t *testing.T) {
	raw := map[string]any{
		"kbList": []any{
			map[string]any{
				"kcmc": "计算机网络",
				"xqj":  "1",
				"jc":   "3-4",
				"zcd":  "1-17周",
				"xm":   "测试教师",
				"cdmc": "第6教研楼101",
			},
		},
	}

	items := extractPersonalScheduleItems(raw)
	if len(items) != 1 {
		t.Fatalf("len(items) = %d, want 1", len(items))
	}
	item := items[0]
	if item["kcmc"] != "计算机网络" {
		t.Fatalf("kcmc = %v", item["kcmc"])
	}
	if item["jzgxx"] != "测试教师" {
		t.Fatalf("jzgxx = %v", item["jzgxx"])
	}
	timeText, _ := item["sksj"].(string)
	for _, want := range []string{"星期一", "第3-4节", "1-17周"} {
		if !strings.Contains(timeText, want) {
			t.Fatalf("sksj = %q, missing %q", timeText, want)
		}
	}
}
