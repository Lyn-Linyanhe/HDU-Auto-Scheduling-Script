package main

import (
	"bufio"
	"context"
	"crypto/sha256"
	"embed"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	neturl "net/url"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"reflect"
	"regexp"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	kcconfig "github.com/cr4n5/HDU-KillCourse/config"
	agentexecutor "hdu-smart-course-agent/executor"
)

//go:embed web/*
var webFS embed.FS

const (
	addr                          = "127.0.0.1:6899"
	schemaVersion                 = 1
	maxRequestBytes               = 80 << 20
	defaultMainBaseURL            = "http://127.0.0.1:6789"
	defaultRefreshIntervalSeconds = 60
	minRefreshIntervalSeconds     = 10
	maxRefreshIntervalSeconds     = 7200
)

type CoursePayload struct {
	SchemaVersion int              `json:"schemaVersion,omitempty"`
	Source        string           `json:"source,omitempty"`
	Term          string           `json:"term,omitempty"`
	ExportedAt    string           `json:"exportedAt,omitempty"`
	CurrentRound  *int             `json:"currentRound,omitempty"`
	Items         []map[string]any `json:"items"`
}

type Course struct {
	SchemaVersion int            `json:"schemaVersion"`
	ID            string         `json:"id"`
	DisplayCode   string         `json:"displayCode"`
	GroupID       string         `json:"groupId"`
	RawCourseCode string         `json:"rawCourseCode"`
	CourseName    string         `json:"courseName"`
	SectionName   string         `json:"sectionName"`
	Teacher       string         `json:"teacher"`
	TimeText      string         `json:"timeText"`
	Location      string         `json:"location"`
	ClassName     string         `json:"className,omitempty"`
	Credits       float64        `json:"credits"`
	SelectEnabled *bool          `json:"selectEnabled,omitempty"`
	DropEnabled   *bool          `json:"dropEnabled,omitempty"`
	SelectRounds  []int          `json:"selectRounds,omitempty"`
	Capacity      *int           `json:"capacity,omitempty"`
	Enrolled      *int           `json:"enrolled,omitempty"`
	Selected      *int           `json:"selected,omitempty"`
	Raw           map[string]any `json:"raw,omitempty"`
}

type StatusResponse struct {
	WorkspaceDir       string `json:"workspaceDir"`
	SettingsPath       string `json:"settingsPath"`
	SchedulerDir       string `json:"schedulerDir"`
	KillCourseDir      string `json:"killCourseDir"`
	CoursePath         string `json:"coursePath"`
	PersonalPath       string `json:"personalPath"`
	LivePersonalPath   string `json:"livePersonalPath"`
	LiveSyncPath       string `json:"liveSyncPath"`
	KillConfigPath     string `json:"killConfigPath"`
	CourseExists       bool   `json:"courseExists"`
	PersonalExists     bool   `json:"personalExists"`
	LivePersonalExists bool   `json:"livePersonalExists"`
	KillCourseExists   bool   `json:"killCourseExists"`
	CourseCount        int    `json:"courseCount"`
	PersonalCount      int    `json:"personalCount"`
	LivePersonalCount  int    `json:"livePersonalCount"`
	Term               string `json:"term"`
	Message            string `json:"message"`
	ActionPlanPath     string `json:"actionPlanPath"`
	CanFallback        bool   `json:"canFallback"`
	CanWriteKillConfig bool   `json:"canWriteKillConfig"`
	TargetPath         string `json:"targetPath"`
	TargetExists       bool   `json:"targetExists"`
	TargetUpdatedAt    string `json:"targetUpdatedAt,omitempty"`
	TargetCount        int    `json:"targetCount"`
}

type AgentSettings struct {
	SchedulerDir           string `json:"schedulerDir"`
	KillCourseDir          string `json:"killCourseDir"`
	MainBaseURL            string `json:"mainBaseURL"`
	AutoRefresh            *bool  `json:"autoRefresh"`
	RefreshIntervalSeconds int    `json:"refreshIntervalSeconds"`
	RefreshIntervalMinutes int    `json:"refreshIntervalMinutes,omitempty"`
}

type SettingsResponse struct {
	OK       bool           `json:"ok"`
	Path     string         `json:"path"`
	Settings AgentSettings  `json:"settings"`
	Status   StatusResponse `json:"status,omitempty"`
	Error    string         `json:"error,omitempty"`
}

type PlanRequest struct {
	TargetPayload         CoursePayload `json:"targetPayload"`
	LockedCodes           []string      `json:"lockedCodes"`
	WriteActionPlan       bool          `json:"writeActionPlan"`
	WriteKillCourseConfig bool          `json:"writeKillCourseConfig"`
}

type LiveScheduleImportRequest struct {
	Payload CoursePayload `json:"payload"`
}

type LiveScheduleResponse struct {
	OK          bool             `json:"ok"`
	HasSnapshot bool             `json:"hasSnapshot"`
	Sync        LiveScheduleSync `json:"sync,omitempty"`
	Items       []Course         `json:"items"`
	Path        string           `json:"path,omitempty"`
	Error       string           `json:"error,omitempty"`
	Warnings    []string         `json:"warnings,omitempty"`
	Status      StatusResponse   `json:"status,omitempty"`
}

type targetScheduleCandidate struct {
	Path      string
	UpdatedAt string
	Payload   CoursePayload
	Items     []Course
}

type TargetScheduleResponse struct {
	OK        bool           `json:"ok"`
	Exists    bool           `json:"exists"`
	Path      string         `json:"path,omitempty"`
	UpdatedAt string         `json:"updatedAt,omitempty"`
	Payload   CoursePayload  `json:"payload,omitempty"`
	Items     []Course       `json:"items"`
	Warnings  []string       `json:"warnings,omitempty"`
	Error     string         `json:"error,omitempty"`
	Status    StatusResponse `json:"status,omitempty"`
}

type CourseOptionsResponse struct {
	OK            bool     `json:"ok"`
	SchemaVersion int      `json:"schemaVersion"`
	Term          string   `json:"term"`
	CurrentRound  *int     `json:"currentRound"`
	Items         []Course `json:"items"`
	Warnings      []string `json:"warnings,omitempty"`
	Error         string   `json:"error,omitempty"`
}

type ClassScheduleResponse struct {
	OK            bool     `json:"ok"`
	SchemaVersion int      `json:"schemaVersion"`
	Term          string   `json:"term"`
	GroupID       string   `json:"groupId,omitempty"`
	DisplayCode   string   `json:"displayCode,omitempty"`
	ClassName     string   `json:"className,omitempty"`
	Items         []Course `json:"items"`
	Warnings      []string `json:"warnings,omitempty"`
	Error         string   `json:"error,omitempty"`
}

type ClassOption struct {
	Name  string `json:"name"`
	Count int    `json:"count"`
}

type ClassOptionsResponse struct {
	OK            bool          `json:"ok"`
	SchemaVersion int           `json:"schemaVersion"`
	Term          string        `json:"term"`
	Total         int           `json:"total"`
	Items         []ClassOption `json:"items"`
	Warnings      []string      `json:"warnings,omitempty"`
	Error         string        `json:"error,omitempty"`
}

type CourseCapacityItem struct {
	ID          string `json:"id"`
	DisplayCode string `json:"displayCode"`
	CourseName  string `json:"courseName"`
	Capacity    *int   `json:"capacity"`
	Enrolled    *int   `json:"enrolled"`
	Selected    *int   `json:"selected"`
	Remaining   *int   `json:"remaining"`
	Full        *bool  `json:"full"`
}

type CourseCapacityResponse struct {
	OK              bool                 `json:"ok"`
	SchemaVersion   int                  `json:"schemaVersion"`
	Term            string               `json:"term"`
	ObservedAt      string               `json:"observedAt"`
	SourceUpdatedAt string               `json:"sourceUpdatedAt,omitempty"`
	Source          string               `json:"source"`
	Stale           bool                 `json:"stale"`
	Items           []CourseCapacityItem `json:"items"`
	Warnings        []string             `json:"warnings,omitempty"`
	Error           string               `json:"error,omitempty"`
}

type Risk struct {
	Level   string `json:"level"`
	Message string `json:"message"`
	Code    string `json:"code,omitempty"`
}

type ValidationIssue struct {
	Level   string `json:"level"`
	Code    string `json:"code,omitempty"`
	Message string `json:"message"`
}

type PlanExplanation struct {
	Category string `json:"category"`
	Code     string `json:"code,omitempty"`
	Message  string `json:"message"`
}

type FallbackGroup struct {
	CourseBase   string   `json:"courseBase"`
	CourseName   string   `json:"courseName"`
	Preferred    string   `json:"preferred"`
	Alternatives []Course `json:"alternatives"`
}

type ActionPlan struct {
	SchemaVersion  int               `json:"schemaVersion"`
	Term           string            `json:"term"`
	Mode           string            `json:"mode"`
	Source         string            `json:"source"`
	GeneratedAt    string            `json:"generatedAt"`
	CurrentSource  string            `json:"currentSource"`
	CurrentHash    string            `json:"currentHash"`
	SelectionRound *int              `json:"selectionRound,omitempty"`
	Current        []Course          `json:"current"`
	Target         []Course          `json:"target"`
	Keep           []Course          `json:"keep"`
	Select         []Course          `json:"select"`
	Drop           []Course          `json:"drop"`
	Locked         []Course          `json:"locked"`
	FallbackGroups []FallbackGroup   `json:"fallbackGroups"`
	Risks          []Risk            `json:"risks"`
	Validation     []ValidationIssue `json:"validationIssues"`
	Explanations   []PlanExplanation `json:"explanations"`
}

type PlanResponse struct {
	OK              bool                `json:"ok"`
	Plan            ActionPlan          `json:"plan"`
	ActionPlanPath  string              `json:"actionPlanPath,omitempty"`
	ConfigPath      string              `json:"configPath,omitempty"`
	ConfigPreview   *ConfigPreview      `json:"configPreview,omitempty"`
	Readiness       *ExecutionReadiness `json:"readiness,omitempty"`
	GeneratedConfig *KillCourseConfig   `json:"generatedConfig,omitempty"`
	ConfigBlocked   bool                `json:"configBlocked,omitempty"`
	Blockers        []ValidationIssue   `json:"blockers,omitempty"`
	Warnings        []string            `json:"warnings,omitempty"`
	Error           string              `json:"error,omitempty"`
}

type DryRunRequest struct {
	Plan            ActionPlan        `json:"plan"`
	GeneratedConfig *KillCourseConfig `json:"generatedConfig,omitempty"`
}

type DryRunResponse struct {
	OK     bool            `json:"ok"`
	DryRun ExecutionDryRun `json:"dryRun,omitempty"`
	Error  string          `json:"error,omitempty"`
}

type ExecutionAuthorizationRequest struct {
	Plan               ActionPlan        `json:"plan"`
	GeneratedConfig    *KillCourseConfig `json:"generatedConfig,omitempty"`
	ConfirmationPhrase string            `json:"confirmationPhrase"`
}

type ExecutionAuthorizationResponse struct {
	OK            bool                   `json:"ok"`
	Authorization ExecutionAuthorization `json:"authorization,omitempty"`
	Error         string                 `json:"error,omitempty"`
}

type ExecutionPackageRequest struct {
	Plan            ActionPlan             `json:"plan"`
	GeneratedConfig *KillCourseConfig      `json:"generatedConfig,omitempty"`
	Authorization   ExecutionAuthorization `json:"authorization"`
}

type ExecutionPackageResponse struct {
	OK      bool             `json:"ok"`
	Package ExecutionPackage `json:"package,omitempty"`
	Error   string           `json:"error,omitempty"`
}

type ExecutionStartRequest struct {
	Plan            ActionPlan             `json:"plan"`
	GeneratedConfig *KillCourseConfig      `json:"generatedConfig,omitempty"`
	Authorization   ExecutionAuthorization `json:"authorization"`
	WaitEnabled     bool                   `json:"waitEnabled"`
}

type ExecutionStartResponse struct {
	OK       bool   `json:"ok"`
	Started  bool   `json:"started"`
	TicketID string `json:"ticketId,omitempty"`
	LogPath  string `json:"logPath,omitempty"`
	Error    string `json:"error,omitempty"`
}

type ExecutionStatusResponse struct {
	OK        bool         `json:"ok"`
	Active    bool         `json:"active"`
	TicketID  string       `json:"ticketId,omitempty"`
	StartedAt string       `json:"startedAt,omitempty"`
	Log       ExecutionLog `json:"log,omitempty"`
	Error     string       `json:"error,omitempty"`
}

type ExecutionStopResponse struct {
	OK    bool   `json:"ok"`
	Error string `json:"error,omitempty"`
}

type ExecutionLogRequest struct {
	Plan              ActionPlan        `json:"plan"`
	GeneratedConfig   *KillCourseConfig `json:"generatedConfig,omitempty"`
	WriteExecutionLog bool              `json:"writeExecutionLog"`
}

type ExecutionLogResponse struct {
	OK                  bool         `json:"ok"`
	Log                 ExecutionLog `json:"log,omitempty"`
	Path                string       `json:"path,omitempty"`
	RefreshAfterSuccess bool         `json:"refreshAfterSuccess,omitempty"`
	Error               string       `json:"error,omitempty"`
}

type FallbackRecommendationRequest struct {
	Plan                         ActionPlan   `json:"plan"`
	ExecutionLog                 ExecutionLog `json:"executionLog"`
	WriteFallbackRecommendations bool         `json:"writeFallbackRecommendations"`
}

type FallbackRecommendationResponse struct {
	OK              bool                    `json:"ok"`
	Recommendations FallbackRecommendations `json:"recommendations,omitempty"`
	Path            string                  `json:"path,omitempty"`
	Error           string                  `json:"error,omitempty"`
}

type ConfigActionPreview struct {
	Code      string `json:"code"`
	OldAction string `json:"oldAction,omitempty"`
	NewAction string `json:"newAction,omitempty"`
	Status    string `json:"status"`
}

type ConfigPreview struct {
	Path                string                `json:"path"`
	ExistingConfigFound bool                  `json:"existingConfigFound"`
	HasCASLogin         bool                  `json:"hasCasLogin"`
	HasNewJWLogin       bool                  `json:"hasNewjwLogin"`
	CookiesEnabled      string                `json:"cookiesEnabled"`
	WaitCourseEnabled   string                `json:"waitCourseEnabled"`
	SMTPEnabled         string                `json:"smtpEnabled"`
	StartTime           string                `json:"startTime"`
	XueNian             string                `json:"xueNian"`
	XueQi               string                `json:"xueQi"`
	OldActionCount      int                   `json:"oldActionCount"`
	NewActionCount      int                   `json:"newActionCount"`
	Actions             []ConfigActionPreview `json:"actions"`
}

type LiveScheduleSync struct {
	SchemaVersion int      `json:"schemaVersion"`
	Source        string   `json:"source"`
	SyncedAt      string   `json:"syncedAt"`
	LocalPath     string   `json:"localPath"`
	LivePath      string   `json:"livePath"`
	LocalCount    int      `json:"localCount"`
	LiveCount     int      `json:"liveCount"`
	LocalHash     string   `json:"localHash"`
	LiveHash      string   `json:"liveHash"`
	HasDrift      bool     `json:"hasDrift"`
	Added         []Course `json:"added"`
	Removed       []Course `json:"removed"`
	Changed       []Course `json:"changed"`
	Unchanged     []Course `json:"unchanged"`
}

type ReadinessCheck struct {
	Level   string `json:"level"`
	Passed  bool   `json:"passed"`
	Message string `json:"message"`
}

type ExecutionReadiness struct {
	Ready      bool             `json:"ready"`
	Summary    string           `json:"summary"`
	Checks     []ReadinessCheck `json:"checks"`
	CanExecute bool             `json:"canExecute"`
}

type ExecutionEvent struct {
	Level   string `json:"level"`
	Message string `json:"message"`
}

type ExecutionActionCounts struct {
	Select int `json:"select"`
	Drop   int `json:"drop"`
	Total  int `json:"total"`
}

type ExecutionDryRun struct {
	Ready              bool                  `json:"ready"`
	CanExecute         bool                  `json:"canExecute"`
	Summary            string                `json:"summary"`
	WorkspaceDir       string                `json:"workspaceDir"`
	KillCourseDir      string                `json:"killCourseDir"`
	ConfigPath         string                `json:"configPath"`
	EntryPath          string                `json:"entryPath"`
	LogPath            string                `json:"logPath"`
	Command            string                `json:"command"`
	EntryFound         bool                  `json:"entryFound"`
	ActionCounts       ExecutionActionCounts `json:"actionCounts"`
	HasDropRisk        bool                  `json:"hasDropRisk"`
	ConfirmationPhrase string                `json:"confirmationPhrase"`
	Readiness          ExecutionReadiness    `json:"readiness"`
	Events             []ExecutionEvent      `json:"events"`
	GeneratedAt        string                `json:"generatedAt"`
}

type ExecutionAuthorization struct {
	Authorized         bool                  `json:"authorized"`
	TicketID           string                `json:"ticketId"`
	PlanHash           string                `json:"planHash"`
	ConfigHash         string                `json:"configHash"`
	CreatedAt          string                `json:"createdAt"`
	ExpiresAt          string                `json:"expiresAt"`
	Command            string                `json:"command"`
	KillCourseDir      string                `json:"killCourseDir"`
	ConfigPath         string                `json:"configPath"`
	ActionCounts       ExecutionActionCounts `json:"actionCounts"`
	HasDropRisk        bool                  `json:"hasDropRisk"`
	DropRiskAccepted   bool                  `json:"dropRiskAccepted"`
	ConfirmationPhrase string                `json:"confirmationPhrase"`
}

type ExecutionPackage struct {
	Ready          bool                  `json:"ready"`
	Summary        string                `json:"summary"`
	BatchPath      string                `json:"batchPath"`
	RunbookPath    string                `json:"runbookPath"`
	ManifestPath   string                `json:"manifestPath"`
	Command        string                `json:"command"`
	KillCourseDir  string                `json:"killCourseDir"`
	ConfigPath     string                `json:"configPath"`
	EntryPath      string                `json:"entryPath"`
	LogPath        string                `json:"logPath"`
	LogStartOffset int64                 `json:"logStartOffset"`
	TicketID       string                `json:"ticketId"`
	ActionCounts   ExecutionActionCounts `json:"actionCounts"`
	Warnings       []string              `json:"warnings"`
	GeneratedAt    string                `json:"generatedAt"`
}

type ExecutionLogSummary struct {
	Total   int `json:"total"`
	Success int `json:"success"`
	Failed  int `json:"failed"`
	Pending int `json:"pending"`
	Skipped int `json:"skipped"`
}

type ExecutionLogItem struct {
	CourseCode  string   `json:"courseCode"`
	CourseName  string   `json:"courseName,omitempty"`
	TimeText    string   `json:"timeText,omitempty"`
	Action      string   `json:"action"`
	Status      string   `json:"status"`
	FailureType string   `json:"failureType,omitempty"`
	Message     string   `json:"message,omitempty"`
	RawLines    []string `json:"rawLines"`
	StartedAt   string   `json:"startedAt,omitempty"`
	FinishedAt  string   `json:"finishedAt,omitempty"`
}

type ExecutionLog struct {
	SchemaVersion int                 `json:"schemaVersion"`
	Source        string              `json:"source"`
	GeneratedAt   string              `json:"generatedAt"`
	LogPath       string              `json:"logPath"`
	PlanHash      string              `json:"planHash,omitempty"`
	ConfigHash    string              `json:"configHash,omitempty"`
	Summary       ExecutionLogSummary `json:"summary"`
	Items         []ExecutionLogItem  `json:"items"`
}

type FallbackRecommendationOption struct {
	Course         Course   `json:"course"`
	Rank           int      `json:"rank"`
	Score          int      `json:"score"`
	Conflicts      []Course `json:"conflicts"`
	Reasons        []string `json:"reasons"`
	Warnings       []string `json:"warnings"`
	SameTeacher    bool     `json:"sameTeacher"`
	HasTimeInfo    bool     `json:"hasTimeInfo"`
	TimeCompatible bool     `json:"timeCompatible"`
}

type FallbackRecommendationItem struct {
	FailedCourse   string                         `json:"failedCourse"`
	CourseName     string                         `json:"courseName,omitempty"`
	FailureType    string                         `json:"failureType"`
	Message        string                         `json:"message,omitempty"`
	Recommendation string                         `json:"recommendation"`
	Options        []FallbackRecommendationOption `json:"options"`
}

type FallbackRecommendationsSummary struct {
	FailedSelectCount int `json:"failedSelectCount"`
	WithOptions       int `json:"withOptions"`
	WithoutOptions    int `json:"withoutOptions"`
}

type FallbackRecommendations struct {
	SchemaVersion int                            `json:"schemaVersion"`
	GeneratedAt   string                         `json:"generatedAt"`
	PlanHash      string                         `json:"planHash,omitempty"`
	ExecutionHash string                         `json:"executionHash,omitempty"`
	Summary       FallbackRecommendationsSummary `json:"summary"`
	Items         []FallbackRecommendationItem   `json:"items"`
}

type KillCourseConfig struct {
	CasLogin struct {
		Username               string `json:"username"`
		Password               string `json:"password,omitempty"`
		DingDingQrLoginEnabled string `json:"dingDingQrLoginEnabled"`
		Level                  string `json:"level"`
	} `json:"cas_login"`
	NewJWLogin struct {
		Username string `json:"username"`
		Password string `json:"password,omitempty"`
		Level    string `json:"level"`
	} `json:"newjw_login"`
	UserAgent string `json:"user_agent"`
	Cookies   struct {
		JSESSIONID string `json:"JSESSIONID,omitempty"`
		Route      string `json:"route,omitempty"`
		Enabled    string `json:"enabled"`
	} `json:"cookies"`
	Time struct {
		XueNian string `json:"XueNian"`
		XueQi   string `json:"XueQi"`
	} `json:"time"`
	Course     map[string]string `json:"course"`
	WaitCourse struct {
		Interval int    `json:"interval"`
		Enabled  string `json:"enabled"`
	} `json:"wait_course"`
	SMTPEmail struct {
		Host     string `json:"host"`
		Username string `json:"username"`
		Password string `json:"password,omitempty"`
		To       string `json:"to"`
		Enabled  string `json:"enabled"`
	} `json:"smtp_email"`
	StartTime               string `json:"start_time"`
	ClientBodyConfigEnabled string `json:"ClientBodyConfigEnabled,omitempty"`
	CrossGradeEnabled       string `json:"CrossGradeEnabled,omitempty"`
}

type paths struct {
	workspace        string
	settingsPath     string
	schedulerDir     string
	downloadsDir     string
	killCourseDir    string
	coursePath       string
	personalPath     string
	livePersonalPath string
	liveSyncPath     string
	killConfigPath   string
	actionPlanPath   string
	approvalPath     string
	runBatchPath     string
	runbookPath      string
	manifestPath     string
	executionLogPath string
	fallbackRecsPath string
}

// execution-state keeps track of the single in-process KillCourse executor
// run. Only one run may be active at a time and /stop cancels its context.
var (
	execStateMu   sync.Mutex
	execCancel    context.CancelFunc
	execActive    bool
	execTicketID  string
	execStartedAt string
	execEvents    []agentexecutor.ExecutionEvent
)

// executionRunner is the subset of the executor package the Smart Agent needs.
// newExecutionRunner is a variable so tests can inject a fake that never
// touches the network.
type executionRunner interface {
	RunOnce(ctx context.Context, plan map[string]string) ([]agentexecutor.ExecutionEvent, error)
	StartWait(ctx context.Context, plan map[string]string, intervalSec int, done <-chan struct{}) ([]agentexecutor.ExecutionEvent, error)
}

var newExecutionRunner = func(cfg *kcconfig.Config, coursePath string) (executionRunner, error) {
	return agentexecutor.New(cfg, coursePath)
}

func execManagerStart(ticketID string, cancel context.CancelFunc) bool {
	execStateMu.Lock()
	defer execStateMu.Unlock()
	if execActive {
		return false
	}
	execCancel = cancel
	execActive = true
	execTicketID = ticketID
	execStartedAt = time.Now().Format(time.RFC3339)
	execEvents = nil
	return true
}

func execManagerFinish() {
	execStateMu.Lock()
	defer execStateMu.Unlock()
	if execCancel != nil {
		execCancel()
	}
	execCancel = nil
	execActive = false
	execTicketID = ""
	execStartedAt = ""
	execEvents = nil
}

// execManagerAppendEvent records a live-progress event so /status can show
// per-course progress while a long wait/select run is still in flight.
func execManagerAppendEvent(execEvent agentexecutor.ExecutionEvent) {
	execStateMu.Lock()
	defer execStateMu.Unlock()
	execEvents = append(execEvents, execEvent)
}

func execManagerStop() (bool, string, string) {
	execStateMu.Lock()
	defer execStateMu.Unlock()
	if !execActive || execCancel == nil {
		return false, "", ""
	}
	execCancel()
	return true, execTicketID, execStartedAt
}

func main() {
	listenAddr, listenPort := smartAgentListenAddress()
	mux := http.NewServeMux()
	mux.HandleFunc("/", serveStatic)
	mux.HandleFunc("/api/status", handleStatus)
	mux.HandleFunc("/api/settings", handleSettings)
	mux.HandleFunc("/api/current", handleCurrent)
	mux.HandleFunc("/api/target-schedule", handleTargetSchedule)
	mux.HandleFunc("/api/live-schedule", handleLiveSchedule)
	mux.HandleFunc("/api/live-schedule/refresh", handleLiveScheduleRefresh)
	mux.HandleFunc("/api/course", handleCourse)
	mux.HandleFunc("/api/course-options", handleCourseOptions)
	mux.HandleFunc("/api/class-schedule", handleClassSchedule)
	mux.HandleFunc("/api/class-options", handleClassOptions)
	mux.HandleFunc("/api/course-capacity", handleCourseCapacity)
	mux.HandleFunc("/api/plan", handlePlan)
	mux.HandleFunc("/api/execution/dry-run", handleExecutionDryRun)
	mux.HandleFunc("/api/execution/authorize", handleExecutionAuthorize)
	mux.HandleFunc("/api/execution/package", handleExecutionPackage)
	mux.HandleFunc("/api/execution/parse-log", handleExecutionParseLog)
	mux.HandleFunc("/api/execution/start", handleExecutionStart)
	mux.HandleFunc("/api/execution/status", handleExecutionStatus)
	mux.HandleFunc("/api/execution/stop", handleExecutionStop)
	mux.HandleFunc("/api/execution/fallback-recommendations", handleFallbackRecommendations)

	if os.Getenv("HDU_AGENT_NO_BROWSER") != "1" {
		go openBrowser("http://" + listenAddr + "/")
	}

	fmt.Println("HDU Smart Course Agent running at http://" + listenAddr)
	server := &http.Server{
		Addr:              listenAddr,
		Handler:           withLocalCORS(mux, listenPort),
		ReadHeaderTimeout: 5 * time.Second,
	}
	if err := server.ListenAndServe(); err != nil {
		panic(err)
	}
}

func smartAgentListenAddress() (string, string) {
	const defaultPort = "6899"
	port := strings.TrimSpace(os.Getenv("HDU_SMART_AGENT_PORT"))
	parsed, err := strconv.Atoi(port)
	if err != nil || parsed < 1 || parsed > 65535 {
		return addr, defaultPort
	}
	return "127.0.0.1:" + port, port
}

func serveStatic(w http.ResponseWriter, r *http.Request) {
	name := strings.TrimPrefix(path.Clean(r.URL.Path), "/")
	if name == "" || name == "." || !strings.Contains(name, ".") {
		name = "index.html"
	}
	data, err := webFS.ReadFile(path.Join("web", name))
	if err != nil {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Content-Type", contentType(name))
	_, _ = w.Write(data)
}

func handleStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	p := discoverPaths()
	resp := statusFromPaths(p)
	writeJSON(w, resp)
}

func handleSettings(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		p := discoverPaths()
		settings, _ := loadAgentSettings(p.settingsPath)
		writeJSON(w, SettingsResponse{
			OK:       true,
			Path:     p.settingsPath,
			Settings: settings,
			Status:   statusFromPaths(p),
		})
	case http.MethodPost:
		r.Body = http.MaxBytesReader(w, r.Body, 2<<20)
		body, err := io.ReadAll(r.Body)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		var settings AgentSettings
		if err := json.Unmarshal(body, &settings); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		p := discoverPaths()
		cleaned := cleanAgentSettings(settings)
		if err := validateAgentSettings(cleaned); err != nil {
			writeJSON(w, SettingsResponse{OK: false, Path: p.settingsPath, Settings: cleaned, Error: err.Error()})
			return
		}
		if err := writeJSONFile(p.settingsPath, cleaned); err != nil {
			writeJSON(w, SettingsResponse{OK: false, Path: p.settingsPath, Settings: cleaned, Error: err.Error()})
			return
		}
		next := discoverPaths()
		writeJSON(w, SettingsResponse{
			OK:       true,
			Path:     next.settingsPath,
			Settings: cleaned,
			Status:   statusFromPaths(next),
		})
	case http.MethodDelete:
		p := discoverPaths()
		if err := os.Remove(p.settingsPath); err != nil && !errors.Is(err, os.ErrNotExist) {
			writeJSON(w, SettingsResponse{OK: false, Path: p.settingsPath, Error: err.Error()})
			return
		}
		next := discoverPaths()
		writeJSON(w, SettingsResponse{
			OK:       true,
			Path:     next.settingsPath,
			Settings: defaultAgentSettings(),
			Status:   statusFromPaths(next),
		})
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func handleCurrent(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	p := discoverPaths()
	current, _, err := loadCourses(p.personalPath)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	writeJSON(w, map[string]any{"items": current})
}

func handleTargetSchedule(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	p := discoverPaths()
	candidate, warnings := discoverTargetSchedule(p)
	resp := TargetScheduleResponse{
		OK:       true,
		Exists:   candidate.Path != "",
		Items:    candidate.Items,
		Warnings: warnings,
		Status:   statusFromPaths(p),
	}
	if candidate.Path != "" {
		resp.Path = candidate.Path
		resp.UpdatedAt = candidate.UpdatedAt
		resp.Payload = candidate.Payload
	}
	writeJSON(w, resp)
}

func handleLiveScheduleRefresh(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	p := discoverPaths()
	settings, _ := loadAgentSettings(p.settingsPath)
	ctx, cancel := context.WithTimeout(r.Context(), 2*time.Minute)
	defer cancel()
	response, err := refreshFromMain(ctx, settings.MainBaseURL, p)
	if err != nil {
		w.WriteHeader(refreshHTTPStatus(err))
		writeJSON(w, LiveScheduleResponse{OK: false, Error: err.Error(), HasSnapshot: fileExists(p.livePersonalPath), Status: statusFromPaths(p)})
		return
	}
	writeJSON(w, response)
}

func handleLiveSchedule(w http.ResponseWriter, r *http.Request) {
	p := discoverPaths()
	switch r.Method {
	case http.MethodGet:
		sync, warnings := readOrBuildLiveScheduleSync(p)
		items, _, err := loadCourses(p.livePersonalPath)
		if err != nil && !errors.Is(err, os.ErrNotExist) {
			writeJSON(w, LiveScheduleResponse{OK: false, Error: err.Error(), Status: statusFromPaths(p)})
			return
		}
		if items == nil {
			items = []Course{}
		}
		writeJSON(w, LiveScheduleResponse{
			OK:          true,
			HasSnapshot: fileExists(p.livePersonalPath),
			Sync:        sync,
			Items:       nonNilCourses(items),
			Path:        p.livePersonalPath,
			Warnings:    warnings,
			Status:      statusFromPaths(p),
		})
	case http.MethodPost:
		r.Body = http.MaxBytesReader(w, r.Body, maxRequestBytes)
		body, err := io.ReadAll(r.Body)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		var req LiveScheduleImportRequest
		if err := json.Unmarshal(body, &req); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		response, err := writeLiveScheduleSnapshot(p, req.Payload, "")
		if err != nil {
			w.WriteHeader(http.StatusUnprocessableEntity)
			writeJSON(w, LiveScheduleResponse{OK: false, Error: err.Error(), HasSnapshot: fileExists(p.livePersonalPath), Status: statusFromPaths(p)})
			return
		}
		writeJSON(w, response)
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

type mainExportStatus struct {
	Phase               string `json:"phase"`
	Step                string `json:"step"`
	Message             string `json:"message"`
	Error               string `json:"error"`
	PersonalExportError string `json:"personalExportError"`
	PersonalOutputPath  string `json:"personalOutputPath"`
}

type mainRefreshHTTPError struct {
	StatusCode int
	Message    string
}

func (e *mainRefreshHTTPError) Error() string {
	if e == nil {
		return "主站刷新接口请求失败"
	}
	return e.Message
}

func newMainRefreshHTTPClient() *http.Client {
	return &http.Client{}
}

func refreshHTTPStatus(err error) int {
	if err == nil {
		return http.StatusOK
	}
	var upstream *mainRefreshHTTPError
	if errors.As(err, &upstream) {
		if upstream.StatusCode >= http.StatusBadRequest && upstream.StatusCode < http.StatusInternalServerError {
			return upstream.StatusCode
		}
		return http.StatusBadGateway
	}
	if errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled) {
		return http.StatusGatewayTimeout
	}
	if strings.Contains(err.Error(), "loopback") {
		return http.StatusBadRequest
	}
	return http.StatusBadGateway
}

func refreshFromMain(ctx context.Context, baseURL string, p paths) (LiveScheduleResponse, error) {
	if err := validateMainBaseURL(baseURL); err != nil {
		return LiveScheduleResponse{}, err
	}
	baseURL = strings.TrimRight(strings.TrimSpace(baseURL), "/")
	if baseURL == "" {
		baseURL = defaultMainBaseURL
	}
	if _, ok := ctx.Deadline(); !ok {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, 2*time.Minute)
		defer cancel()
	}
	client := newMainRefreshHTTPClient()
	startRequest, err := http.NewRequestWithContext(ctx, http.MethodPost, baseURL+"/api/export/personal-schedule", nil)
	if err != nil {
		return LiveScheduleResponse{}, err
	}
	startResponse, err := client.Do(startRequest)
	if err != nil {
		return LiveScheduleResponse{}, fmt.Errorf("无法连接主站刷新接口：%w", err)
	}
	startBody, readErr := io.ReadAll(io.LimitReader(startResponse.Body, 2<<20))
	startResponse.Body.Close()
	if readErr != nil {
		return LiveScheduleResponse{}, readErr
	}
	if startResponse.StatusCode < http.StatusOK || startResponse.StatusCode >= http.StatusMultipleChoices {
		var failure struct {
			Error string `json:"error"`
		}
		_ = json.Unmarshal(startBody, &failure)
		message := firstNonEmpty(failure.Error, strings.TrimSpace(string(startBody)), fmt.Sprintf("主站刷新接口返回 HTTP %d", startResponse.StatusCode))
		return LiveScheduleResponse{}, &mainRefreshHTTPError{StatusCode: startResponse.StatusCode, Message: message}
	}

	var completedStatus mainExportStatus
	for {
		status, err := fetchMainExportStatus(ctx, client, baseURL)
		if err != nil {
			return LiveScheduleResponse{}, fmt.Errorf("读取主站刷新状态失败：%w", err)
		}
		if strings.EqualFold(status.Phase, "success") {
			completedStatus = status
			break
		}
		if strings.EqualFold(status.Phase, "error") || status.Error != "" || status.PersonalExportError != "" {
			return LiveScheduleResponse{}, errors.New(firstNonEmpty(status.Error, status.PersonalExportError, status.Message, "主站个人课表刷新失败"))
		}
		select {
		case <-ctx.Done():
			return LiveScheduleResponse{}, fmt.Errorf("等待主站个人课表刷新超时：%w", ctx.Err())
		case <-time.After(200 * time.Millisecond):
		}
	}

	personalPath := p.personalPath
	if reported := resolveReportedPersonalSchedulePath(p, completedStatus.PersonalOutputPath); reported != "" {
		personalPath = reported
	}
	_, payload, err := loadCourses(personalPath)
	if err != nil {
		return LiveScheduleResponse{}, fmt.Errorf("主站刷新完成但个人课表文件不可读（%s）：%w", personalPath, err)
	}
	return writeLiveScheduleSnapshot(p, payload, "main-exporter")
}

func fetchMainExportStatus(ctx context.Context, client *http.Client, baseURL string) (mainExportStatus, error) {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, baseURL+"/api/export/status", nil)
	if err != nil {
		return mainExportStatus{}, err
	}
	response, err := client.Do(request)
	if err != nil {
		return mainExportStatus{}, err
	}
	defer response.Body.Close()
	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		return mainExportStatus{}, &mainRefreshHTTPError{
			StatusCode: response.StatusCode,
			Message:    fmt.Sprintf("主站状态接口返回 HTTP %d", response.StatusCode),
		}
	}
	var status mainExportStatus
	if err := json.NewDecoder(io.LimitReader(response.Body, 2<<20)).Decode(&status); err != nil {
		return mainExportStatus{}, err
	}
	return status, nil
}

func resolveReportedPersonalSchedulePath(p paths, reported string) string {
	reported = strings.TrimSpace(reported)
	if reported == "" || filepath.Base(filepath.Clean(reported)) != "personal-schedule.json" {
		return ""
	}
	if !filepath.IsAbs(reported) {
		reported = filepath.Join(p.schedulerDir, reported)
	}
	reported = filepath.Clean(reported)
	if !fileExists(reported) {
		return ""
	}
	return reported
}

func writeLiveScheduleSnapshot(p paths, payload CoursePayload, source string) (LiveScheduleResponse, error) {
	items, normalized, err := normalizePayload(payload)
	if err != nil {
		return LiveScheduleResponse{}, err
	}
	normalized.SchemaVersion = schemaVersion
	normalized.Source = firstNonEmpty(source, normalized.Source, "live-personal-schedule")
	normalized.ExportedAt = firstNonEmpty(normalized.ExportedAt, time.Now().Format(time.RFC3339))
	sync, warnings := buildLiveScheduleSyncFromItems(p, items, firstNonEmpty(source, "file-bridge"), nil)
	liveFile, err := prepareJSONFile(p.livePersonalPath, normalized)
	if err != nil {
		return LiveScheduleResponse{}, err
	}
	syncFile, err := prepareJSONFile(p.liveSyncPath, sync)
	if err != nil {
		discardPreparedJSONFile(liveFile)
		return LiveScheduleResponse{}, fmt.Errorf("写入课表同步差异失败：%w", err)
	}
	if err := commitPreparedJSONFiles(liveFile, syncFile); err != nil {
		return LiveScheduleResponse{}, fmt.Errorf("提交课表快照失败：%w", err)
	}
	return LiveScheduleResponse{
		OK:          true,
		HasSnapshot: true,
		Sync:        sync,
		Items:       nonNilCourses(items),
		Path:        p.livePersonalPath,
		Warnings:    warnings,
		Status:      statusFromPaths(p),
	}, nil
}

func handleCourse(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	p := discoverPaths()
	courses, payload, err := loadCourses(p.coursePath)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}
	writeJSON(w, map[string]any{
		"schemaVersion": schemaVersion,
		"term":          inferTerm(payload.Term, courses),
		"items":         courses,
	})
}

func handleCourseOptions(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	p := discoverPaths()
	courses, payload, err := loadCourses(p.coursePath)
	if err != nil {
		writeJSON(w, CourseOptionsResponse{OK: false, SchemaVersion: schemaVersion, Error: err.Error()})
		return
	}
	warnings := []string{}
	if payload.CurrentRound == nil {
		warnings = append(warnings, "课程数据未提供当前选课轮次；已保留课程轮次信息，计划生成时不会猜测轮次。")
	}
	writeJSON(w, CourseOptionsResponse{
		OK:            true,
		SchemaVersion: schemaVersion,
		Term:          inferTerm(payload.Term, courses),
		CurrentRound:  payload.CurrentRound,
		Items:         sortCourses(courses),
		Warnings:      warnings,
	})
}

func handleClassSchedule(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	p := discoverPaths()
	courses, payload, err := loadCourses(p.coursePath)
	if err != nil {
		writeJSON(w, ClassScheduleResponse{OK: false, SchemaVersion: schemaVersion, Error: err.Error()})
		return
	}
	groupID := strings.TrimSpace(r.URL.Query().Get("groupId"))
	displayCode := strings.TrimSpace(r.URL.Query().Get("displayCode"))
	className := strings.TrimSpace(r.URL.Query().Get("className"))
	var filtered []Course
	warnings := []string{}
	if className != "" {
		filtered = filterCoursesByClass(courses, className)
		if len(filtered) == 0 {
			warnings = append(warnings, fmt.Sprintf("课程库中未找到行政班 %q 的课程（className/jxbzc 缺失或名称不匹配）。", className))
		}
	} else {
		filtered = filterCourseLibrary(courses, groupID, displayCode)
		if groupID == "" && displayCode == "" {
			warnings = append(warnings, "未提供 groupId、displayCode 或 className，返回全部教学班课表。")
		}
	}
	writeJSON(w, ClassScheduleResponse{
		OK:            true,
		SchemaVersion: schemaVersion,
		Term:          inferTerm(payload.Term, courses),
		GroupID:       groupID,
		DisplayCode:   displayCode,
		ClassName:     className,
		Items:         sortCourses(filtered),
		Warnings:      warnings,
	})
}

func handleClassOptions(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	p := discoverPaths()
	courses, payload, err := loadCourses(p.coursePath)
	if err != nil {
		writeJSON(w, ClassOptionsResponse{OK: false, SchemaVersion: schemaVersion, Error: err.Error()})
		return
	}
	options := collectClassOptions(courses)
	warnings := []string{}
	if len(options) == 0 {
		warnings = append(warnings, "课程库中没有可识别的行政班数据（className/jxbzc 缺失）。")
	}
	writeJSON(w, ClassOptionsResponse{
		OK:            true,
		SchemaVersion: schemaVersion,
		Term:          inferTerm(payload.Term, courses),
		Total:         len(options),
		Items:         options,
		Warnings:      warnings,
	})
}

func handleCourseCapacity(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	p := discoverPaths()
	courses, payload, err := loadCourses(p.coursePath)
	if err != nil {
		writeJSON(w, CourseCapacityResponse{OK: false, SchemaVersion: schemaVersion, ObservedAt: time.Now().Format(time.RFC3339), Source: "course.json", Stale: true, Error: err.Error()})
		return
	}
	filtered := filterCourseLibrary(courses, strings.TrimSpace(r.URL.Query().Get("groupId")), strings.TrimSpace(r.URL.Query().Get("displayCode")))
	items := make([]CourseCapacityItem, 0, len(filtered))
	for _, course := range sortCourses(filtered) {
		items = append(items, capacityItem(course))
	}
	warnings := []string{"容量和人数来自本地 course.json 快照，不是教务实时接口。"}
	if len(items) == 0 {
		warnings = append(warnings, "没有匹配到教学班；容量和选课人数只来自当前 course.json 快照。")
	}
	writeJSON(w, CourseCapacityResponse{
		OK:              true,
		SchemaVersion:   schemaVersion,
		Term:            inferTerm(payload.Term, courses),
		ObservedAt:      time.Now().Format(time.RFC3339),
		SourceUpdatedAt: courseSourceUpdatedAt(p.coursePath, payload),
		Source:          "course.json",
		Stale:           true,
		Items:           items,
		Warnings:        warnings,
	})
}

func courseSourceUpdatedAt(coursePath string, payload CoursePayload) string {
	if exportedAt := strings.TrimSpace(payload.ExportedAt); exportedAt != "" {
		return exportedAt
	}
	info, err := os.Stat(coursePath)
	if err != nil {
		return ""
	}
	return info.ModTime().Format(time.RFC3339)
}

func filterCourseLibrary(courses []Course, groupID, displayCode string) []Course {
	if groupID == "" && displayCode == "" {
		return append([]Course(nil), courses...)
	}
	filtered := make([]Course, 0, len(courses))
	for _, course := range courses {
		if displayCode != "" && normalizeCode(course.DisplayCode) == normalizeCode(displayCode) {
			filtered = append(filtered, course)
			continue
		}
		if groupID != "" && courseMatchesGroup(course, groupID) {
			filtered = append(filtered, course)
		}
	}
	return filtered
}

func courseMatchesGroup(course Course, groupID string) bool {
	target := strings.TrimSpace(groupID)
	for _, value := range []string{course.GroupID, course.RawCourseCode, baseCourseCode(course.DisplayCode, course.GroupID, course.RawCourseCode)} {
		if strings.EqualFold(strings.TrimSpace(value), target) {
			return true
		}
	}
	return false
}

// splitClassNames splits a className/jxbzc value into distinct class tokens.
// The school data may join several classes with ";", "、", "," or spaces, so a
// segment-based exact match is safer than treating the whole string as one id.
func splitClassNames(value string) []string {
	fields := strings.FieldsFunc(strings.TrimSpace(value), func(r rune) bool {
		switch r {
		case ';', '；', ',', '，', '、', '/', '\t':
			return true
		}
		return r == ' ' || r == '\u3000'
	})
	seen := make(map[string]bool, len(fields))
	var result []string
	for _, field := range fields {
		field = strings.TrimSpace(field)
		if field == "" || seen[field] {
			continue
		}
		seen[field] = true
		result = append(result, field)
	}
	return result
}

func collectClassOptions(courses []Course) []ClassOption {
	counts := make(map[string]int)
	for _, course := range courses {
		for _, name := range splitClassNames(course.ClassName) {
			counts[name]++
		}
	}
	names := make([]string, 0, len(counts))
	for name := range counts {
		names = append(names, name)
	}
	sort.Strings(names)
	options := make([]ClassOption, 0, len(names))
	for _, name := range names {
		options = append(options, ClassOption{Name: name, Count: counts[name]})
	}
	return options
}

func filterCoursesByClass(courses []Course, className string) []Course {
	target := strings.TrimSpace(className)
	if target == "" {
		return nil
	}
	filtered := make([]Course, 0, len(courses))
	for _, course := range courses {
		for _, name := range splitClassNames(course.ClassName) {
			if name == target {
				filtered = append(filtered, course)
				break
			}
		}
	}
	return filtered
}

func capacityItem(course Course) CourseCapacityItem {
	people := course.Enrolled
	if people == nil {
		people = course.Selected
	}
	var remaining *int
	var full *bool
	if course.Capacity != nil && people != nil {
		value := *course.Capacity - *people
		if value < 0 {
			value = 0
		}
		isFull := value == 0
		remaining = &value
		full = &isFull
	}
	return CourseCapacityItem{
		ID:          course.ID,
		DisplayCode: course.DisplayCode,
		CourseName:  course.CourseName,
		Capacity:    course.Capacity,
		Enrolled:    course.Enrolled,
		Selected:    course.Selected,
		Remaining:   remaining,
		Full:        full,
	}
}

func handlePlan(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, maxRequestBytes)
	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	var req PlanRequest
	if err := json.Unmarshal(body, &req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	p := discoverPaths()
	plan, warnings, err := buildActionPlan(req, p)
	if err != nil {
		writeJSON(w, PlanResponse{OK: false, Error: err.Error(), Warnings: warnings})
		return
	}

	blockers := blockingValidationIssues(plan.Validation)
	resp := PlanResponse{OK: true, Plan: plan, Warnings: warnings, ConfigBlocked: len(blockers) > 0, Blockers: blockers}
	var config KillCourseConfig
	var preview ConfigPreview
	var configErr error
	if resp.ConfigBlocked {
		resp.Warnings = append(resp.Warnings, fmt.Sprintf("execution config blocked by %d plan validation error(s)", len(blockers)))
	} else {
		config, preview, configErr = buildKillCourseConfig(plan, p.killConfigPath)
	}
	if configErr == nil && !resp.ConfigBlocked {
		resp.ConfigPreview = &preview
		readiness := buildExecutionReadiness(plan, config, preview, p)
		resp.Readiness = &readiness
		if req.WriteKillCourseConfig {
			resp.GeneratedConfig = &config
		} else {
			redacted := redactedKillCourseConfig(config)
			resp.GeneratedConfig = &redacted
		}
	} else if configErr != nil {
		resp.Warnings = append(resp.Warnings, configErr.Error())
	}
	if req.WriteActionPlan {
		if err := writeJSONFile(p.actionPlanPath, plan); err != nil {
			writeJSON(w, PlanResponse{OK: false, Error: err.Error(), Warnings: warnings})
			return
		}
		resp.ActionPlanPath = p.actionPlanPath
	}
	if req.WriteKillCourseConfig {
		if resp.ConfigBlocked {
			writeJSON(w, resp)
			return
		}
		if configErr != nil {
			writeJSON(w, PlanResponse{OK: false, Error: configErr.Error(), Warnings: resp.Warnings})
			return
		}
		if err := writeJSONFile(p.killConfigPath, config); err != nil {
			writeJSON(w, PlanResponse{OK: false, Error: err.Error(), Warnings: resp.Warnings})
			return
		}
		resp.ConfigPath = p.killConfigPath
	}
	writeJSON(w, resp)
}

func handleExecutionDryRun(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, maxRequestBytes)
	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	var req DryRunRequest
	if err := json.Unmarshal(body, &req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	p := discoverPaths()
	dryRun, err := buildExecutionDryRun(req, p)
	if err != nil {
		writeJSON(w, DryRunResponse{OK: false, Error: err.Error()})
		return
	}
	writeJSON(w, DryRunResponse{OK: true, DryRun: dryRun})
}

func handleExecutionAuthorize(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, maxRequestBytes)
	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	var req ExecutionAuthorizationRequest
	if err := json.Unmarshal(body, &req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	p := discoverPaths()
	authorization, err := buildExecutionAuthorization(req, p)
	if err != nil {
		writeJSON(w, ExecutionAuthorizationResponse{OK: false, Error: err.Error()})
		return
	}
	if err := writeJSONFile(p.approvalPath, authorization); err != nil {
		writeJSON(w, ExecutionAuthorizationResponse{OK: false, Error: err.Error()})
		return
	}
	writeJSON(w, ExecutionAuthorizationResponse{OK: true, Authorization: authorization})
}

func handleExecutionPackage(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, maxRequestBytes)
	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	var req ExecutionPackageRequest
	if err := json.Unmarshal(body, &req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	p := discoverPaths()
	pkg, err := buildExecutionPackage(req, p)
	if err != nil {
		writeJSON(w, ExecutionPackageResponse{OK: false, Error: err.Error()})
		return
	}
	if err := writeExecutionPackageFiles(pkg, req.Authorization, p); err != nil {
		writeJSON(w, ExecutionPackageResponse{OK: false, Error: err.Error()})
		return
	}
	writeJSON(w, ExecutionPackageResponse{OK: true, Package: pkg})
}

func handleExecutionParseLog(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, maxRequestBytes)
	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	var req ExecutionLogRequest
	if err := json.Unmarshal(body, &req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	p := discoverPaths()
	logResult, err := buildExecutionLog(req, p)
	if err != nil {
		writeJSON(w, ExecutionLogResponse{OK: false, Error: err.Error()})
		return
	}
	resp := ExecutionLogResponse{
		OK:                  true,
		Log:                 logResult,
		RefreshAfterSuccess: executionLogNeedsRefresh(logResult),
	}
	if req.WriteExecutionLog {
		if err := writeJSONFile(p.executionLogPath, logResult); err != nil {
			writeJSON(w, ExecutionLogResponse{OK: false, Error: err.Error()})
			return
		}
		resp.Path = p.executionLogPath
	}
	writeJSON(w, resp)
}

func handleExecutionStart(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, maxRequestBytes)
	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	var req ExecutionStartRequest
	if err := json.Unmarshal(body, &req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	p := discoverPaths()

	dryRun, cfg, err := buildStartDryRun(req, p)
	if err != nil {
		writeJSON(w, ExecutionStartResponse{OK: false, Error: err.Error()})
		return
	}
	if err := validateExecutionAuthorization(req.Authorization, req.Plan, &cfg, dryRun, p.approvalPath); err != nil {
		writeJSON(w, ExecutionStartResponse{OK: false, Error: err.Error()})
		return
	}
	if !hasUsableKillCredentials(cfg) {
		writeJSON(w, ExecutionStartResponse{OK: false, Error: "KillCourse 配置缺少可用的登录凭据（账号密码或 cookies），请先完成登录。"})
		return
	}
	if !fileExists(p.coursePath) {
		writeJSON(w, ExecutionStartResponse{OK: false, Error: "未找到课程库文件：" + p.coursePath})
		return
	}
	kcCfg, err := killCourseConfigToUpstream(cfg)
	if err != nil {
		writeJSON(w, ExecutionStartResponse{OK: false, Error: err.Error()})
		return
	}
	planMap := make(map[string]string, len(cfg.Course))
	for code, action := range cfg.Course {
		planMap[code] = action
	}

	if err := writeJSONFile(p.actionPlanPath, req.Plan); err != nil {
		writeJSON(w, ExecutionStartResponse{OK: false, Error: "写入执行计划失败：" + err.Error()})
		return
	}
	ctx, cancel := context.WithCancel(context.Background())
	if !execManagerStart(req.Authorization.TicketID, cancel) {
		writeJSON(w, ExecutionStartResponse{OK: false, Error: "已有执行任务正在运行，请先停止或等待其完成。"})
		return
	}

	go func() {
		defer execManagerFinish()
		ex, newErr := newExecutionRunner(kcCfg, p.coursePath)
		if newErr != nil {
			_ = writeExecutionEventsLog(p, req.Authorization, nil, newErr)
			return
		}
		if streamingRunner, ok := ex.(interface {
			SetOnEvent(func(agentexecutor.ExecutionEvent))
		}); ok {
			streamingRunner.SetOnEvent(execManagerAppendEvent)
		}
		var runEvents []agentexecutor.ExecutionEvent
		var runErr error
		if req.WaitEnabled {
			runEvents, runErr = ex.StartWait(ctx, planMap, cfg.WaitCourse.Interval, nil)
		} else {
			runEvents, runErr = ex.RunOnce(ctx, planMap)
		}
		_ = writeExecutionEventsLog(p, req.Authorization, runEvents, runErr)
	}()

	writeJSON(w, ExecutionStartResponse{OK: true, Started: true, TicketID: req.Authorization.TicketID, LogPath: p.executionLogPath})
}

func handleExecutionStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	p := discoverPaths()
	execStateMu.Lock()
	active := execActive
	ticketID := execTicketID
	startedAt := execStartedAt
	events := append([]agentexecutor.ExecutionEvent(nil), execEvents...)
	execStateMu.Unlock()
	resp := ExecutionStatusResponse{OK: true, Active: active, TicketID: ticketID, StartedAt: startedAt}
	if active || len(events) > 0 {
		resp.Log = executionLogFromEvents(p, ExecutionAuthorization{}, events, nil)
	} else if data, err := os.ReadFile(p.executionLogPath); err == nil {
		var runLog ExecutionLog
		if json.Unmarshal(data, &runLog) == nil {
			resp.Log = runLog
		}
	}
	writeJSON(w, resp)
}

func handleExecutionStop(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	execManagerStop()
	writeJSON(w, ExecutionStopResponse{OK: true})
}

func buildStartDryRun(req ExecutionStartRequest, p paths) (ExecutionDryRun, KillCourseConfig, error) {
	if err := validatePlanExecutionEligibility(req.Plan); err != nil {
		return ExecutionDryRun{}, KillCourseConfig{}, err
	}
	if len(req.Plan.Select) == 0 && len(req.Plan.Drop) == 0 {
		return ExecutionDryRun{}, KillCourseConfig{}, errors.New("当前执行计划没有选课或退课动作")
	}
	var cfg KillCourseConfig
	if req.GeneratedConfig != nil && len(req.GeneratedConfig.Course) > 0 {
		cfg = *req.GeneratedConfig
	} else {
		generated, _, err := buildKillCourseConfig(req.Plan, p.killConfigPath)
		if err != nil {
			return ExecutionDryRun{}, KillCourseConfig{}, err
		}
		cfg = generated
	}
	command, entryFound := killCourseCommand(p)
	hasDrop := len(req.Plan.Drop) > 0
	return ExecutionDryRun{
		Ready:              true,
		CanExecute:         true,
		Summary:            "内置执行器检查通过",
		WorkspaceDir:       p.workspace,
		KillCourseDir:      p.killCourseDir,
		ConfigPath:         p.killConfigPath,
		EntryPath:          killCourseEntryPath(p),
		LogPath:            p.executionLogPath,
		Command:            command,
		EntryFound:         entryFound,
		ActionCounts:       countExecutionActions(cfg.Course),
		HasDropRisk:        hasDrop,
		ConfirmationPhrase: expectedConfirmationPhrase(hasDrop),
		GeneratedAt:        time.Now().Format(time.RFC3339),
	}, cfg, nil
}

func hasUsableKillCredentials(cfg KillCourseConfig) bool {
	if hasUsableCookie(cfg) {
		return true
	}
	return (cfg.CasLogin.Username != "" && cfg.CasLogin.Password != "") ||
		(cfg.NewJWLogin.Username != "" && cfg.NewJWLogin.Password != "")
}

func killCourseConfigToUpstream(cfg KillCourseConfig) (*kcconfig.Config, error) {
	data, err := json.Marshal(cfg)
	if err != nil {
		return nil, fmt.Errorf("序列化 KillCourse 配置失败: %w", err)
	}
	var kcCfg kcconfig.Config
	if err := json.Unmarshal(data, &kcCfg); err != nil {
		return nil, fmt.Errorf("解析 KillCourse 配置失败: %w", err)
	}
	return &kcCfg, nil
}

func executionLogFromEvents(p paths, auth ExecutionAuthorization, events []agentexecutor.ExecutionEvent, runErr error) ExecutionLog {
	now := time.Now().Format(time.RFC3339)
	items := make([]ExecutionLogItem, 0, len(events)+1)
	for _, ev := range events {
		items = append(items, ExecutionLogItem{
			CourseCode: ev.CourseCode,
			Action:     ev.Action,
			Status:     ev.Status,
			Message:    ev.Message,
			StartedAt:  ev.StartedAt,
			FinishedAt: ev.FinishedAt,
			RawLines:   []string{ev.Message},
		})
	}
	if runErr != nil && !errors.Is(runErr, context.Canceled) {
		items = append(items, ExecutionLogItem{
			CourseCode: "",
			Action:     "unknown",
			Status:     "failed",
			Message:    runErr.Error(),
			FinishedAt: now,
			RawLines:   []string{runErr.Error()},
		})
	}
	if len(items) == 0 {
		items = append(items, ExecutionLogItem{
			CourseCode: "",
			Action:     "unknown",
			Status:     "skipped",
			Message:    "执行已停止，未产生动作结果",
			FinishedAt: now,
			RawLines:   []string{},
		})
	}
	return ExecutionLog{
		SchemaVersion: schemaVersion,
		Source:        "executor",
		GeneratedAt:   now,
		LogPath:       p.executionLogPath,
		PlanHash:      auth.PlanHash,
		ConfigHash:    auth.ConfigHash,
		Summary:       summarizeExecutionLog(items),
		Items:         items,
	}
}

func writeExecutionEventsLog(p paths, auth ExecutionAuthorization, events []agentexecutor.ExecutionEvent, runErr error) error {
	runLog := executionLogFromEvents(p, auth, events, runErr)
	return writeJSONFile(p.executionLogPath, runLog)
}

func handleFallbackRecommendations(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, maxRequestBytes)
	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	var req FallbackRecommendationRequest
	if err := json.Unmarshal(body, &req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	p := discoverPaths()
	recommendations, err := buildFallbackRecommendations(req, p)
	if err != nil {
		writeJSON(w, FallbackRecommendationResponse{OK: false, Error: err.Error()})
		return
	}
	resp := FallbackRecommendationResponse{OK: true, Recommendations: recommendations}
	if req.WriteFallbackRecommendations {
		if err := writeJSONFile(p.fallbackRecsPath, recommendations); err != nil {
			writeJSON(w, FallbackRecommendationResponse{OK: false, Error: err.Error()})
			return
		}
		resp.Path = p.fallbackRecsPath
	}
	writeJSON(w, resp)
}

func buildActionPlan(req PlanRequest, p paths) (ActionPlan, []string, error) {
	var warnings []string
	target, targetPayload, err := normalizePayload(req.TargetPayload)
	if err != nil {
		return ActionPlan{}, warnings, err
	}
	if skipped := len(req.TargetPayload.Items) - len(target); skipped > 0 {
		warnings = append(warnings, fmt.Sprintf("目标课表中有 %d 条记录缺少完整教学班号，已跳过", skipped))
	}
	if len(target) == 0 {
		return ActionPlan{}, warnings, errors.New("目标课表为空，请导入排课助手导出的 JSON")
	}
	for _, warn := range validateTarget(target) {
		warnings = append(warnings, warn)
	}

	current, currentPayload, currentSource, err := loadCurrentCourses(p)
	if err != nil {
		current = []Course{}
		currentSource = "empty"
		warnings = append(warnings, "personal-schedule.json 缺失或不可解析，当前课表按空处理")
	}

	allCourses, allPayload, err := loadCourses(p.coursePath)
	if err != nil {
		allCourses = []Course{}
		warnings = append(warnings, "course.json 缺失或不可解析，无法生成备选教学班")
	}

	planTerm := inferTerm(targetPayload.Term, currentPayload.Term, target)
	selectionRound := targetPayload.CurrentRound
	if selectionRound == nil {
		selectionRound = allPayload.CurrentRound
	}
	lockedSet := makeCodeSet(req.LockedCodes)
	currentMap := mapByCode(current)
	targetMap := mapByCode(target)
	issues := validatePlanInputs(target, current, allCourses, lockedSet, warnings, planTerm)

	plan := ActionPlan{
		SchemaVersion:  schemaVersion,
		Term:           planTerm,
		Mode:           "full",
		Source:         firstNonEmpty(targetPayload.Source, "scheduler-export"),
		GeneratedAt:    time.Now().Format(time.RFC3339),
		CurrentSource:  currentSource,
		CurrentHash:    scheduleHash(current),
		SelectionRound: selectionRound,
		Current:        sortCourses(current),
		Target:         sortCourses(target),
		Validation:     issues,
		Risks: []Risk{{
			Level:   "high",
			Message: "完整执行计划包含退课动作。真实执行前请确认退课后新教学班未选上可能导致原课程丢失。",
		}},
	}

	for code, item := range targetMap {
		if _, ok := currentMap[code]; ok {
			plan.Keep = append(plan.Keep, item)
			plan.Explanations = append(plan.Explanations, PlanExplanation{
				Category: "keep",
				Code:     code,
				Message:  fmt.Sprintf("%s 同时存在于当前课表和目标课表，因此保留。", courseTitle(item)),
			})
		} else {
			plan.Select = append(plan.Select, item)
			plan.Explanations = append(plan.Explanations, PlanExplanation{
				Category: "select",
				Code:     code,
				Message:  fmt.Sprintf("%s 存在于目标课表但不在当前课表中，因此列为选课。", courseTitle(item)),
			})
		}
	}
	for code, item := range currentMap {
		if lockedSet[code] {
			plan.Locked = append(plan.Locked, item)
			plan.Explanations = append(plan.Explanations, PlanExplanation{
				Category: "locked",
				Code:     code,
				Message:  fmt.Sprintf("%s 已被用户锁定，不会自动退课。", courseTitle(item)),
			})
			if _, ok := targetMap[code]; !ok {
				plan.Risks = append(plan.Risks, Risk{
					Level:   "high",
					Code:    code,
					Message: "锁定课程不在目标课表中，已阻止自动退课。",
				})
			}
			continue
		}
		if _, ok := targetMap[code]; !ok {
			plan.Drop = append(plan.Drop, item)
			plan.Explanations = append(plan.Explanations, PlanExplanation{
				Category: "drop",
				Code:     code,
				Message:  fmt.Sprintf("%s 当前已选但不在目标课表中，因此完整执行计划会退课。", courseTitle(item)),
			})
			plan.Risks = append(plan.Risks, Risk{
				Level:   "high",
				Code:    code,
				Message: "该课程在当前课表中存在但不在目标课表中，完整执行计划会将其退课。",
			})
		}
	}
	for _, code := range req.LockedCodes {
		normalized := normalizeCode(code)
		if normalized == "" || currentMap[normalized].DisplayCode != "" {
			continue
		}
		if item, ok := targetMap[normalized]; ok {
			plan.Locked = append(plan.Locked, item)
			plan.Explanations = append(plan.Explanations, PlanExplanation{
				Category: "locked",
				Code:     normalized,
				Message:  fmt.Sprintf("%s 被标记为锁定课程，但它只存在于目标课表中。", courseTitle(item)),
			})
		}
	}

	plan.Validation = append(plan.Validation, courseActionCapabilityIssues(plan, allCourses, selectionRound)...)
	plan.Validation = append(plan.Validation, liveSchedulePlanIssues(plan, p)...)
	plan.Keep = sortCourses(plan.Keep)
	plan.Select = sortCourses(plan.Select)
	plan.Drop = sortCourses(plan.Drop)
	plan.Locked = sortCourses(dedupeCourses(plan.Locked))
	plan.FallbackGroups = buildFallbackGroups(plan.Select, allCourses)
	plan.Explanations = append(plan.Explanations, fallbackExplanations(plan.Select, plan.FallbackGroups)...)
	plan.Explanations = sortExplanations(plan.Explanations)
	plan.Validation = sortValidationIssues(dedupeValidationIssues(plan.Validation))
	return plan, warnings, nil
}

func validatePlanInputs(target []Course, current []Course, allCourses []Course, lockedSet map[string]bool, warnings []string, planTerm string) []ValidationIssue {
	var issues []ValidationIssue
	for _, warning := range warnings {
		level := "warning"
		if strings.Contains(warning, "course.json") {
			level = "warning"
		}
		issues = append(issues, ValidationIssue{Level: level, Message: warning})
	}
	issues = append(issues, duplicateIssues("目标课表", target)...)
	issues = append(issues, duplicateIssues("当前课表", current)...)

	issues = append(issues, strictTargetIssues(target, allCourses, planTerm)...)
	if len(allCourses) == 0 {
		issues = append(issues, ValidationIssue{
			Level:   "warning",
			Message: "缺少全校课表，无法确认同课程号备选教学班。",
		})
	}

	targetMap := mapByCode(target)
	currentMap := mapByCode(current)
	for code := range lockedSet {
		_, inCurrent := currentMap[code]
		_, inTarget := targetMap[code]
		switch {
		case !inCurrent && !inTarget:
			issues = append(issues, ValidationIssue{
				Level:   "warning",
				Code:    code,
				Message: "锁定课程既不在当前课表也不在目标课表中，可能来自过期选择。",
			})
		case inCurrent && !inTarget:
			issues = append(issues, ValidationIssue{
				Level:   "error",
				Code:    code,
				Message: "锁定课程不在目标课表中，系统会阻止自动退课；目标方案可能无法完整达成。",
			})
		}
	}
	return issues
}

func courseActionCapabilityIssues(plan ActionPlan, allCourses []Course, selectionRound *int) []ValidationIssue {
	library := mapByCode(allCourses)
	var issues []ValidationIssue
	for _, item := range plan.Select {
		course := mergeCourseCapabilities(item, library)
		if course.SelectEnabled != nil && !*course.SelectEnabled {
			issues = append(issues, ValidationIssue{Level: "error", Code: item.DisplayCode, Message: fmt.Sprintf("课程 %s 明确标记为不可选，已阻止生成选课动作。", courseTitle(item))})
		}
		people := course.Enrolled
		if people == nil {
			people = course.Selected
		}
		if course.Capacity != nil && people != nil && *people >= *course.Capacity {
			issues = append(issues, ValidationIssue{Level: "error", Code: item.DisplayCode, Message: fmt.Sprintf("课程 %s 已满（容量 %d，人次 %d），已阻止生成选课动作。", courseTitle(item), *course.Capacity, *people)})
		}
		if len(course.SelectRounds) == 0 {
			continue
		}
		if selectionRound == nil {
			issues = append(issues, ValidationIssue{Level: "error", Code: item.DisplayCode, Message: fmt.Sprintf("课程 %s 提供了选课轮次（%s），但当前选课轮次未知，已阻止生成选课动作。", courseTitle(item), formatSelectionRounds(course.SelectRounds))})
			continue
		}
		if !containsSelectionRound(course.SelectRounds, *selectionRound) {
			issues = append(issues, ValidationIssue{Level: "error", Code: item.DisplayCode, Message: fmt.Sprintf("课程 %s 不支持当前选课轮次 %d（可选轮次：%s），已阻止生成选课动作。", courseTitle(item), *selectionRound, formatSelectionRounds(course.SelectRounds))})
		}
	}
	for _, item := range plan.Drop {
		course := mergeCourseCapabilities(item, library)
		if course.DropEnabled != nil && !*course.DropEnabled {
			issues = append(issues, ValidationIssue{Level: "error", Code: item.DisplayCode, Message: fmt.Sprintf("课程 %s 明确标记为不可退，已阻止生成退课动作。", courseTitle(item))})
		}
	}
	return issues
}

func mergeCourseCapabilities(item Course, library map[string]Course) Course {
	candidate, ok := library[normalizeCode(item.DisplayCode)]
	if !ok {
		return item
	}
	if item.SelectEnabled == nil {
		item.SelectEnabled = candidate.SelectEnabled
	}
	if item.DropEnabled == nil {
		item.DropEnabled = candidate.DropEnabled
	}
	if len(item.SelectRounds) == 0 {
		item.SelectRounds = append([]int(nil), candidate.SelectRounds...)
	}
	if item.Capacity == nil {
		item.Capacity = candidate.Capacity
	}
	if item.Enrolled == nil {
		item.Enrolled = candidate.Enrolled
	}
	if item.Selected == nil {
		item.Selected = candidate.Selected
	}
	return item
}

func containsSelectionRound(rounds []int, target int) bool {
	for _, round := range rounds {
		if round == target {
			return true
		}
	}
	return false
}

func formatSelectionRounds(rounds []int) string {
	values := make([]string, 0, len(rounds))
	for _, round := range rounds {
		values = append(values, strconv.Itoa(round))
	}
	return strings.Join(values, ", ")
}

func strictTargetIssues(target []Course, allCourses []Course, planTerm string) []ValidationIssue {
	var issues []ValidationIssue
	byCode := make(map[string]int)
	byBase := make(map[string]int)
	library := mapByCode(allCourses)

	for _, item := range target {
		code := normalizeCode(item.DisplayCode)
		base := baseCourseCode(item.DisplayCode, item.GroupID, item.RawCourseCode, item.SectionName)
		if code != "" {
			byCode[code]++
			if len(allCourses) > 0 {
				if _, ok := library[code]; !ok {
					issues = append(issues, ValidationIssue{Level: "error", Code: code, Message: "Target teaching class is not present in the current course.json."})
				}
			}
			if term := termFromCode(code); planTerm != "" && term != "" && term != planTerm {
				issues = append(issues, ValidationIssue{Level: "error", Code: code, Message: "Target teaching class term does not match the execution plan term."})
			}
		}
		if base != "" {
			byBase[base]++
		}
		if strings.TrimSpace(item.TimeText) != "" {
			_, hasTime, parseWarnings := parseCourseSlots(item.TimeText)
			if !hasTime || len(parseWarnings) > 0 {
				message := "无法解析课程时间，已阻断执行"
				if len(parseWarnings) > 0 {
					message += ": " + strings.Join(parseWarnings, "；")
				}
				issues = append(issues, ValidationIssue{Level: "error", Code: code, Message: message})
			}
		}
	}
	for code, count := range byCode {
		if count > 1 {
			issues = append(issues, ValidationIssue{Level: "error", Code: code, Message: "The target schedule contains the same teaching class more than once."})
		}
	}
	for base, count := range byBase {
		if count > 1 {
			issues = append(issues, ValidationIssue{Level: "error", Code: base, Message: "The target schedule contains multiple teaching classes for the same base course."})
		}
	}
	if planTerm == "" {
		issues = append(issues, ValidationIssue{Level: "error", Message: "The target schedule has no recognizable academic term."})
	}

	for index, left := range target {
		leftSlots, leftHasTime, _ := parseCourseSlots(left.TimeText)
		if !leftHasTime {
			continue
		}
		for _, right := range target[index+1:] {
			if normalizeCode(left.DisplayCode) == normalizeCode(right.DisplayCode) {
				continue
			}
			rightSlots, rightHasTime, _ := parseCourseSlots(right.TimeText)
			if rightHasTime && slotsConflict(leftSlots, rightSlots) {
				issues = append(issues, ValidationIssue{
					Level:   "error",
					Code:    left.DisplayCode + " / " + right.DisplayCode,
					Message: "Target schedule contains a time conflict between teaching classes.",
				})
			}
		}
	}
	return issues
}

func liveSchedulePlanIssues(plan ActionPlan, p paths) []ValidationIssue {
	if len(plan.Drop) == 0 {
		return nil
	}
	live, _, err := loadCourses(p.livePersonalPath)
	if err != nil {
		return []ValidationIssue{{Level: "error", Message: "Drop plan requires an imported personal-schedule-live.json snapshot."}}
	}
	if plan.CurrentSource != "live" {
		return []ValidationIssue{{Level: "error", Message: "Drop plan was not generated from the live personal schedule snapshot."}}
	}
	if plan.CurrentHash == "" || plan.CurrentHash != scheduleHash(live) {
		return []ValidationIssue{{Level: "error", Message: "The live personal schedule changed after this drop plan was generated."}}
	}
	return nil
}

func blockingValidationIssues(issues []ValidationIssue) []ValidationIssue {
	var blockers []ValidationIssue
	for _, issue := range issues {
		if issue.Level == "error" {
			blockers = append(blockers, issue)
		}
	}
	return sortValidationIssues(dedupeValidationIssues(blockers))
}

func validatePlanExecutionEligibility(plan ActionPlan) error {
	blockers := blockingValidationIssues(plan.Validation)
	if len(blockers) == 0 {
		return nil
	}
	return fmt.Errorf("execution config blocked by %d plan validation error(s)", len(blockers))
}

func duplicateIssues(label string, items []Course) []ValidationIssue {
	var issues []ValidationIssue
	codeCount := make(map[string]int)
	baseCount := make(map[string]int)
	for _, item := range items {
		code := normalizeCode(item.DisplayCode)
		if code != "" {
			codeCount[code]++
		}
		base := baseCourseCode(item.DisplayCode, item.GroupID, item.RawCourseCode, item.SectionName)
		if base != "" {
			baseCount[base]++
		}
		if item.CourseName == "" || item.CourseName == "未命名课程" {
			issues = append(issues, ValidationIssue{Level: "warning", Code: code, Message: label + "中有课程缺少课程名称。"})
		}
		if item.TimeText == "" {
			issues = append(issues, ValidationIssue{Level: "warning", Code: code, Message: label + "中有课程缺少上课时间。"})
		}
		if item.Teacher == "" || item.Teacher == "未填写教师" {
			issues = append(issues, ValidationIssue{Level: "info", Code: code, Message: label + "中有课程未填写教师。"})
		}
	}
	for code, count := range codeCount {
		if count > 1 {
			issues = append(issues, ValidationIssue{Level: "warning", Code: code, Message: fmt.Sprintf("%s中教学班 %s 出现 %d 次。", label, code, count)})
		}
	}
	for base, count := range baseCount {
		if count > 1 {
			issues = append(issues, ValidationIssue{Level: "warning", Code: base, Message: fmt.Sprintf("%s中同一课程号 %s 出现 %d 个教学班，请确认是否误选。", label, base, count)})
		}
	}
	return issues
}

func fallbackExplanations(selectItems []Course, groups []FallbackGroup) []PlanExplanation {
	groupByPreferred := make(map[string]FallbackGroup)
	for _, group := range groups {
		groupByPreferred[group.Preferred] = group
	}
	var explanations []PlanExplanation
	for _, item := range selectItems {
		if group, ok := groupByPreferred[item.DisplayCode]; ok {
			explanations = append(explanations, PlanExplanation{
				Category: "fallback",
				Code:     item.DisplayCode,
				Message:  fmt.Sprintf("%s 有 %d 个同课程号备选教学班。", courseTitle(item), len(group.Alternatives)),
			})
		} else {
			explanations = append(explanations, PlanExplanation{
				Category: "fallback",
				Code:     item.DisplayCode,
				Message:  fmt.Sprintf("%s 暂未找到同课程号备选教学班。", courseTitle(item)),
			})
		}
	}
	return explanations
}

func buildFallbackGroups(selectItems []Course, allCourses []Course) []FallbackGroup {
	byBase := make(map[string][]Course)
	for _, course := range allCourses {
		base := baseCourseCode(course.DisplayCode, course.GroupID, course.RawCourseCode, course.SectionName)
		if base == "" {
			continue
		}
		byBase[base] = append(byBase[base], course)
	}

	var groups []FallbackGroup
	for _, item := range selectItems {
		base := baseCourseCode(item.DisplayCode, item.GroupID, item.RawCourseCode, item.SectionName)
		if base == "" {
			continue
		}
		var alternatives []Course
		for _, candidate := range byBase[base] {
			if normalizeCode(candidate.DisplayCode) == normalizeCode(item.DisplayCode) {
				continue
			}
			alternatives = append(alternatives, candidate)
		}
		if len(alternatives) == 0 {
			continue
		}
		groups = append(groups, FallbackGroup{
			CourseBase:   base,
			CourseName:   item.CourseName,
			Preferred:    item.DisplayCode,
			Alternatives: sortCourses(alternatives),
		})
	}
	sort.Slice(groups, func(i, j int) bool {
		return groups[i].Preferred < groups[j].Preferred
	})
	return groups
}

func buildKillCourseConfig(plan ActionPlan, configPath string) (KillCourseConfig, ConfigPreview, error) {
	cfg := defaultKillCourseConfig(plan.Term)
	if err := validatePlanExecutionEligibility(plan); err != nil {
		return cfg, ConfigPreview{}, err
	}
	existingFound := false
	if data, err := os.ReadFile(configPath); err == nil {
		_ = json.Unmarshal(data, &cfg)
		existingFound = true
	}
	oldActions := copyActionMap(cfg.Course)
	cfg.Course = make(map[string]string)
	for _, item := range plan.Select {
		if item.DisplayCode != "" {
			cfg.Course[item.DisplayCode] = "1"
		}
	}
	for _, item := range plan.Drop {
		if item.DisplayCode != "" {
			cfg.Course[item.DisplayCode] = "0"
		}
	}
	if len(cfg.Course) == 0 {
		return cfg, ConfigPreview{}, errors.New("执行计划没有选课或退课动作，未生成 KillCourse 配置")
	}
	xn, xq := splitTerm(plan.Term)
	if xn != "" {
		cfg.Time.XueNian = xn
	}
	if xq != "" {
		cfg.Time.XueQi = xq
	}
	preview := buildConfigPreview(configPath, existingFound, cfg, oldActions)
	return cfg, preview, nil
}

func buildConfigPreview(configPath string, existingFound bool, cfg KillCourseConfig, oldActions map[string]string) ConfigPreview {
	return ConfigPreview{
		Path:                configPath,
		ExistingConfigFound: existingFound,
		HasCASLogin:         strings.TrimSpace(cfg.CasLogin.Username) != "" && strings.TrimSpace(cfg.CasLogin.Password) != "",
		HasNewJWLogin:       strings.TrimSpace(cfg.NewJWLogin.Username) != "" && strings.TrimSpace(cfg.NewJWLogin.Password) != "",
		CookiesEnabled:      cfg.Cookies.Enabled,
		WaitCourseEnabled:   cfg.WaitCourse.Enabled,
		SMTPEnabled:         cfg.SMTPEmail.Enabled,
		StartTime:           cfg.StartTime,
		XueNian:             cfg.Time.XueNian,
		XueQi:               cfg.Time.XueQi,
		OldActionCount:      len(oldActions),
		NewActionCount:      len(cfg.Course),
		Actions:             buildConfigActionPreview(oldActions, cfg.Course),
	}
}

func buildConfigActionPreview(oldActions map[string]string, newActions map[string]string) []ConfigActionPreview {
	codes := make(map[string]bool)
	for code := range oldActions {
		codes[code] = true
	}
	for code := range newActions {
		codes[code] = true
	}
	var list []ConfigActionPreview
	for code := range codes {
		oldAction := oldActions[code]
		newAction := newActions[code]
		status := "unchanged"
		switch {
		case oldAction == "" && newAction != "":
			status = "added"
		case oldAction != "" && newAction == "":
			status = "removed"
		case oldAction != newAction:
			status = "changed"
		}
		list = append(list, ConfigActionPreview{
			Code:      code,
			OldAction: oldAction,
			NewAction: newAction,
			Status:    status,
		})
	}
	sort.Slice(list, func(i, j int) bool {
		return list[i].Code < list[j].Code
	})
	return list
}

func liveScheduleReadinessChecks(plan ActionPlan, p paths) []ReadinessCheck {
	hasDrop := len(plan.Drop) > 0
	live, _, err := loadCourses(p.livePersonalPath)
	liveHash := ""
	if err == nil {
		liveHash = scheduleHash(live)
	}
	switch {
	case !hasDrop && err != nil:
		return []ReadinessCheck{{
			Level:   "warning",
			Passed:  false,
			Message: "No live current schedule snapshot is available. Select-only plans may continue, but execution should refresh the academic-system schedule first.",
		}}
	case !hasDrop:
		return []ReadinessCheck{{
			Level:   "info",
			Passed:  true,
			Message: "Live current schedule snapshot is available. This select-only plan may proceed after manual confirmation.",
		}}
	case err != nil:
		return []ReadinessCheck{{
			Level:   "error",
			Passed:  false,
			Message: "Drop plan blocked: import or refresh personal-schedule-live.json before authorization.",
		}}
	case plan.CurrentSource != "live":
		return []ReadinessCheck{{
			Level:   "error",
			Passed:  false,
			Message: "Drop plan blocked: this plan was not generated from personal-schedule-live.json. Regenerate the plan after live sync.",
		}}
	case plan.CurrentHash == "" || plan.CurrentHash != liveHash:
		return []ReadinessCheck{{
			Level:   "error",
			Passed:  false,
			Message: "Drop plan blocked: the live current schedule changed after this plan was generated. Regenerate the plan.",
		}}
	default:
		return []ReadinessCheck{{
			Level:   "info",
			Passed:  true,
			Message: "Drop plan is based on the current personal-schedule-live.json snapshot.",
		}}
	}
}

func buildExecutionReadiness(plan ActionPlan, cfg KillCourseConfig, preview ConfigPreview, p paths) ExecutionReadiness {
	var checks []ReadinessCheck
	add := func(level string, passed bool, message string) {
		checks = append(checks, ReadinessCheck{Level: level, Passed: passed, Message: message})
	}

	add("error", len(cfg.Course) > 0, "KillCourse config 至少需要 1 条 course 动作。")
	add("error", dirExists(p.killCourseDir), "已检测到 HDU-KillCourse 目录。")

	hasLogin := preview.HasCASLogin || preview.HasNewJWLogin || hasUsableCookie(cfg)
	add("warning", hasLogin, "已保留账号密码或可用 Cookie 登录信息。")

	xn, xq := cfg.Time.XueNian, cfg.Time.XueQi
	add("error", regexp.MustCompile(`^\d{4}$`).MatchString(xn) && (xq == "1" || xq == "2"), "学年学期格式满足 KillCourse 要求。")
	add("error", actionsMatchTerm(cfg.Course, xn, xq), "所有 course 动作的教学班号与配置学年学期一致。")
	add("error", actionsAreValid(cfg.Course), "所有 course 动作均为 1(选课) 或 0(退课)。")

	startOK := validStartTime(cfg.StartTime)
	add("error", startOK, "start_time 可被解析为本地时间。")

	if cfg.WaitCourse.Enabled == "1" {
		add("error", cfg.WaitCourse.Interval > 0, "蹲课已开启，interval 必须大于 0。")
	} else {
		add("info", true, "蹲课未开启，本计划只生成普通选退课配置。")
	}

	if len(plan.Drop) > 0 {
		add("warning", false, fmt.Sprintf("计划包含 %d 门退课动作，真实执行前需要再次确认风险。", len(plan.Drop)))
	} else {
		add("info", true, "计划不包含退课动作。")
	}

	for _, check := range liveScheduleReadinessChecks(plan, p) {
		add(check.Level, check.Passed, check.Message)
	}

	for _, issue := range plan.Validation {
		if issue.Level == "warning" || issue.Level == "error" {
			add(issue.Level, false, "计划校验提示："+issue.Message)
		}
	}

	ready := true
	canExecute := true
	for _, check := range checks {
		if check.Level == "error" && !check.Passed {
			ready = false
			canExecute = false
		}
	}
	summary := "执行准备检查通过，但真实执行前仍需人工确认。"
	if !ready {
		summary = "执行准备检查未通过，请先修复错误项。"
	} else if hasFailedWarning(checks) {
		summary = "执行准备检查有警告，建议确认风险后再交给 KillCourse。"
	}
	return ExecutionReadiness{
		Ready:      ready,
		Summary:    summary,
		Checks:     checks,
		CanExecute: canExecute,
	}
}

func buildExecutionDryRun(req DryRunRequest, p paths) (ExecutionDryRun, error) {
	if err := validatePlanExecutionEligibility(req.Plan); err != nil {
		return ExecutionDryRun{}, err
	}
	if len(req.Plan.Select) == 0 && len(req.Plan.Drop) == 0 {
		return ExecutionDryRun{}, errors.New("当前执行计划没有选课或退课动作，无法生成 dry-run")
	}

	var cfg KillCourseConfig
	var preview ConfigPreview
	var err error
	if req.GeneratedConfig != nil && len(req.GeneratedConfig.Course) > 0 {
		cfg = *req.GeneratedConfig
		preview = buildConfigPreview(p.killConfigPath, fileExists(p.killConfigPath), cfg, readExistingCourseActions(p.killConfigPath))
	} else {
		cfg, preview, err = buildKillCourseConfig(req.Plan, p.killConfigPath)
		if err != nil {
			return ExecutionDryRun{}, err
		}
	}

	readiness := buildExecutionReadiness(req.Plan, cfg, preview, p)
	command, entryFound := killCourseCommand(p)
	entryPath := killCourseEntryPath(p)
	logPath := executionLogFilePath(p)
	actionCounts := countExecutionActions(cfg.Course)
	configExists, configMatches := configFileMatchesGenerated(p.killConfigPath, cfg)
	events := []ExecutionEvent{
		{Level: "info", Message: "已完成 dry-run：没有真实执行选课、退课或蹲课。"},
		{Level: "info", Message: "工作目录：" + p.killCourseDir},
		{Level: "info", Message: "配置文件：" + p.killConfigPath},
		{Level: "info", Message: "启动入口：" + firstNonEmpty(entryPath, command)},
		{Level: "info", Message: "日志文件：" + logPath},
		{Level: "info", Message: fmt.Sprintf("将写入/使用 %d 条选课动作、%d 条退课动作。", actionCounts.Select, actionCounts.Drop)},
	}
	switch {
	case !configExists:
		events = append(events, ExecutionEvent{Level: "error", Message: "KillCourse/config.json 尚未写入磁盘，真实执行前请先勾选写入配置并重新生成计划。"})
	case !configMatches:
		events = append(events, ExecutionEvent{Level: "error", Message: "磁盘上的 KillCourse/config.json 与当前生成的执行计划不一致，真实执行前请重新写入配置。"})
	default:
		events = append(events, ExecutionEvent{Level: "info", Message: "磁盘上的 KillCourse/config.json 与当前生成计划一致。"})
	}
	if entryFound {
		events = append(events, ExecutionEvent{Level: "info", Message: "检测到 KillCourse 启动入口：" + command})
	} else {
		events = append(events, ExecutionEvent{Level: "error", Message: "未找到 KillCourse 可执行文件或 Go 入口。"})
	}
	if actionCounts.Drop > 0 {
		events = append(events, ExecutionEvent{Level: "warning", Message: "计划包含退课动作；真实执行前必须人工确认退课风险。"})
	}
	for _, check := range readiness.Checks {
		if !check.Passed {
			events = append(events, ExecutionEvent{Level: check.Level, Message: check.Message})
		}
	}

	canExecute := readiness.CanExecute && entryFound && configExists && configMatches
	summary := "Dry-run 通过：可以把生成的 config.json 交给 KillCourse，但真实执行前仍需人工确认。"
	if !entryFound || !readiness.Ready || !configExists || !configMatches {
		summary = "Dry-run 未通过：请先修复错误项，再考虑真实执行。"
	} else if hasFailedWarning(readiness.Checks) || actionCounts.Drop > 0 {
		summary = "Dry-run 有警告：配置基本可用，但存在需要人工确认的风险。"
	}

	return ExecutionDryRun{
		Ready:              readiness.Ready && entryFound && configExists && configMatches,
		CanExecute:         canExecute,
		Summary:            summary,
		WorkspaceDir:       p.workspace,
		KillCourseDir:      p.killCourseDir,
		ConfigPath:         p.killConfigPath,
		EntryPath:          entryPath,
		LogPath:            logPath,
		Command:            command,
		EntryFound:         entryFound,
		ActionCounts:       actionCounts,
		HasDropRisk:        actionCounts.Drop > 0,
		ConfirmationPhrase: expectedConfirmationPhrase(actionCounts.Drop > 0),
		Readiness:          readiness,
		Events:             events,
		GeneratedAt:        time.Now().Format(time.RFC3339),
	}, nil
}

func buildExecutionAuthorization(req ExecutionAuthorizationRequest, p paths) (ExecutionAuthorization, error) {
	dryRun, err := buildExecutionDryRun(DryRunRequest{
		Plan:            req.Plan,
		GeneratedConfig: req.GeneratedConfig,
	}, p)
	if err != nil {
		return ExecutionAuthorization{}, err
	}
	if !dryRun.CanExecute {
		return ExecutionAuthorization{}, errors.New("dry-run 未通过，不能生成执行授权")
	}
	expected := dryRun.ConfirmationPhrase
	if strings.TrimSpace(req.ConfirmationPhrase) != expected {
		return ExecutionAuthorization{}, fmt.Errorf("确认短语不匹配，请输入：%s", expected)
	}

	var cfg KillCourseConfig
	if req.GeneratedConfig != nil {
		cfg = *req.GeneratedConfig
	} else {
		cfg, _, err = buildKillCourseConfig(req.Plan, p.killConfigPath)
		if err != nil {
			return ExecutionAuthorization{}, err
		}
	}

	now := time.Now()
	planHash := planExecutionHash(req.Plan)
	configHash := configExecutionHash(cfg)
	ticketID := stableHash(map[string]any{
		"plan":      planHash,
		"config":    configHash,
		"createdAt": now.Format(time.RFC3339Nano),
	})
	if len(ticketID) > 16 {
		ticketID = ticketID[:16]
	}
	return ExecutionAuthorization{
		Authorized:         true,
		TicketID:           ticketID,
		PlanHash:           planHash,
		ConfigHash:         configHash,
		CreatedAt:          now.Format(time.RFC3339),
		ExpiresAt:          now.Add(15 * time.Minute).Format(time.RFC3339),
		Command:            dryRun.Command,
		KillCourseDir:      dryRun.KillCourseDir,
		ConfigPath:         dryRun.ConfigPath,
		ActionCounts:       dryRun.ActionCounts,
		HasDropRisk:        dryRun.HasDropRisk,
		DropRiskAccepted:   dryRun.HasDropRisk,
		ConfirmationPhrase: expected,
	}, nil
}

func buildExecutionPackage(req ExecutionPackageRequest, p paths) (ExecutionPackage, error) {
	dryRun, err := buildExecutionDryRun(DryRunRequest{
		Plan:            req.Plan,
		GeneratedConfig: req.GeneratedConfig,
	}, p)
	if err != nil {
		return ExecutionPackage{}, err
	}
	if !dryRun.CanExecute {
		return ExecutionPackage{}, errors.New("dry-run 未通过，不能生成执行启动包")
	}
	if err := validateExecutionAuthorization(req.Authorization, req.Plan, req.GeneratedConfig, dryRun, p.approvalPath); err != nil {
		return ExecutionPackage{}, err
	}
	warnings := []string{
		"启动包不会自动运行；需要用户手动执行 run-killcourse.bat。",
		"KillCourse 启动后仍会等待一次 Enter，请在确认配置无误后手动按 Enter。",
	}
	if dryRun.HasDropRisk {
		warnings = append(warnings, "本计划包含退课动作，真实执行前请再次确认退课风险。")
	}
	logStartOffset := currentLogOffset(dryRun.LogPath)
	return ExecutionPackage{
		Ready:          true,
		Summary:        "执行启动包已生成。请手动运行 bat，并在 KillCourse 提示时自行按 Enter。",
		BatchPath:      p.runBatchPath,
		RunbookPath:    p.runbookPath,
		ManifestPath:   p.manifestPath,
		Command:        dryRun.Command,
		KillCourseDir:  dryRun.KillCourseDir,
		ConfigPath:     dryRun.ConfigPath,
		EntryPath:      dryRun.EntryPath,
		LogPath:        dryRun.LogPath,
		LogStartOffset: logStartOffset,
		TicketID:       req.Authorization.TicketID,
		ActionCounts:   dryRun.ActionCounts,
		Warnings:       warnings,
		GeneratedAt:    time.Now().Format(time.RFC3339),
	}, nil
}

func validateExecutionAuthorization(auth ExecutionAuthorization, plan ActionPlan, cfg *KillCourseConfig, dryRun ExecutionDryRun, approvalPath string) error {
	if !auth.Authorized {
		return errors.New("缺少有效执行授权票据")
	}
	if strings.TrimSpace(auth.TicketID) == "" {
		return errors.New("执行授权票据缺少票据编号")
	}
	expiresAt, err := time.Parse(time.RFC3339, auth.ExpiresAt)
	if err != nil {
		return errors.New("执行授权票据过期时间无效")
	}
	if time.Now().After(expiresAt) {
		return errors.New("执行授权票据已过期，请重新 dry-run 并确认")
	}
	if auth.Command != dryRun.Command || auth.KillCourseDir != dryRun.KillCourseDir || auth.ConfigPath != dryRun.ConfigPath {
		return errors.New("执行授权票据路径与当前 dry-run 不一致")
	}
	if auth.ActionCounts != dryRun.ActionCounts {
		return errors.New("执行授权票据动作数量与当前 dry-run 不一致")
	}
	if auth.HasDropRisk != dryRun.HasDropRisk {
		return errors.New("执行授权票据退课风险状态与当前 dry-run 不一致")
	}
	if auth.DropRiskAccepted != dryRun.HasDropRisk {
		return errors.New("执行授权票据未确认当前退课风险")
	}
	if auth.ConfirmationPhrase != expectedConfirmationPhrase(dryRun.HasDropRisk) {
		return errors.New("执行授权票据确认短语无效")
	}
	if auth.PlanHash != planExecutionHash(plan) {
		return errors.New("执行授权票据与当前计划不一致")
	}
	if cfg == nil {
		return errors.New("缺少当前生成的 KillCourse 配置")
	}
	if auth.ConfigHash != configExecutionHash(*cfg) {
		return errors.New("执行授权票据与当前 KillCourse 配置不一致")
	}
	if strings.TrimSpace(approvalPath) == "" {
		return errors.New("缺少落盘执行授权票据")
	}
	persistedData, err := os.ReadFile(approvalPath)
	if err != nil {
		return errors.New("未找到落盘执行授权票据，请重新确认")
	}
	var persisted ExecutionAuthorization
	if err := json.Unmarshal(persistedData, &persisted); err != nil {
		return errors.New("落盘执行授权票据无效，请重新确认")
	}
	if !reflect.DeepEqual(persisted, auth) {
		return errors.New("请求中的授权票据与落盘票据不一致，请重新确认")
	}
	return nil
}

func writeExecutionPackageFiles(pkg ExecutionPackage, auth ExecutionAuthorization, p paths) error {
	bat := renderExecutionBatch(pkg)
	runbook := renderExecutionRunbook(pkg, auth)
	manifest := map[string]any{
		"schemaVersion":  schemaVersion,
		"generatedAt":    pkg.GeneratedAt,
		"package":        pkg,
		"authorization":  auth,
		"manualRunOnly":  true,
		"requiresEnter":  true,
		"preStartPause":  true,
		"safetyBoundary": "本启动包不会由 Smart Agent 自动运行；用户必须手动运行 bat，并在 KillCourse 窗口自行按 Enter。",
	}
	if err := os.WriteFile(p.runBatchPath, []byte(bat), 0644); err != nil {
		return err
	}
	if err := os.WriteFile(p.runbookPath, []byte(runbook), 0644); err != nil {
		return err
	}
	return writeJSONFile(p.manifestPath, manifest)
}

func renderExecutionBatch(pkg ExecutionPackage) string {
	return strings.Join([]string{
		"@echo off",
		"chcp 65001 >nul",
		"echo HDU Smart Course Agent - KillCourse 启动包",
		"echo.",
		"echo 票据: " + pkg.TicketID,
		fmt.Sprintf("echo 动作: 选课 %d 条, 退课 %d 条", pkg.ActionCounts.Select, pkg.ActionCounts.Drop),
		"echo 配置: " + pkg.ConfigPath,
		"echo.",
		"echo 注意: KillCourse 启动后会等待一次 Enter。",
		"echo 请确认配置页面无误后，再在该窗口按 Enter 继续。",
		"echo.",
		"echo 即将启动 KillCourse。若不是你本人主动运行，请直接关闭本窗口。",
		"pause",
		"echo.",
		"cd /d " + quoteWindowsArg(pkg.KillCourseDir),
		batchCommand(pkg.Command),
		"echo.",
		"echo KillCourse 已退出。",
		"pause",
		"",
	}, "\r\n")
}

func renderExecutionRunbook(pkg ExecutionPackage, auth ExecutionAuthorization) string {
	lines := []string{
		"# HDU-Smart-Course-Agent 执行启动包",
		"",
		"本文件由 HDU 智能选课执行助手生成。它不会自动选课，真正执行发生在你手动运行 `run-killcourse.bat` 并在 KillCourse 提示后按 Enter 之后。",
		"",
		"## 启动信息",
		"",
		"- 票据：" + pkg.TicketID,
		"- 工作目录：" + pkg.KillCourseDir,
		"- 配置文件：" + pkg.ConfigPath,
		"- 启动入口：" + firstNonEmpty(pkg.EntryPath, pkg.Command),
		"- 日志文件：" + pkg.LogPath,
		fmt.Sprintf("- 日志起始偏移：%d 字节", pkg.LogStartOffset),
		"- 命令：" + pkg.Command,
		"- 启动脚本：" + pkg.BatchPath,
		"- 启动清单：" + pkg.ManifestPath,
		fmt.Sprintf("- 动作数量：选课 %d 条，退课 %d 条", pkg.ActionCounts.Select, pkg.ActionCounts.Drop),
		"- 授权有效期：" + auth.ExpiresAt,
		"",
		"## 操作步骤",
		"",
		"1. 确认 `HDU-KillCourse/config.json` 已经是本次计划生成的配置。",
		"2. 双击或在终端运行 `run-killcourse.bat`。",
		"3. bat 会先暂停一次，确认是你本人主动启动后再按任意键。",
		"4. KillCourse 会打开配置编辑服务并提示按 Enter 继续。",
		"5. 最后确认课程动作和登录信息后，再手动按 Enter。",
		"6. 若包含退课动作，请确认退课风险后再继续。",
	}
	if pkg.ActionCounts.Drop > 0 {
		lines = append(lines, "", "## 退课风险", "", "本计划包含退课动作。若新教学班未成功选上，原课程可能已经被退掉。")
	}
	lines = append(lines, "")
	return strings.Join(lines, "\n")
}

func quoteWindowsArg(value string) string {
	return `"` + strings.ReplaceAll(value, `"`, `\"`) + `"`
}

func batchCommand(command string) string {
	if filepath.IsAbs(command) {
		return quoteWindowsArg(command)
	}
	return command
}

func executionLogFilePath(p paths) string {
	return filepath.Join(p.killCourseDir, "log_files", "app.log")
}

func currentLogOffset(logPath string) int64 {
	info, err := os.Stat(logPath)
	if err != nil || !info.Mode().IsRegular() {
		return 0
	}
	return info.Size()
}

func buildExecutionLog(req ExecutionLogRequest, p paths) (ExecutionLog, error) {
	logPath := executionLogFilePath(p)
	file, err := os.Open(logPath)
	if err != nil {
		return ExecutionLog{}, err
	}
	defer file.Close()
	if offset, ok := readExecutionLogOffset(p.manifestPath); ok {
		if offset <= currentLogOffset(logPath) {
			if _, err := file.Seek(offset, io.SeekStart); err != nil {
				return ExecutionLog{}, err
			}
		}
	}

	expectedActions := expectedActionMap(req.Plan, req.GeneratedConfig)
	result := ExecutionLog{
		SchemaVersion: schemaVersion,
		Source:        "hdu-killcourse-app-log",
		GeneratedAt:   time.Now().Format(time.RFC3339),
		LogPath:       logPath,
		PlanHash:      planExecutionHash(req.Plan),
	}
	if req.GeneratedConfig != nil {
		result.ConfigHash = configExecutionHash(*req.GeneratedConfig)
	}

	var active *ExecutionLogItem
	flushActive := func() {
		if active == nil {
			return
		}
		if active.Status == "" {
			active.Status = "running"
		}
		result.Items = append(result.Items, *active)
		active = nil
	}

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		raw := scanner.Text()
		parsedAt, level, message := parseKillCourseLogLine(raw)
		_ = level
		if code, ok := parseProcessingCourse(message); ok {
			flushActive()
			action := expectedActions[code]
			if action == "" {
				action = "unknown"
			}
			active = &ExecutionLogItem{
				CourseCode: code,
				Action:     action,
				Status:     "running",
				RawLines:   []string{raw},
				StartedAt:  parsedAt,
			}
			continue
		}
		if active == nil {
			continue
		}
		active.RawLines = append(active.RawLines, raw)
		if value, ok := parseLogValue(message, "课程名称:"); ok {
			active.CourseName = value
			continue
		}
		if value, ok := parseLogValue(message, "上课时间:"); ok {
			active.TimeText = value
			continue
		}
		if status, failureType, terminalMessage, terminal := classifyExecutionMessage(message); terminal {
			active.Status = status
			active.FailureType = failureType
			active.Message = terminalMessage
			active.FinishedAt = parsedAt
			flushActive()
		}
	}
	if err := scanner.Err(); err != nil {
		return ExecutionLog{}, err
	}
	flushActive()
	result.Summary = summarizeExecutionLog(result.Items)
	return result, nil
}

var killCourseLogRe = regexp.MustCompile(`^(\d{4})/(\d{2})/(\d{2})\s+(\d{2}:\d{2}:\d{2})(?:\.\d+)?\s+\[(INFO|ERROR)\]\s*(.*)$`)

func parseKillCourseLogLine(raw string) (string, string, string) {
	match := killCourseLogRe.FindStringSubmatch(raw)
	if len(match) != 7 {
		return "", "", strings.TrimSpace(raw)
	}
	when := fmt.Sprintf("%s-%s-%s %s", match[1], match[2], match[3], match[4])
	parsed, err := time.ParseInLocation("2006-01-02 15:04:05", when, time.Local)
	if err != nil {
		return "", match[5], strings.TrimSpace(match[6])
	}
	return parsed.Format(time.RFC3339), match[5], strings.TrimSpace(match[6])
}

func parseProcessingCourse(message string) (string, bool) {
	const prefix = "正在处理课程:"
	if !strings.Contains(message, prefix) {
		return "", false
	}
	value := strings.TrimSpace(strings.SplitN(message, prefix, 2)[1])
	code := normalizeCode(value)
	if code == "" {
		code = value
	}
	return code, code != ""
}

func parseLogValue(message string, prefix string) (string, bool) {
	if !strings.Contains(message, prefix) {
		return "", false
	}
	return strings.TrimSpace(strings.SplitN(message, prefix, 2)[1]), true
}

func classifyExecutionMessage(message string) (string, string, string, bool) {
	switch {
	case strings.Contains(message, "选课成功"):
		return "success", "", message, true
	case strings.Contains(message, "退课成功"):
		return "success", "", message, true
	case strings.Contains(message, "蹲选课成功"):
		return "success", "", message, true
	case strings.Contains(message, "选课失败"):
		return "failed", classifyFailure(message), message, true
	case strings.Contains(message, "退课失败"):
		return "failed", classifyFailure(message), message, true
	case strings.Contains(message, "处理课程失败"):
		return "failed", classifyFailure(message), message, true
	case strings.Contains(message, "登录过期"):
		return "failed", "login-expired", message, true
	default:
		return "", "", "", false
	}
}

func classifyFailure(message string) string {
	switch {
	case strings.Contains(message, "人数可能已满") || strings.Contains(message, "无余量") || strings.Contains(message, "已满"):
		return "full"
	case strings.Contains(message, "登录过期"):
		return "login-expired"
	case strings.Contains(message, "不存在") || strings.Contains(message, "未查询到该课程"):
		return "not-found"
	case strings.Contains(message, "冲突"):
		return "conflict"
	case strings.Contains(message, "时间格式错误") || strings.Contains(message, "学期格式错误") || strings.Contains(message, "配置"):
		return "config"
	case strings.Contains(strings.ToLower(message), "timeout") || strings.Contains(message, "网络") || strings.Contains(message, "请求"):
		return "network"
	default:
		return "unknown"
	}
}

func expectedActionMap(plan ActionPlan, cfg *KillCourseConfig) map[string]string {
	result := make(map[string]string)
	if cfg != nil {
		for code, action := range cfg.Course {
			switch action {
			case "1":
				result[normalizeCodeOrSelf(code)] = "select"
			case "0":
				result[normalizeCodeOrSelf(code)] = "drop"
			}
		}
	}
	for _, item := range plan.Select {
		result[normalizeCodeOrSelf(item.DisplayCode)] = "select"
	}
	for _, item := range plan.Drop {
		result[normalizeCodeOrSelf(item.DisplayCode)] = "drop"
	}
	return result
}

func normalizeCodeOrSelf(value string) string {
	if code := normalizeCode(value); code != "" {
		return code
	}
	return strings.TrimSpace(value)
}

func summarizeExecutionLog(items []ExecutionLogItem) ExecutionLogSummary {
	summary := ExecutionLogSummary{Total: len(items)}
	for _, item := range items {
		switch item.Status {
		case "success":
			summary.Success++
		case "failed":
			summary.Failed++
		case "skipped":
			summary.Skipped++
		case "pending", "running", "":
			summary.Pending++
		}
	}
	return summary
}

func executionLogNeedsRefresh(log ExecutionLog) bool {
	for _, item := range log.Items {
		if item.Status == "success" && (item.Action == "select" || item.Action == "drop") {
			return true
		}
	}
	return false
}

func buildFallbackRecommendations(req FallbackRecommendationRequest, p paths) (FallbackRecommendations, error) {
	allCourses, _, err := loadCourses(p.coursePath)
	if err != nil {
		return FallbackRecommendations{}, fmt.Errorf("读取 course.json 失败，无法生成备选推荐: %w", err)
	}
	byCode := mapByCode(allCourses)
	byBase := make(map[string][]Course)
	for _, course := range allCourses {
		base := baseCourseCode(course.DisplayCode, course.GroupID, course.RawCourseCode, course.SectionName)
		if base != "" {
			byBase[base] = append(byBase[base], course)
		}
	}
	targetCodes := make(map[string]bool)
	for _, course := range req.Plan.Target {
		if code := normalizeCode(course.DisplayCode); code != "" {
			targetCodes[code] = true
		}
	}

	result := FallbackRecommendations{
		SchemaVersion: schemaVersion,
		GeneratedAt:   time.Now().Format(time.RFC3339),
		PlanHash:      planExecutionHash(req.Plan),
		ExecutionHash: stableHash(req.ExecutionLog),
	}
	for _, failed := range req.ExecutionLog.Items {
		if failed.Action != "select" || failed.Status != "failed" {
			continue
		}
		result.Summary.FailedSelectCount++
		failedCourse := courseFromExecutionItem(failed, req.Plan, byCode)
		base := baseCourseCode(failedCourse.DisplayCode, failedCourse.GroupID, failedCourse.RawCourseCode, failedCourse.SectionName, failed.CourseCode)
		item := FallbackRecommendationItem{
			FailedCourse:   firstNonEmpty(normalizeCode(failed.CourseCode), failed.CourseCode, failedCourse.DisplayCode),
			CourseName:     firstNonEmpty(failedCourse.CourseName, failed.CourseName),
			FailureType:    firstNonEmpty(failed.FailureType, "unknown"),
			Message:        failed.Message,
			Recommendation: "暂未找到可替代教学班，建议回到排课助手添加约束后重新生成目标课表。",
		}
		if !fallbackFailureEligible(failed.FailureType) {
			item.Recommendation = "该失败类型更适合先检查登录、配置或网络状态，暂不自动推荐替代教学班。"
			result.Summary.WithoutOptions++
			result.Items = append(result.Items, item)
			continue
		}
		if base == "" {
			item.Recommendation = "无法识别课程号主干，不能可靠寻找同课程号备选教学班。"
			result.Summary.WithoutOptions++
			result.Items = append(result.Items, item)
			continue
		}
		baseline := recommendationBaselineCourses(req.Plan, failedCourse, base)
		for _, candidate := range byBase[base] {
			candidateCode := normalizeCode(candidate.DisplayCode)
			if candidateCode == "" || candidateCode == normalizeCode(failed.CourseCode) {
				continue
			}
			if targetCodes[candidateCode] {
				continue
			}
			item.Options = append(item.Options, scoreFallbackOption(candidate, failedCourse, baseline))
		}
		sort.Slice(item.Options, func(i, j int) bool {
			if item.Options[i].Score != item.Options[j].Score {
				return item.Options[i].Score > item.Options[j].Score
			}
			if item.Options[i].TimeCompatible != item.Options[j].TimeCompatible {
				return item.Options[i].TimeCompatible
			}
			if item.Options[i].SameTeacher != item.Options[j].SameTeacher {
				return item.Options[i].SameTeacher
			}
			return item.Options[i].Course.DisplayCode < item.Options[j].Course.DisplayCode
		})
		for index := range item.Options {
			item.Options[index].Rank = index + 1
		}
		if len(item.Options) > 0 {
			item.Recommendation = fmt.Sprintf("找到 %d 个同课程号备选教学班，优先尝试排名靠前且无时间冲突的方案。", len(item.Options))
			result.Summary.WithOptions++
		} else {
			result.Summary.WithoutOptions++
		}
		result.Items = append(result.Items, item)
	}
	return result, nil
}

func fallbackFailureEligible(failureType string) bool {
	switch failureType {
	case "", "full", "not-found", "unknown", "conflict":
		return true
	default:
		return false
	}
}

func courseFromExecutionItem(item ExecutionLogItem, plan ActionPlan, byCode map[string]Course) Course {
	code := normalizeCode(item.CourseCode)
	if code != "" {
		if course, ok := byCode[code]; ok {
			return course
		}
	}
	for _, list := range [][]Course{plan.Target, plan.Select, plan.Keep, plan.Locked, plan.Current} {
		for _, course := range list {
			if normalizeCode(course.DisplayCode) == code {
				return course
			}
		}
	}
	return Course{DisplayCode: firstNonEmpty(code, item.CourseCode), CourseName: item.CourseName, TimeText: item.TimeText}
}

func recommendationBaselineCourses(plan ActionPlan, failed Course, failedBase string) []Course {
	seen := make(map[string]bool)
	add := func(result []Course, course Course) []Course {
		code := normalizeCode(course.DisplayCode)
		if code == "" || seen[code] {
			return result
		}
		if code == normalizeCode(failed.DisplayCode) {
			return result
		}
		if baseCourseCode(course.DisplayCode, course.GroupID, course.RawCourseCode, course.SectionName) == failedBase {
			return result
		}
		seen[code] = true
		return append(result, course)
	}
	var result []Course
	for _, course := range plan.Target {
		result = add(result, course)
	}
	for _, course := range plan.Locked {
		result = add(result, course)
	}
	for _, course := range plan.Keep {
		result = add(result, course)
	}
	return result
}

func scoreFallbackOption(candidate Course, failed Course, baseline []Course) FallbackRecommendationOption {
	option := FallbackRecommendationOption{
		Course:         candidate,
		Score:          100,
		TimeCompatible: true,
		Reasons:        []string{"同课程号教学班"},
	}
	if sameTeacher(candidate.Teacher, failed.Teacher) {
		option.Score += 20
		option.SameTeacher = true
		option.Reasons = append(option.Reasons, "教师与原目标一致")
	}
	candidateSlots, hasTime, warnings := parseCourseSlots(candidate.TimeText)
	option.HasTimeInfo = hasTime
	if hasTime {
		option.Score += 10
	} else {
		option.Score -= 20
		option.Warnings = append(option.Warnings, "备选教学班时间信息不足，无法完整判断冲突")
	}
	option.Warnings = append(option.Warnings, warnings...)
	for _, existing := range baseline {
		existingSlots, existingHasTime, existingWarnings := parseCourseSlots(existing.TimeText)
		if !existingHasTime {
			option.Warnings = append(option.Warnings, fmt.Sprintf("%s 时间信息不足，冲突判断可能不完整", courseTitle(existing)))
			option.Warnings = append(option.Warnings, existingWarnings...)
			continue
		}
		if hasTime && slotsConflict(candidateSlots, existingSlots) {
			option.Conflicts = append(option.Conflicts, existing)
		}
	}
	if len(option.Conflicts) > 0 {
		option.Score -= 120
		option.TimeCompatible = false
		option.Warnings = append(option.Warnings, fmt.Sprintf("与 %d 门目标/锁定课程存在时间冲突", len(option.Conflicts)))
	} else if hasTime {
		option.Score += 50
		option.Reasons = append(option.Reasons, "与当前目标课表无时间冲突")
	}
	option.Warnings = uniqueStrings(option.Warnings)
	option.Reasons = uniqueStrings(option.Reasons)
	return option
}

func sameTeacher(a string, b string) bool {
	left := strings.TrimSpace(a)
	right := strings.TrimSpace(b)
	if left == "" || right == "" {
		return false
	}
	if strings.Contains(left, "未填写") || strings.Contains(right, "未填写") {
		return false
	}
	return left == right
}

type courseSlot struct {
	Day     int
	Periods map[int]bool
	Weeks   map[int]bool
}

var dayRe = regexp.MustCompile(`(?:星期|周)([一二三四五六日天1-7])`)
var periodRe = regexp.MustCompile(`第?\s*([0-9]{1,2}(?:\s*-\s*[0-9]{1,2})?(?:\s*,\s*[0-9]{1,2}(?:\s*-\s*[0-9]{1,2})?)*)\s*节`)
var weekRe = regexp.MustCompile(`([0-9]{1,2})(?:\s*-\s*([0-9]{1,2}))?\s*周`)

func parseCourseSlots(text string) ([]courseSlot, bool, []string) {
	text = strings.TrimSpace(text)
	if text == "" {
		return nil, false, nil
	}
	matches := dayRe.FindAllStringSubmatchIndex(text, -1)
	if len(matches) == 0 {
		return nil, false, []string{"未识别到星期信息"}
	}
	var slots []courseSlot
	var warnings []string
	for index, match := range matches {
		start := match[0]
		end := len(text)
		if index+1 < len(matches) {
			end = matches[index+1][0]
		}
		segment := text[start:end]
		day := parseDayToken(text[match[2]:match[3]])
		if day == 0 {
			warnings = append(warnings, "未识别到有效星期")
			continue
		}
		periods := parsePeriods(segment)
		if len(periods) == 0 {
			warnings = append(warnings, "未识别到节次信息: "+segment)
			continue
		}
		slots = append(slots, courseSlot{Day: day, Periods: periods, Weeks: parseWeeksWithHDUAlternation(segment)})
	}
	return slots, len(slots) > 0, uniqueStrings(warnings)
}

func parseDayToken(token string) int {
	switch token {
	case "一", "1":
		return 1
	case "二", "2":
		return 2
	case "三", "3":
		return 3
	case "四", "4":
		return 4
	case "五", "5":
		return 5
	case "六", "6":
		return 6
	case "日", "天", "7":
		return 7
	default:
		return 0
	}
}

func parsePeriods(segment string) map[int]bool {
	result := make(map[int]bool)
	match := periodRe.FindStringSubmatch(segment)
	if len(match) < 2 {
		return result
	}
	for _, part := range strings.Split(match[1], ",") {
		start, end, ok := parseNumberRange(part)
		if !ok {
			continue
		}
		for period := start; period <= end; period++ {
			if period > 0 && period <= 20 {
				result[period] = true
			}
		}
	}
	return result
}

func parseWeeks(segment string) map[int]bool {
	result := make(map[int]bool)
	matches := weekRe.FindAllStringSubmatch(segment, -1)
	if len(matches) == 0 {
		for week := 1; week <= 20; week++ {
			result[week] = true
		}
		return result
	}
	for _, match := range matches {
		value := match[1]
		if match[2] != "" {
			value += "-" + match[2]
		}
		start, end, ok := parseNumberRange(value)
		if !ok {
			continue
		}
		for week := start; week <= end; week++ {
			if week > 0 && week <= 30 {
				result[week] = true
			}
		}
	}
	if strings.Contains(segment, "单周") {
		for week := range result {
			if week%2 == 0 {
				delete(result, week)
			}
		}
	}
	if strings.Contains(segment, "双周") {
		for week := range result {
			if week%2 == 1 {
				delete(result, week)
			}
		}
	}
	return result
}

func parseWeeksWithHDUAlternation(segment string) map[int]bool {
	weeks := parseWeeks(segment)
	if strings.Contains(segment, "(\u5355)") {
		for week := range weeks {
			if week%2 == 0 {
				delete(weeks, week)
			}
		}
	}
	if strings.Contains(segment, "(\u53cc)") {
		for week := range weeks {
			if week%2 == 1 {
				delete(weeks, week)
			}
		}
	}
	return weeks
}

func parseNumberRange(value string) (int, int, bool) {
	value = strings.TrimSpace(value)
	if value == "" {
		return 0, 0, false
	}
	parts := strings.Split(value, "-")
	start, err := parseSmallInt(parts[0])
	if err != nil {
		return 0, 0, false
	}
	end := start
	if len(parts) > 1 && strings.TrimSpace(parts[1]) != "" {
		var err error
		end, err = parseSmallInt(parts[1])
		if err != nil {
			return 0, 0, false
		}
	}
	if end < start {
		start, end = end, start
	}
	return start, end, true
}

func parseSmallInt(value string) (int, error) {
	var result int
	_, err := fmt.Sscanf(strings.TrimSpace(value), "%d", &result)
	return result, err
}

func slotsConflict(left []courseSlot, right []courseSlot) bool {
	for _, a := range left {
		for _, b := range right {
			if a.Day == b.Day && intSetOverlap(a.Periods, b.Periods) && intSetOverlap(a.Weeks, b.Weeks) {
				return true
			}
		}
	}
	return false
}

func intSetOverlap(left map[int]bool, right map[int]bool) bool {
	for value := range left {
		if right[value] {
			return true
		}
	}
	return false
}

func uniqueStrings(values []string) []string {
	seen := make(map[string]bool)
	var result []string
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" || seen[value] {
			continue
		}
		seen[value] = true
		result = append(result, value)
	}
	return result
}

func expectedConfirmationPhrase(hasDropRisk bool) string {
	if hasDropRisk {
		return "我确认退课风险并准备执行"
	}
	return "我确认准备执行"
}

func stableHash(value any) string {
	data, _ := json.Marshal(value)
	sum := sha256.Sum256(data)
	return fmt.Sprintf("%x", sum[:])
}

func planExecutionHash(plan ActionPlan) string {
	return stableHash(map[string]any{
		"term":   plan.Term,
		"select": courseCodes(plan.Select),
		"drop":   courseCodes(plan.Drop),
		"locked": courseCodes(plan.Locked),
	})
}

func configExecutionHash(cfg KillCourseConfig) string {
	return stableHash(map[string]any{
		"time":   cfg.Time,
		"course": cfg.Course,
	})
}

func courseCodes(items []Course) []string {
	codes := make([]string, 0, len(items))
	for _, item := range items {
		if item.DisplayCode != "" {
			codes = append(codes, item.DisplayCode)
		}
	}
	sort.Strings(codes)
	return codes
}

func configFileMatchesGenerated(configPath string, generated KillCourseConfig) (bool, bool) {
	var onDisk KillCourseConfig
	data, err := os.ReadFile(configPath)
	if err != nil {
		return false, false
	}
	if err := json.Unmarshal(data, &onDisk); err != nil {
		return true, false
	}
	if !reflect.DeepEqual(onDisk, generated) {
		return true, false
	}
	return true, true
}

func readExistingCourseActions(configPath string) map[string]string {
	var cfg KillCourseConfig
	if data, err := os.ReadFile(configPath); err == nil {
		_ = json.Unmarshal(data, &cfg)
	}
	return copyActionMap(cfg.Course)
}

func countExecutionActions(actions map[string]string) ExecutionActionCounts {
	var counts ExecutionActionCounts
	for _, action := range actions {
		switch action {
		case "1":
			counts.Select++
		case "0":
			counts.Drop++
		}
	}
	counts.Total = counts.Select + counts.Drop
	return counts
}

func killCourseCommand(p paths) (string, bool) {
	if entry := killCourseEntryPath(p); entry != "" {
		if filepath.Ext(entry) == ".go" {
			return "go run ./cmd/HDU-KillCourse", true
		}
		return entry, true
	}
	return "未找到可执行入口", false
}

func killCourseEntryPath(p paths) string {
	for _, name := range []string{"HDU-KillCourse.exe", "HDU-KillCourse-main.exe", "main.exe"} {
		candidate := filepath.Join(p.killCourseDir, name)
		if fileExists(candidate) {
			return candidate
		}
	}
	entry := filepath.Join(p.killCourseDir, "cmd", "HDU-KillCourse", "main.go")
	if fileExists(entry) {
		return entry
	}
	return ""
}

func readExecutionLogOffset(manifestPath string) (int64, bool) {
	if strings.TrimSpace(manifestPath) == "" {
		return 0, false
	}
	data, err := os.ReadFile(manifestPath)
	if err != nil {
		return 0, false
	}
	var manifest struct {
		Package struct {
			LogStartOffset int64 `json:"logStartOffset"`
		} `json:"package"`
	}
	if err := json.Unmarshal(data, &manifest); err != nil {
		return 0, false
	}
	return manifest.Package.LogStartOffset, true
}

func hasUsableCookie(cfg KillCourseConfig) bool {
	return cfg.Cookies.Enabled == "1" &&
		strings.TrimSpace(cfg.Cookies.JSESSIONID) != "" &&
		strings.TrimSpace(cfg.Cookies.Route) != ""
}

func actionsMatchTerm(actions map[string]string, xueNian string, xueQi string) bool {
	if xueNian == "" || xueQi == "" {
		return false
	}
	for code := range actions {
		if len(code) < 12 {
			return false
		}
		term := termFromCode(code)
		xn, xq := splitTerm(term)
		if xn != xueNian || xq != xueQi {
			return false
		}
	}
	return true
}

func actionsAreValid(actions map[string]string) bool {
	for code, action := range actions {
		if normalizeCode(code) == "" {
			return false
		}
		if action != "1" && action != "0" {
			return false
		}
	}
	return true
}

func validStartTime(value string) bool {
	if strings.TrimSpace(value) == "" {
		return false
	}
	_, err := time.ParseInLocation("2006-01-02 15:04:05", value, time.Local)
	return err == nil
}

func hasFailedWarning(checks []ReadinessCheck) bool {
	for _, check := range checks {
		if check.Level == "warning" && !check.Passed {
			return true
		}
	}
	return false
}

func copyActionMap(source map[string]string) map[string]string {
	result := make(map[string]string)
	for key, value := range source {
		result[key] = value
	}
	return result
}

func defaultKillCourseConfig(term string) KillCourseConfig {
	var cfg KillCourseConfig
	cfg.CasLogin.Username = ""
	cfg.CasLogin.Password = ""
	cfg.CasLogin.DingDingQrLoginEnabled = "0"
	cfg.CasLogin.Level = "0"
	cfg.NewJWLogin.Username = ""
	cfg.NewJWLogin.Password = ""
	cfg.NewJWLogin.Level = "1"
	cfg.Cookies.Enabled = "1"
	cfg.WaitCourse.Interval = 60
	cfg.WaitCourse.Enabled = "0"
	cfg.SMTPEmail.Enabled = "0"
	cfg.StartTime = time.Now().Format("2006-01-02 15:04:05")
	cfg.Course = make(map[string]string)
	xn, xq := splitTerm(term)
	if xn == "" {
		xn = fmt.Sprint(time.Now().Year())
	}
	if xq == "" {
		xq = "1"
	}
	cfg.Time.XueNian = xn
	cfg.Time.XueQi = xq
	return cfg
}

func redactedKillCourseConfig(cfg KillCourseConfig) KillCourseConfig {
	cfg.CasLogin.Password = ""
	cfg.NewJWLogin.Password = ""
	cfg.Cookies.JSESSIONID = ""
	cfg.Cookies.Route = ""
	cfg.SMTPEmail.Password = ""
	return cfg
}
func loadAgentSettings(file string) (AgentSettings, error) {
	var settings AgentSettings
	data, err := os.ReadFile(file)
	if err != nil {
		return defaultAgentSettings(), err
	}
	if err := json.Unmarshal(data, &settings); err != nil {
		return defaultAgentSettings(), err
	}
	return cleanAgentSettings(settings), nil
}

func defaultAgentSettings() AgentSettings {
	autoRefresh := true
	return AgentSettings{
		MainBaseURL:            defaultMainBaseURL,
		AutoRefresh:            &autoRefresh,
		RefreshIntervalSeconds: defaultRefreshIntervalSeconds,
		RefreshIntervalMinutes: 1,
	}
}

func cleanAgentSettings(settings AgentSettings) AgentSettings {
	cleaned := defaultAgentSettings()
	cleaned.SchedulerDir = cleanOptionalDir(settings.SchedulerDir)
	cleaned.KillCourseDir = cleanOptionalDir(settings.KillCourseDir)
	if strings.TrimSpace(settings.MainBaseURL) != "" {
		cleaned.MainBaseURL = strings.TrimRight(strings.TrimSpace(settings.MainBaseURL), "/")
	}
	if settings.AutoRefresh != nil {
		autoRefresh := *settings.AutoRefresh
		cleaned.AutoRefresh = &autoRefresh
	}
	if settings.RefreshIntervalSeconds != 0 {
		cleaned.RefreshIntervalSeconds = settings.RefreshIntervalSeconds
	} else if settings.RefreshIntervalMinutes != 0 {
		cleaned.RefreshIntervalSeconds = settings.RefreshIntervalMinutes * 60
	}
	cleaned.RefreshIntervalMinutes = legacyRefreshIntervalMinutes(cleaned.RefreshIntervalSeconds)
	return cleaned
}

func legacyRefreshIntervalMinutes(seconds int) int {
	if seconds <= 0 {
		return seconds
	}
	minutes := seconds / 60
	if seconds%60 != 0 {
		minutes++
	}
	if minutes == 0 {
		return 1
	}
	return minutes
}

func cleanOptionalDir(value string) string {
	text := strings.TrimSpace(value)
	if text == "" {
		return ""
	}
	if cleaned, err := filepath.Abs(text); err == nil {
		return filepath.Clean(cleaned)
	}
	return filepath.Clean(text)
}

func validateAgentSettings(settings AgentSettings) error {
	settings = cleanAgentSettings(settings)
	if settings.SchedulerDir != "" && !dirExists(settings.SchedulerDir) {
		return fmt.Errorf("排课助手目录不存在：%s", settings.SchedulerDir)
	}
	if settings.KillCourseDir != "" && !dirExists(settings.KillCourseDir) {
		return fmt.Errorf("HDU-KillCourse 目录不存在：%s", settings.KillCourseDir)
	}
	if settings.KillCourseDir != "" && !looksLikeKillCourseDir(settings.KillCourseDir) {
		return fmt.Errorf("HDU-KillCourse 目录缺少可识别入口：%s", settings.KillCourseDir)
	}
	if err := validateMainBaseURL(settings.MainBaseURL); err != nil {
		return err
	}
	if settings.RefreshIntervalSeconds < minRefreshIntervalSeconds || settings.RefreshIntervalSeconds > maxRefreshIntervalSeconds {
		return fmt.Errorf("自动刷新间隔应在 %d 到 %d 秒之间", minRefreshIntervalSeconds, maxRefreshIntervalSeconds)
	}
	return nil
}

func validateMainBaseURL(value string) error {
	text := strings.TrimSpace(value)
	if text == "" {
		return nil
	}
	parsed, err := neturl.Parse(text)
	if err != nil || parsed.Scheme != "http" || parsed.Hostname() == "" {
		return fmt.Errorf("主站地址必须是 loopback HTTP 地址")
	}
	host := strings.Trim(parsed.Hostname(), "[]")
	if host != "localhost" {
		ip := net.ParseIP(host)
		if ip == nil || !ip.IsLoopback() {
			return fmt.Errorf("主站地址必须指向本机 loopback")
		}
	}
	return nil
}

func looksLikeKillCourseDir(dir string) bool {
	if fileExists(filepath.Join(dir, "config.example.json")) {
		return true
	}
	if fileExists(filepath.Join(dir, "cmd", "HDU-KillCourse", "main.go")) {
		return true
	}
	for _, name := range []string{"HDU-KillCourse.exe", "HDU-KillCourse-main.exe", "main.exe"} {
		if fileExists(filepath.Join(dir, name)) {
			return true
		}
	}
	return false
}

func discoverPaths() paths {
	wd, _ := os.Getwd()
	workspace := filepath.Clean(wd)
	parent := filepath.Dir(workspace)
	grandparent := filepath.Dir(parent)
	settingsPath := filepath.Join(workspace, "agent-settings.json")
	settings, _ := loadAgentSettings(settingsPath)

	schedulerDir := settings.SchedulerDir
	if schedulerDir == "" {
		schedulerCandidates := []string{
			filepath.Join(grandparent, "HDU-Auto-Scheduling-Script"),
			filepath.Join(parent, "HDU-Auto-Scheduling-Script"),
			parent,
			workspace,
		}
		schedulerDir = firstExistingDirWith(schedulerCandidates, "course.json")
		if schedulerDir == "" {
			schedulerDir = firstExistingDirWith(schedulerCandidates, "testdata")
		}
		if schedulerDir == "" {
			schedulerDir = parent
		}
	}

	killDir := settings.KillCourseDir
	if killDir == "" {
		killCandidates := []string{
			filepath.Join(grandparent, "HDU-KillCourse"),
			filepath.Join(grandparent, "HDU-KillCourse-main"),
			filepath.Join(grandparent, "HDU-KillCourse-main", "HDU-KillCourse-main"),
			filepath.Join(parent, "HDU-KillCourse"),
			filepath.Join(parent, "HDU-KillCourse-main"),
			filepath.Join(parent, "HDU-KillCourse-main", "HDU-KillCourse-main"),
			filepath.Join(workspace, "HDU-KillCourse"),
		}
		killDir = firstExistingDirWith(killCandidates, "config.example.json")
		if killDir == "" {
			killDir = filepath.Join(parent, "HDU-KillCourse")
		}
	}

	return paths{
		workspace:        workspace,
		settingsPath:     settingsPath,
		schedulerDir:     schedulerDir,
		downloadsDir:     defaultDownloadsDir(),
		killCourseDir:    killDir,
		coursePath:       filepath.Join(schedulerDir, "course.json"),
		personalPath:     filepath.Join(schedulerDir, "personal-schedule.json"),
		livePersonalPath: filepath.Join(schedulerDir, "personal-schedule-live.json"),
		liveSyncPath:     filepath.Join(workspace, "live-schedule-sync.json"),
		killConfigPath:   filepath.Join(killDir, "config.json"),
		actionPlanPath:   filepath.Join(workspace, "action-plan.json"),
		approvalPath:     filepath.Join(workspace, "execution-approval.json"),
		runBatchPath:     filepath.Join(workspace, "run-killcourse.bat"),
		runbookPath:      filepath.Join(workspace, "execution-runbook.md"),
		manifestPath:     filepath.Join(workspace, "execution-package.json"),
		executionLogPath: filepath.Join(workspace, "execution-log.json"),
		fallbackRecsPath: filepath.Join(workspace, "fallback-recommendations.json"),
	}
}

func defaultDownloadsDir() string {
	if override := strings.TrimSpace(os.Getenv("HDU_AGENT_DOWNLOADS_DIR")); override != "" {
		return filepath.Clean(override)
	}
	home, err := os.UserHomeDir()
	if err != nil || strings.TrimSpace(home) == "" {
		return ""
	}
	return filepath.Join(home, "Downloads")
}

func discoverTargetSchedule(p paths) (targetScheduleCandidate, []string) {
	directories := uniqueNonEmptyPaths(p.workspace, p.schedulerDir, p.downloadsDir)
	fixedNames := []string{"target-schedule.json", "hdu-target-timetable.json"}
	var warnings []string
	seen := map[string]bool{}

	for _, dir := range directories {
		for _, name := range fixedNames {
			file := filepath.Join(dir, name)
			if seen[file] || !fileExists(file) {
				continue
			}
			seen[file] = true
			candidate, err := loadTargetScheduleFile(file)
			if err != nil {
				warnings = append(warnings, fmt.Sprintf("目标课表文件无效：%s（%v）", file, err))
				continue
			}
			return candidate, warnings
		}
	}

	legacyFiles := map[string]os.FileInfo{}
	for _, dir := range directories {
		for _, pattern := range []string{"hdu-target-timetable-*.json"} {
			matches, _ := filepath.Glob(filepath.Join(dir, pattern))
			for _, file := range matches {
				if seen[file] {
					continue
				}
				info, err := os.Stat(file)
				if err == nil && info.Mode().IsRegular() {
					legacyFiles[file] = info
				}
				seen[file] = true
			}
		}
	}

	files := make([]string, 0, len(legacyFiles))
	for file := range legacyFiles {
		files = append(files, file)
	}
	sort.Slice(files, func(i, j int) bool {
		return legacyFiles[files[i]].ModTime().After(legacyFiles[files[j]].ModTime())
	})
	for _, file := range files {
		candidate, err := loadTargetScheduleFile(file)
		if err != nil {
			warnings = append(warnings, fmt.Sprintf("目标课表文件无效：%s（%v）", file, err))
			continue
		}
		return candidate, warnings
	}
	return targetScheduleCandidate{}, warnings
}

func uniqueNonEmptyPaths(values ...string) []string {
	seen := make(map[string]bool, len(values))
	result := make([]string, 0, len(values))
	for _, value := range values {
		cleaned := filepath.Clean(strings.TrimSpace(value))
		if cleaned == "." || cleaned == "" || seen[cleaned] {
			continue
		}
		seen[cleaned] = true
		result = append(result, cleaned)
	}
	return result
}

func loadTargetScheduleFile(file string) (targetScheduleCandidate, error) {
	items, payload, err := loadCourses(file)
	if err != nil {
		return targetScheduleCandidate{}, err
	}
	if len(items) == 0 {
		return targetScheduleCandidate{}, errors.New("目标课表为空")
	}
	info, err := os.Stat(file)
	if err != nil {
		return targetScheduleCandidate{}, err
	}
	return targetScheduleCandidate{
		Path:      filepath.Clean(file),
		UpdatedAt: info.ModTime().Format(time.RFC3339),
		Payload:   payload,
		Items:     items,
	}, nil
}

func statusFromPaths(p paths) StatusResponse {
	target, _ := discoverTargetSchedule(p)
	resp := StatusResponse{
		WorkspaceDir:       p.workspace,
		SettingsPath:       p.settingsPath,
		SchedulerDir:       p.schedulerDir,
		KillCourseDir:      p.killCourseDir,
		CoursePath:         p.coursePath,
		PersonalPath:       p.personalPath,
		LivePersonalPath:   p.livePersonalPath,
		LiveSyncPath:       p.liveSyncPath,
		KillConfigPath:     p.killConfigPath,
		ActionPlanPath:     p.actionPlanPath,
		KillCourseExists:   dirExists(p.killCourseDir),
		CourseExists:       fileExists(p.coursePath),
		PersonalExists:     fileExists(p.personalPath),
		LivePersonalExists: fileExists(p.livePersonalPath),
		CanFallback:        fileExists(p.coursePath),
		CanWriteKillConfig: dirExists(p.killCourseDir),
		TargetPath:         target.Path,
		TargetExists:       target.Path != "",
		TargetUpdatedAt:    target.UpdatedAt,
	}
	if courses, payload, err := loadCourses(p.coursePath); err == nil {
		resp.CourseCount = len(courses)
		resp.Term = inferTerm(payload.Term, courses)
	}
	if personal, _, err := loadCourses(p.personalPath); err == nil {
		resp.PersonalCount = len(personal)
	}
	if live, _, err := loadCourses(p.livePersonalPath); err == nil {
		resp.LivePersonalCount = len(live)
	}
	resp.TargetCount = len(target.Items)
	switch {
	case !resp.CourseExists:
		resp.Message = "未检测到 course.json，可先通过排课助手导出课程数据。"
	case !resp.PersonalExists:
		resp.Message = "已检测到 course.json，但 personal-schedule.json 缺失；仍可生成只选不退的目标计划。"
	default:
		resp.Message = "数据已就绪，可导入目标课表生成执行计划。"
	}
	return resp
}

func loadCourses(file string) ([]Course, CoursePayload, error) {
	data, err := os.ReadFile(file)
	if err != nil {
		return nil, CoursePayload{}, err
	}
	var payload CoursePayload
	if err := json.Unmarshal(data, &payload); err != nil || payload.Items == nil {
		var list []map[string]any
		if listErr := json.Unmarshal(data, &list); listErr != nil {
			if err != nil {
				return nil, CoursePayload{}, err
			}
			return nil, CoursePayload{}, listErr
		}
		payload.Items = list
	}
	courses, normalized, err := normalizePayload(payload)
	return courses, normalized, err
}

func loadCurrentCourses(p paths) ([]Course, CoursePayload, string, error) {
	if courses, payload, err := loadCourses(p.livePersonalPath); err == nil {
		return courses, payload, "live", nil
	}
	courses, payload, err := loadCourses(p.personalPath)
	return courses, payload, "local", err
}

func scheduleHash(items []Course) string {
	hashed := make([]string, 0, len(items))
	for _, item := range items {
		hashed = append(hashed, courseSemanticHash(item))
	}
	sort.Strings(hashed)
	return stableHash(hashed)
}

func courseSemanticHash(item Course) string {
	return stableHash(map[string]any{
		"displayCode":   item.DisplayCode,
		"groupId":       item.GroupID,
		"rawCourseCode": item.RawCourseCode,
		"courseName":    item.CourseName,
		"sectionName":   item.SectionName,
		"teacher":       item.Teacher,
		"timeText":      item.TimeText,
		"location":      item.Location,
		"className":     item.ClassName,
		"credits":       item.Credits,
		"selectEnabled": item.SelectEnabled,
		"dropEnabled":   item.DropEnabled,
		"selectRounds":  item.SelectRounds,
		"capacity":      item.Capacity,
		"enrolled":      item.Enrolled,
		"selected":      item.Selected,
	})
}

func readLiveScheduleSync(file string) (LiveScheduleSync, error) {
	var sync LiveScheduleSync
	data, err := os.ReadFile(file)
	if err != nil {
		return sync, err
	}
	if err := json.Unmarshal(data, &sync); err != nil {
		return sync, err
	}
	return sync, nil
}

func readOrBuildLiveScheduleSync(p paths) (LiveScheduleSync, []string) {
	sync, err := readLiveScheduleSync(p.liveSyncPath)
	if err == nil {
		sync.Added = nonNilCourses(sync.Added)
		sync.Removed = nonNilCourses(sync.Removed)
		sync.Changed = nonNilCourses(sync.Changed)
		sync.Unchanged = nonNilCourses(sync.Unchanged)
		return sync, nil
	}
	return buildLiveScheduleSync(p)
}

func buildLiveScheduleSync(p paths) (LiveScheduleSync, []string) {
	return buildLiveScheduleSyncWithSource(p, "file-bridge")
}

func buildLiveScheduleSyncWithSource(p paths, source string) (LiveScheduleSync, []string) {
	var warnings []string
	local, _, err := loadCourses(p.personalPath)
	if err != nil {
		local = []Course{}
		warnings = append(warnings, "personal-schedule.json is missing or unreadable; drift comparison used an empty local baseline")
	}
	live, _, err := loadCourses(p.livePersonalPath)
	if err != nil {
		live = []Course{}
		warnings = append(warnings, "personal-schedule-live.json is missing or unreadable")
	}
	return buildLiveScheduleSyncFromItemsWithLocal(p, local, live, source, warnings)
}

func buildLiveScheduleSyncFromItems(p paths, live []Course, source string, warnings []string) (LiveScheduleSync, []string) {
	local, _, err := loadCourses(p.personalPath)
	if err != nil {
		local = []Course{}
		warnings = append(warnings, "personal-schedule.json is missing or unreadable; drift comparison used an empty local baseline")
	}
	return buildLiveScheduleSyncFromItemsWithLocal(p, local, live, source, warnings)
}

func buildLiveScheduleSyncFromItemsWithLocal(p paths, local, live []Course, source string, warnings []string) (LiveScheduleSync, []string) {
	localMap := mapByCode(local)
	liveMap := mapByCode(live)

	var added, removed, changed, unchanged []Course
	for code, item := range liveMap {
		localItem, ok := localMap[code]
		if !ok {
			added = append(added, item)
		} else if courseSemanticHash(localItem) != courseSemanticHash(item) {
			changed = append(changed, item)
		} else {
			unchanged = append(unchanged, item)
		}
	}
	for code, item := range localMap {
		if _, ok := liveMap[code]; !ok {
			removed = append(removed, item)
		}
	}

	return LiveScheduleSync{
		SchemaVersion: schemaVersion,
		Source:        firstNonEmpty(source, "file-bridge"),
		SyncedAt:      time.Now().Format(time.RFC3339),
		LocalPath:     p.personalPath,
		LivePath:      p.livePersonalPath,
		LocalCount:    len(local),
		LiveCount:     len(live),
		LocalHash:     scheduleHash(local),
		LiveHash:      scheduleHash(live),
		HasDrift:      len(added) > 0 || len(removed) > 0 || len(changed) > 0,
		Added:         sortCourses(added),
		Removed:       sortCourses(removed),
		Changed:       sortCourses(changed),
		Unchanged:     sortCourses(unchanged),
	}, warnings
}

func nonNilCourses(items []Course) []Course {
	if items == nil {
		return []Course{}
	}
	return items
}

func normalizePayload(payload CoursePayload) ([]Course, CoursePayload, error) {
	if payload.Items == nil {
		return nil, payload, errors.New("JSON 缺少 items 数组")
	}
	payload.Items = mergeCourseRawItems(payload.Items)
	courses := make([]Course, 0, len(payload.Items))
	for index, raw := range payload.Items {
		course := normalizeCourse(raw, index)
		if course.DisplayCode == "" {
			continue
		}
		courses = append(courses, course)
	}
	payload.SchemaVersion = schemaVersion
	return courses, payload, nil
}

// mergeCourseRawItems merges rows that belong to the same teaching class so a
// teaching class is counted once. Weekly sessions are combined into one
// schedule string while keeping location changes visible.
func mergeCourseRawItems(items []map[string]any) []map[string]any {
	result := make([]map[string]any, 0, len(items))
	byKey := make(map[string]int, len(items))
	for _, raw := range items {
		key := firstNonEmpty(
			firstText(raw["id"]),
			firstText(raw["sectionId"]),
			firstText(raw["jxb_id"]),
			extractSectionCode(firstText(raw["displayCode"]), firstText(raw["jxbmc"]), firstText(raw["sectionName"]), firstText(raw["Jxbmc"])),
		)
		if key == "" {
			result = append(result, raw)
			continue
		}
		if idx, ok := byKey[key]; ok {
			mergeCourseRawText(result[idx], raw, []string{"sksj", "timeText", "time", "schedule"})
			mergeCourseRawText(result[idx], raw, []string{"jxdd", "location", "cdlbmc"})
			continue
		}
		byKey[key] = len(result)
		result = append(result, raw)
	}
	return result
}

func mergeCourseRawText(existing, next map[string]any, keys []string) {
	for _, key := range keys {
		left := strings.TrimSpace(firstText(existing[key]))
		right := strings.TrimSpace(firstText(next[key]))
		if right == "" {
			continue
		}
		if left == "" {
			existing[key] = right
			continue
		}
		if !strings.Contains(left, right) {
			existing[key] = left + ";" + right
		}
	}
}

func normalizeCourse(raw map[string]any, index int) Course {
	display := extractSectionCode(
		firstText(raw["displayCode"]),
		firstText(raw["jxbmc"]),
		firstText(raw["sectionName"]),
		firstText(raw["Jxbmc"]),
	)
	sectionName := firstNonEmpty(display, firstText(raw["jxbmc"]), firstText(raw["sectionName"]), firstText(raw["Jxbmc"]))
	courseName := firstNonEmpty(firstText(raw["courseName"]), firstText(raw["kcmc"]), firstText(raw["name"]), "未命名课程")
	rawCode := firstNonEmpty(firstText(raw["courseCode"]), firstText(raw["kch"]), firstText(raw["kch_id"]), firstText(raw["courseId"]))
	groupID := firstNonEmpty(firstText(raw["groupId"]), baseCourseCode(display, rawCode), rawCode, courseName)
	id := firstNonEmpty(firstText(raw["id"]), firstText(raw["sectionId"]), firstText(raw["jxb_id"]), display, fmt.Sprintf("%s-%d", groupID, index+1))
	return Course{
		SchemaVersion: schemaVersion,
		ID:            id,
		DisplayCode:   display,
		GroupID:       groupID,
		RawCourseCode: rawCode,
		CourseName:    courseName,
		SectionName:   sectionName,
		Teacher:       firstNonEmpty(firstText(raw["teacher"]), firstText(raw["jzgxx"]), firstText(raw["jsxm"]), firstText(raw["js"]), "未填写教师"),
		TimeText:      firstNonEmpty(firstText(raw["timeText"]), firstText(raw["sksj"]), firstText(raw["time"]), firstText(raw["schedule"])),
		Location:      firstNonEmpty(firstText(raw["location"]), firstText(raw["jxdd"]), firstText(raw["cdlbmc"])),
		ClassName:     firstNonEmpty(firstText(raw["className"]), firstText(raw["jxbzc"]), firstText(raw["bjmc"])),
		Credits:       parseCredits(firstNonEmpty(firstText(raw["credits"]), firstText(raw["xf"]), firstText(raw["credit"]))),
		SelectEnabled: optionalBool(raw, "selectEnabled", "canSelect", "xkEnabled", "selectable", "可选"),
		DropEnabled:   optionalBool(raw, "dropEnabled", "canDrop", "tkEnabled", "droppable", "可退"),
		SelectRounds:  optionalRounds(raw, "selectRounds", "selectionRounds", "selectRound", "rounds", "xkRounds", "选课轮次"),
		Capacity:      optionalInt(raw, "capacity", "jxbrl", "教学班容量"),
		Enrolled:      optionalInt(raw, "enrolled", "jxbrs", "教学班人数"),
		Selected:      optionalInt(raw, "selected", "xkrs", "选课人数"),
		Raw:           raw,
	}
}

func optionalBool(raw map[string]any, keys ...string) *bool {
	for _, key := range keys {
		value, ok := raw[key]
		if !ok || value == nil {
			continue
		}
		switch item := value.(type) {
		case bool:
			result := item
			return &result
		case float64:
			result := item != 0
			return &result
		case int:
			result := item != 0
			return &result
		}
		text := strings.ToLower(strings.TrimSpace(firstText(value)))
		switch text {
		case "1", "true", "yes", "y", "是", "可", "允许":
			result := true
			return &result
		case "0", "false", "no", "n", "否", "不可", "禁止":
			result := false
			return &result
		}
	}
	return nil
}

func optionalInt(raw map[string]any, keys ...string) *int {
	for _, key := range keys {
		if value, ok := raw[key]; ok {
			if result := parseOptionalInt(value); result != nil {
				return result
			}
		}
	}
	return nil
}

func parseOptionalInt(value any) *int {
	if value == nil {
		return nil
	}
	text := strings.TrimSpace(firstText(value))
	if text == "" {
		return nil
	}
	if result, err := strconv.Atoi(text); err == nil {
		return &result
	}
	if result, err := strconv.ParseFloat(text, 64); err == nil {
		integer := int(result)
		if float64(integer) == result {
			return &integer
		}
	}
	return nil
}

func optionalRounds(raw map[string]any, keys ...string) []int {
	var rounds []int
	var collect func(any)
	collect = func(value any) {
		if value == nil {
			return
		}
		switch item := value.(type) {
		case []any:
			for _, part := range item {
				collect(part)
			}
		case []int:
			rounds = append(rounds, item...)
		case string:
			text := strings.NewReplacer("第", "", "轮", "", "周", "").Replace(item)
			for _, part := range strings.FieldsFunc(text, func(r rune) bool {
				return r == ',' || r == '，' || r == ';' || r == '；' || r == '/' || r == '|' || r == ' ' || r == '\t'
			}) {
				collect(part)
			}
		default:
			if result := parseOptionalInt(value); result != nil {
				rounds = append(rounds, *result)
			}
		}
	}
	for _, key := range keys {
		if value, ok := raw[key]; ok {
			collect(value)
			if len(rounds) > 0 {
				break
			}
		}
	}
	if len(rounds) == 0 {
		return nil
	}
	sort.Ints(rounds)
	result := rounds[:0]
	for _, round := range rounds {
		if round > 0 && (len(result) == 0 || result[len(result)-1] != round) {
			result = append(result, round)
		}
	}
	return result
}

func validateTarget(items []Course) []string {
	var warnings []string
	for _, item := range items {
		if item.CourseName == "" || item.CourseName == "未命名课程" {
			warnings = append(warnings, item.DisplayCode+" 缺少课程名称")
		}
		if item.GroupID == "" {
			warnings = append(warnings, item.DisplayCode+" 缺少课程号")
		}
		if item.TimeText == "" {
			warnings = append(warnings, item.DisplayCode+" 缺少上课时间")
		}
		if item.Teacher == "" || item.Teacher == "未填写教师" {
			warnings = append(warnings, item.DisplayCode+" 未填写教师")
		}
	}
	return warnings
}

type preparedJSONFile struct {
	target      string
	temp        string
	backup      string
	hadOriginal bool
	installed   bool
}

func prepareJSONFile(file string, value any) (*preparedJSONFile, error) {
	if err := os.MkdirAll(filepath.Dir(file), 0755); err != nil {
		return nil, err
	}
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return nil, err
	}
	temp, err := os.CreateTemp(filepath.Dir(file), ".hdu-smart-agent-*.tmp")
	if err != nil {
		return nil, err
	}
	tempName := temp.Name()
	if err := temp.Chmod(0644); err != nil {
		_ = temp.Close()
		_ = os.Remove(tempName)
		return nil, err
	}
	if _, err := temp.Write(append(data, '\n')); err != nil {
		_ = temp.Close()
		_ = os.Remove(tempName)
		return nil, err
	}
	if err := temp.Sync(); err != nil {
		_ = temp.Close()
		_ = os.Remove(tempName)
		return nil, err
	}
	if err := temp.Close(); err != nil {
		_ = os.Remove(tempName)
		return nil, err
	}
	return &preparedJSONFile{target: file, temp: tempName}, nil
}

func discardPreparedJSONFile(file *preparedJSONFile) {
	if file == nil {
		return
	}
	_ = os.Remove(file.temp)
	if file.backup != "" {
		_ = os.Remove(file.backup)
	}
}

func commitPreparedJSONFiles(files ...*preparedJSONFile) (err error) {
	defer func() {
		if err != nil {
			for index := len(files) - 1; index >= 0; index-- {
				file := files[index]
				if file == nil {
					continue
				}
				if file.installed {
					_ = os.Remove(file.target)
				}
				if file.hadOriginal && file.backup != "" {
					_ = os.Rename(file.backup, file.target)
				}
			}
		}
		for _, file := range files {
			discardPreparedJSONFile(file)
		}
	}()

	for _, file := range files {
		if file == nil {
			return errors.New("不能提交空的 JSON 文件")
		}
		if _, statErr := os.Lstat(file.target); statErr == nil {
			backup, backupErr := os.CreateTemp(filepath.Dir(file.target), ".hdu-smart-agent-*.bak")
			if backupErr != nil {
				return backupErr
			}
			file.backup = backup.Name()
			if closeErr := backup.Close(); closeErr != nil {
				return closeErr
			}
			if removeErr := os.Remove(file.backup); removeErr != nil {
				return removeErr
			}
			if renameErr := os.Rename(file.target, file.backup); renameErr != nil {
				return renameErr
			}
			file.hadOriginal = true
		} else if !errors.Is(statErr, os.ErrNotExist) {
			return statErr
		}
	}
	for _, file := range files {
		if err := os.Rename(file.temp, file.target); err != nil {
			return err
		}
		file.installed = true
	}
	return nil
}

func writeJSONFile(file string, value any) error {
	prepared, err := prepareJSONFile(file, value)
	if err != nil {
		return err
	}
	return commitPreparedJSONFiles(prepared)
}

func mapByCode(items []Course) map[string]Course {
	result := make(map[string]Course)
	for _, item := range items {
		code := normalizeCode(item.DisplayCode)
		if code == "" {
			continue
		}
		result[code] = item
	}
	return result
}

func makeCodeSet(values []string) map[string]bool {
	result := make(map[string]bool)
	for _, value := range values {
		code := normalizeCode(value)
		if code != "" {
			result[code] = true
		}
	}
	return result
}

func dedupeCourses(items []Course) []Course {
	seen := make(map[string]bool)
	var result []Course
	for _, item := range items {
		code := normalizeCode(item.DisplayCode)
		if code == "" || seen[code] {
			continue
		}
		seen[code] = true
		result = append(result, item)
	}
	return result
}

func dedupeValidationIssues(items []ValidationIssue) []ValidationIssue {
	seen := make(map[string]bool)
	var result []ValidationIssue
	for _, item := range items {
		key := item.Level + "|" + item.Code + "|" + item.Message
		if seen[key] {
			continue
		}
		seen[key] = true
		result = append(result, item)
	}
	return result
}

func sortCourses(items []Course) []Course {
	next := append([]Course(nil), items...)
	sort.Slice(next, func(i, j int) bool {
		return next[i].DisplayCode < next[j].DisplayCode
	})
	return next
}

func sortValidationIssues(items []ValidationIssue) []ValidationIssue {
	next := append([]ValidationIssue(nil), items...)
	sort.Slice(next, func(i, j int) bool {
		return next[i].Level+next[i].Code+next[i].Message < next[j].Level+next[j].Code+next[j].Message
	})
	return next
}

func sortExplanations(items []PlanExplanation) []PlanExplanation {
	next := append([]PlanExplanation(nil), items...)
	sort.Slice(next, func(i, j int) bool {
		return next[i].Category+next[i].Code+next[i].Message < next[j].Category+next[j].Code+next[j].Message
	})
	return next
}

func courseTitle(item Course) string {
	return firstNonEmpty(item.CourseName, item.DisplayCode, item.SectionName, "未命名课程")
}

var sectionRe = regexp.MustCompile(`\(\d{4}-\d{4}-\d\)-[A-Za-z0-9]+-\d{1,3}[A-Za-z]*`)
var termRe = regexp.MustCompile(`\((\d{4})-\d{4}-(\d)\)`)

func extractSectionCode(values ...string) string {
	for _, value := range values {
		if match := sectionRe.FindString(value); match != "" {
			return match
		}
	}
	return ""
}

func normalizeCode(value string) string {
	return extractSectionCode(value)
}

func baseCourseCode(values ...string) string {
	for _, value := range values {
		text := strings.TrimSpace(value)
		if text == "" {
			continue
		}
		if code := extractSectionCode(text); code != "" {
			return regexp.MustCompile(`-\d{1,3}[A-Za-z]*$`).ReplaceAllString(code, "")
		}
		text = regexp.MustCompile(`-\d{1,3}[A-Za-z]*$`).ReplaceAllString(text, "")
		if strings.Contains(text, ")") {
			return text
		}
	}
	return ""
}

func inferTerm(explicit string, values ...any) string {
	if strings.TrimSpace(explicit) != "" {
		return strings.TrimSpace(explicit)
	}
	for _, value := range values {
		switch item := value.(type) {
		case Course:
			if term := termFromCode(item.DisplayCode); term != "" {
				return term
			}
		case []Course:
			for _, course := range item {
				if term := termFromCode(course.DisplayCode); term != "" {
					return term
				}
			}
		case string:
			if term := termFromCode(item); term != "" {
				return term
			}
		}
	}
	return ""
}

func termFromCode(code string) string {
	match := termRe.FindStringSubmatch(code)
	if len(match) == 3 {
		return strings.Trim(match[0], "()")
	}
	return ""
}

func splitTerm(term string) (string, string) {
	parts := strings.Split(term, "-")
	if len(parts) == 3 {
		return parts[0], parts[2]
	}
	return "", ""
}

func firstText(value any) string {
	if value == nil {
		return ""
	}
	switch item := value.(type) {
	case string:
		return strings.TrimSpace(item)
	case float64:
		return fmt.Sprintf("%g", item)
	case int:
		return fmt.Sprintf("%d", item)
	default:
		return strings.TrimSpace(fmt.Sprint(item))
	}
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func parseCredits(value string) float64 {
	text := regexp.MustCompile(`[^\d.]`).ReplaceAllString(value, "")
	var result float64
	_, _ = fmt.Sscanf(text, "%f", &result)
	return result
}

func fileExists(file string) bool {
	info, err := os.Stat(file)
	return err == nil && !info.IsDir()
}

func dirExists(dir string) bool {
	info, err := os.Stat(dir)
	return err == nil && info.IsDir()
}

func firstExistingDirWith(candidates []string, child string) string {
	for _, candidate := range candidates {
		if candidate == "" {
			continue
		}
		if _, err := os.Stat(filepath.Join(candidate, child)); err == nil {
			return filepath.Clean(candidate)
		}
	}
	return ""
}

func withLocalCORS(next http.Handler, port string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := r.Header.Get("Origin")
		if origin != "" {
			parsed, err := neturl.Parse(origin)
			if err != nil || !isAllowedLocalOrigin(parsed, port) {
				http.Error(w, "forbidden origin", http.StatusForbidden)
				return
			}
			w.Header().Set("Access-Control-Allow-Origin", origin)
			w.Header().Set("Vary", "Origin")
		}
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
		w.Header().Set("Access-Control-Allow-Methods", "GET,POST,OPTIONS")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func isAllowedLocalOrigin(origin *neturl.URL, port string) bool {
	host := strings.ToLower(origin.Hostname())
	return origin.Scheme == "http" &&
		origin.Port() == port &&
		(host == "127.0.0.1" || host == "localhost" || host == "::1")
}

func contentType(name string) string {
	switch strings.ToLower(filepath.Ext(name)) {
	case ".html":
		return "text/html; charset=utf-8"
	case ".css":
		return "text/css; charset=utf-8"
	case ".js":
		return "application/javascript; charset=utf-8"
	default:
		return "application/octet-stream"
	}
}

func writeJSON(w http.ResponseWriter, value any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	_ = json.NewEncoder(w).Encode(value)
}

func openBrowser(url string) {
	time.Sleep(600 * time.Millisecond)
	switch runtime.GOOS {
	case "windows":
		_ = exec.Command("cmd", "/c", "start", "", url).Start()
	case "darwin":
		_ = exec.Command("open", url).Start()
	default:
		_ = exec.Command("xdg-open", url).Start()
	}
}
