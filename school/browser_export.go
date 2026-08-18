package school

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"
)

const defaultBrowserProxyURL = "http://127.0.0.1:3456"

type browserBridge struct {
	proxyURL string
	client   *http.Client
}

type browserTarget struct {
	TargetID string `json:"targetId"`
	Type     string `json:"type"`
	URL      string `json:"url"`
}

type browserFetchResponse struct {
	Status int    `json:"status"`
	Body   string `json:"body"`
}

func newBrowserBridge(proxyURL string, client *http.Client) *browserBridge {
	if strings.TrimSpace(proxyURL) == "" {
		proxyURL = strings.TrimSpace(os.Getenv("HDU_BROWSER_PROXY_URL"))
	}
	if strings.TrimSpace(proxyURL) == "" {
		proxyURL = defaultBrowserProxyURL
	}
	if !isLoopbackBrowserProxyURL(proxyURL) {
		proxyURL = defaultBrowserProxyURL
	}
	if client == nil {
		client = &http.Client{}
	}
	return &browserBridge{proxyURL: strings.TrimRight(proxyURL, "/"), client: client}
}

func isLoopbackBrowserProxyURL(raw string) bool {
	parsed, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || parsed.User != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") {
		return false
	}
	host := strings.Trim(parsed.Hostname(), "[]")
	if host == "localhost" {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}

func (b *browserBridge) findTarget() (string, error) {
	targets, err := b.listTargets()
	if err != nil {
		return "", err
	}
	for _, target := range targets {
		if target.Type == "page" && strings.Contains(target.URL, "newjw.hdu.edu.cn") &&
			strings.Contains(target.URL, "index_initMenu") {
			return target.TargetID, nil
		}
	}
	for _, target := range targets {
		if target.Type == "page" && isCoursePage(target.URL) {
			return target.TargetID, nil
		}
	}
	for _, target := range targets {
		if target.Type == "page" && strings.Contains(target.URL, "newjw.hdu.edu.cn") &&
			!strings.Contains(target.URL, "login_slogin") {
			return target.TargetID, nil
		}
	}
	return b.createTarget()
}

func (b *browserBridge) listTargets() ([]browserTarget, error) {
	request, err := http.NewRequest(http.MethodGet, b.proxyURL+"/targets", nil)
	if err != nil {
		return nil, err
	}
	response, err := b.client.Do(request)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()
	if response.StatusCode >= http.StatusBadRequest {
		return nil, fmt.Errorf("browser proxy targets returned HTTP %d", response.StatusCode)
	}
	var targets []browserTarget
	if err := json.NewDecoder(response.Body).Decode(&targets); err != nil {
		return nil, err
	}
	return targets, nil
}

func (b *browserBridge) createTarget() (string, error) {
	return b.createTargetAt("https://newjw.hdu.edu.cn/jwglxt/xtgl/login_slogin.html")
}

func (b *browserBridge) createTargetAt(rawURL string) (string, error) {
	request, err := http.NewRequest(
		http.MethodPost,
		b.proxyURL+"/new",
		strings.NewReader(rawURL),
	)
	if err != nil {
		return "", err
	}
	request.Header.Set("Content-Type", "text/plain; charset=utf-8")
	response, err := b.client.Do(request)
	if err != nil {
		return "", err
	}
	defer response.Body.Close()
	if response.StatusCode >= http.StatusBadRequest {
		return "", fmt.Errorf("browser proxy new tab returned HTTP %d", response.StatusCode)
	}
	var target browserTarget
	if err := json.NewDecoder(response.Body).Decode(&target); err != nil {
		return "", err
	}
	if target.TargetID == "" {
		return "", errors.New("browser proxy returned an empty target")
	}
	return target.TargetID, nil
}

func isCoursePage(rawURL string) bool {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return false
	}
	return strings.Contains(parsed.Path, "/rwlscx/rwlscx_cxRwlsIndex.html") &&
		strings.EqualFold(parsed.Query().Get("layout"), "default")
}

func (b *browserBridge) findCourseTarget(rawURL string) (string, error) {
	targets, err := b.listTargets()
	if err != nil {
		return "", err
	}
	for _, target := range targets {
		if target.Type == "page" && isCoursePage(target.URL) {
			return target.TargetID, nil
		}
	}
	return b.createTargetAt(rawURL)
}

func (b *browserBridge) eval(targetID, expression string) (string, error) {
	query := url.Values{}
	query.Set("target", targetID)
	request, err := http.NewRequest(
		http.MethodPost,
		b.proxyURL+"/eval?"+query.Encode(),
		strings.NewReader(expression),
	)
	if err != nil {
		return "", err
	}
	request.Header.Set("Content-Type", "text/plain; charset=utf-8")
	response, err := b.client.Do(request)
	if err != nil {
		return "", err
	}
	defer response.Body.Close()
	body, err := io.ReadAll(response.Body)
	if err != nil {
		return "", err
	}
	if response.StatusCode >= http.StatusBadRequest {
		return "", fmt.Errorf("browser eval returned HTTP %d: %s", response.StatusCode, safePreview(body, 300))
	}
	var envelope struct {
		Value string `json:"value"`
		Error string `json:"error"`
	}
	if err := json.Unmarshal(body, &envelope); err != nil {
		return "", err
	}
	if envelope.Error != "" {
		return "", errors.New(envelope.Error)
	}
	if envelope.Value == "" {
		return "", errors.New("browser eval returned no value")
	}
	return envelope.Value, nil
}

func (b *browserBridge) fetch(rawURL, method string, headers map[string]string, body string) (browserFetchResponse, error) {
	targetID, err := b.findTarget()
	if err != nil {
		return browserFetchResponse{}, err
	}
	urlJSON, _ := json.Marshal(rawURL)
	methodJSON, _ := json.Marshal(method)
	headersJSON, _ := json.Marshal(headers)
	bodyOption := ""
	if body != "" {
		bodyJSON, _ := json.Marshal(body)
		bodyOption = ",body:" + string(bodyJSON)
	}
	expression := fmt.Sprintf(`(async()=>{const r=await fetch(%s,{method:%s,credentials:"include",headers:%s%s});const body=await r.text();return JSON.stringify({status:r.status,body:body})})()`, urlJSON, methodJSON, headersJSON, bodyOption)
	value, err := b.eval(targetID, expression)
	if err != nil {
		return browserFetchResponse{}, err
	}
	var result browserFetchResponse
	if err := json.Unmarshal([]byte(value), &result); err != nil {
		return browserFetchResponse{}, err
	}
	return result, nil
}

func newBrowserExporter(endpoints ExporterEndpoints) (*exporter, error) {
	bridge := newBrowserBridge("", &http.Client{Timeout: 180 * time.Second})
	if _, err := bridge.listTargets(); err != nil {
		return nil, err
	}
	exp := newExporterWithEndpoints(endpoints, 180*time.Second)
	exp.browser = bridge
	return exp, nil
}

func (e *exporter) exportCourseFromBrowser(_ ExportRequest, params termParams) (*ExportResult, error) {
	text, statusCode, err := e.fetchCoursePageWithRetry(params, 1)
	if err != nil {
		if cached, cacheErr := loadCachedCourseForTerm(params); cacheErr == nil {
			return cached, nil
		}
		return nil, err
	}
	if statusErr := responseStatusError("course", statusCode); statusErr != nil {
		if statusCode >= http.StatusInternalServerError {
			if cached, cacheErr := loadCachedCourseForTerm(params); cacheErr == nil {
				return cached, nil
			}
		}
		path := writeCourseDiagnosis(params, statusCode, []byte(text))
		if path != "" {
			return nil, fmt.Errorf("%w；诊断文件：%s", statusErr, path)
		}
		return nil, statusErr
	}
	if strings.Contains(text, "统一身份认证") {
		return nil, errors.New("浏览器中的新教务登录会话已失效，请重新登录学校系统")
	}
	if strings.Contains(text, "无功能权限") {
		return nil, errors.New("任务落实查询未开放或当前账号没有权限")
	}
	items, err := extractCourseItems([]byte(text))
	if err != nil {
		if cached, cacheErr := loadCachedCourseForTerm(params); cacheErr == nil {
			return cached, nil
		}
		path := writeCourseDiagnosis(params, statusCode, []byte(text))
		if path != "" {
			return nil, fmt.Errorf("课程接口返回内容不是可解析的 JSON；诊断文件：%s", path)
		}
		return nil, errors.New("课程接口返回内容不是可解析的 JSON")
	}
	total := courseResponseTotal([]byte(text))
	for page := 2; total > len(items) && page <= 20; page++ {
		nextText, nextStatus, pageErr := e.fetchCoursePageWithRetry(params, page)
		if pageErr != nil {
			return nil, pageErr
		}
		if statusErr := responseStatusError("course", nextStatus); statusErr != nil {
			return nil, statusErr
		}
		nextItems, itemsErr := extractCourseItems([]byte(nextText))
		if itemsErr != nil {
			return nil, itemsErr
		}
		if len(nextItems) == 0 {
			break
		}
		items = append(items, nextItems...)
	}
	if len(items) == 0 {
		if cached, cacheErr := loadCachedCourseForTerm(params); cacheErr == nil {
			return cached, nil
		}
		path := writeCourseDiagnosis(params, statusCode, []byte(text))
		suffix := ""
		if path != "" {
			suffix = "；诊断文件：" + path
		}
		return nil, fmt.Errorf("没有拿到课程数据，请确认学年学期是否正确：当前查询 %s-%d 第%s学期，接口参数 xnm=%s xqm=%s%s",
			params.XueNian, mustAtoi(params.XueNian)+1, params.XueQi, params.XueNian, params.Xqm, suffix)
	}
	raw := map[string]any{
		"schemaVersion": CourseSchemaVersion,
		"items":         items,
		"term":          params.XueNian + "-" + strconv.Itoa(mustAtoi(params.XueNian)+1) + "-" + params.XueQi,
		"source":        "task-course-browser",
		"version":       1,
	}
	textBytes, _ := json.MarshalIndent(raw, "", "  ")
	outputPath, err := EnsureOutputFilePath("course.json")
	if err != nil {
		return nil, err
	}
	if err := WriteCourseFile(outputPath, textBytes); err != nil {
		return nil, err
	}
	return &ExportResult{Count: len(items), CourseName: InferCourseName(items), CourseSource: "browser", FileName: "course.json", OutputPath: outputPath}, nil
}

// fetchCoursePageWithRetry fetches one course-query page and retries transient
// empty or invalid responses up to three times before giving up.
func (e *exporter) fetchCoursePageWithRetry(params termParams, page int) (string, int, error) {
	var lastErr error
	lastStatus := 0
	for attempt := 0; attempt < 3; attempt++ {
		response, err := e.browser.fetchCourseQuery(e.endpoints.Course, params, page)
		if err == nil {
			lastStatus = response.Status
			if statusErr := responseStatusError("course", response.Status); statusErr != nil {
				lastErr = statusErr
			} else if _, parseErr := extractCourseItems([]byte(response.Body)); parseErr != nil {
				lastErr = parseErr
			} else {
				return response.Body, response.Status, nil
			}
		} else {
			lastErr = err
		}
		if attempt < 2 {
			time.Sleep(time.Second)
		}
	}
	return "", lastStatus, lastErr
}
func (b *browserBridge) waitForCoursePage(targetID string) error {
	deadline := time.Now().Add(30 * time.Second)
	for time.Now().Before(deadline) {
		expression := `(()=>JSON.stringify({ready:document.readyState,hasGetParam:typeof getParam==='function',href:location.href}))()`
		value, err := b.eval(targetID, expression)
		if err == nil {
			var state struct {
				Ready       string `json:"ready"`
				HasGetParam bool   `json:"hasGetParam"`
				Href        string `json:"href"`
			}
			if json.Unmarshal([]byte(value), &state) == nil &&
				state.Ready == "complete" && state.HasGetParam &&
				strings.Contains(state.Href, "rwlscx") {
				return nil
			}
		}
		time.Sleep(500 * time.Millisecond)
	}
	return errors.New("浏览器课程查询页加载超时")
}
func (b *browserBridge) fetchCourseQuery(rawURL string, params termParams, page int) (browserFetchResponse, error) {
	targetID, err := b.findCourseTarget(rawURL)
	if err != nil {
		return browserFetchResponse{}, err
	}
	if err := b.waitForCoursePage(targetID); err != nil {
		return browserFetchResponse{}, err
	}
	requestID := fmt.Sprintf("hdu-course-%d-%d", time.Now().UnixNano(), page)
	if _, err := b.eval(targetID, courseQueryStartExpression(rawURL, params, page, requestID)); err != nil {
		return browserFetchResponse{}, err
	}
	requestIDJSON, _ := json.Marshal(requestID)
	for attempt := 0; attempt < 360; attempt++ {
		value, pollErr := b.eval(targetID, courseQueryPollExpression(string(requestIDJSON)))
		if pollErr != nil {
			return browserFetchResponse{}, pollErr
		}
		var state struct {
			State  string `json:"state"`
			Status int    `json:"status"`
			Body   string `json:"body"`
			Error  string `json:"error"`
		}
		if err := json.Unmarshal([]byte(value), &state); err != nil {
			return browserFetchResponse{}, err
		}
		switch state.State {
		case "done":
			return browserFetchResponse{Status: state.Status, Body: state.Body}, nil
		case "error":
			if state.Error == "" {
				state.Error = "browser course request failed"
			}
			return browserFetchResponse{}, errors.New(state.Error)
		}
		time.Sleep(500 * time.Millisecond)
	}
	return browserFetchResponse{}, errors.New("学校课程接口响应超时，请稍后重试")
}

func courseQueryStartExpression(rawURL string, params termParams, page int, requestID string) string {
	queryURL := courseQueryURL(rawURL)
	urlJSON, _ := json.Marshal(queryURL)
	requestIDJSON, _ := json.Marshal(requestID)
	xueNianJSON, _ := json.Marshal(params.XueNian)
	xqmJSON, _ := json.Marshal(params.Xqm)
	xnmcJSON, _ := json.Marshal(params.XueNian + "-" + strconv.Itoa(mustAtoi(params.XueNian)+1))
	xqmcJSON, _ := json.Marshal(params.XueQi)
	return fmt.Sprintf(`(()=>{const id=%s;const query=Object.assign({},getParam(),{xnm:%s,xqm:%s,xnmc:%s,xqmc:%s,_search:false,nd:Date.now(),"queryModel.showCount":500,"queryModel.currentPage":%d,"queryModel.sortName":" ","queryModel.sortOrder":"asc",time:1});const results=window.__hduCourseQueryResults||(window.__hduCourseQueryResults={});const entry={state:"pending"};results[id]=entry;fetch(%s,{method:"POST",credentials:"include",headers:{"Accept":"application/json, text/javascript, */*; q=0.01","Content-Type":"application/x-www-form-urlencoded; charset=UTF-8","X-Requested-With":"XMLHttpRequest"},body:new URLSearchParams(query).toString()}).then(async r=>{entry.state="done";entry.status=r.status;entry.body=await r.text()}).catch(error=>{entry.state="error";entry.error=String(error)});return id})()`,
		string(requestIDJSON), string(xueNianJSON), string(xqmJSON), string(xnmcJSON), string(xqmcJSON), page, string(urlJSON))
}

func courseQueryPollExpression(requestIDJSON string) string {
	return fmt.Sprintf(`JSON.stringify((window.__hduCourseQueryResults||{})[%s]||{state:"pending"})`, requestIDJSON)
}

func courseQueryExpression(rawURL string, params termParams, page int) string {
	requestID := fmt.Sprintf("hdu-course-test-%d-%d", time.Now().UnixNano(), page)
	return courseQueryStartExpression(rawURL, params, page, requestID)
}

func courseQueryURL(rawURL string) string {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return rawURL
	}
	query := parsed.Query()
	query.Del("layout")
	query.Set("doType", "query")
	query.Set("gnmkdm", "N1548")
	parsed.RawQuery = query.Encode()
	return parsed.String()
}

func courseResponseTotal(body []byte) int {
	var raw any
	if json.Unmarshal(body, &raw) != nil {
		return 0
	}
	return findCourseTotal(raw)
}

func findCourseTotal(value any) int {
	switch typed := value.(type) {
	case map[string]any:
		for _, key := range []string{"total", "records", "totalCount", "totalResult", "totalresult", "totalResults", "totalresults"} {
			if number := courseTotalNumber(typed[key]); number > 0 {
				return number
			}
		}
		for _, nested := range typed {
			if total := findCourseTotal(nested); total > 0 {
				return total
			}
		}
	case []any:
		for _, nested := range typed {
			if total := findCourseTotal(nested); total > 0 {
				return total
			}
		}
	}
	return 0
}

func courseTotalNumber(value any) int {
	switch typed := value.(type) {
	case float64:
		return int(typed)
	case float32:
		return int(typed)
	case int:
		return typed
	case int64:
		return int(typed)
	case json.Number:
		number, _ := strconv.Atoi(string(typed))
		return number
	case string:
		number, _ := strconv.Atoi(strings.TrimSpace(typed))
		return number
	default:
		return 0
	}
}

func loadCachedCourseForTerm(params termParams) (*ExportResult, error) {
	outputPath, err := OutputFilePath("course.json")
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(outputPath)
	if err != nil {
		return nil, err
	}
	items, err := extractCourseItems(data)
	if err != nil || len(items) == 0 {
		if err == nil {
			err = errors.New("cached course file is empty")
		}
		return nil, err
	}
	term := params.XueNian + "-" + strconv.Itoa(mustAtoi(params.XueNian)+1) + "-" + params.XueQi
	matched := false
	var raw map[string]any
	if json.Unmarshal(data, &raw) == nil {
		if rootTerm, ok := raw["term"].(string); ok && strings.Contains(rootTerm, term) {
			matched = true
		}
	}
	if !matched {
		for _, item := range items {
			if strings.Contains(textAny(item["jxbmc"]), term) || strings.Contains(textAny(item["jxb_id"]), term) {
				matched = true
				break
			}
		}
	}
	if !matched {
		return nil, fmt.Errorf("cached course file does not match term %s", term)
	}
	return &ExportResult{
		Count:        len(items),
		CourseName:   InferCourseName(items),
		CourseSource: "local-cache",
		FileName:     "course.json",
		OutputPath:   outputPath,
	}, nil
}

func (e *exporter) exportPersonalScheduleFromBrowser(_ ExportRequest, params termParams) (*ExportResult, error) {
	apiURL := withTermQuery(e.endpoints.PersonalSchedule, params)
	response, err := e.browser.fetch(
		apiURL,
		http.MethodGet,
		map[string]string{"Accept": "application/json, text/javascript, */*; q=0.01", "X-Requested-With": "XMLHttpRequest"},
		"",
	)
	if err != nil {
		return nil, err
	}
	if statusErr := responseStatusError("personal schedule", response.Status); statusErr != nil {
		return nil, statusErr
	}
	if strings.Contains(response.Body, "统一身份认证") {
		return nil, errors.New("浏览器中的新教务登录会话已失效，请重新登录学校系统")
	}
	raw, items, err := decodePersonalScheduleBody([]byte(response.Body))
	if err != nil {
		return nil, err
	}
	payload := map[string]any{
		"schemaVersion": CourseSchemaVersion,
		"source":        "personal-schedule-browser",
		"term":          params.XueNian + "-" + strconv.Itoa(mustAtoi(params.XueNian)+1) + "-" + params.XueQi,
		"exportedAt":    time.Now().Format(time.RFC3339),
		"items":         items,
		"raw":           raw,
	}
	textBytes, _ := json.MarshalIndent(payload, "", "  ")
	outputPath, err := EnsureOutputFilePath("personal-schedule.json")
	if err != nil {
		return nil, err
	}
	if err := writeFileAtomic(outputPath, textBytes, 0644); err != nil {
		return nil, err
	}
	return &ExportResult{PersonalCount: len(items), PersonalFileName: "personal-schedule.json", PersonalOutputPath: outputPath, PersonalExported: true}, nil
}

const browserLoginTimeout = 120 * time.Second

type browserLoginState struct {
	Href         string `json:"href"`
	Ready        string `json:"ready"`
	HasYhm       bool   `json:"hasYhm"`
	HasMm        bool   `json:"hasMm"`
	HasDl        bool   `json:"hasDl"`
	HasCasUser   bool   `json:"hasCasUser"`
	HasCasPass   bool   `json:"hasCasPass"`
	HasCasSubmit bool   `json:"hasCasSubmit"`
	Authed       bool   `json:"authed"`
}

func (b *browserBridge) loginState(targetID string) (browserLoginState, error) {
	expression := `(()=>{const href=location.href;const hasYhm=!!document.querySelector('#yhm');const hasMm=!!document.querySelector('#mm');const hasDl=!!document.querySelector('#dl');const hasCasUser=!!(document.querySelector('#username')||document.querySelector('input[name="username"]'));const hasCasPass=!!(document.querySelector('#password')||document.querySelector('input[name="password"]'));const hasCasSubmit=!!(document.querySelector('#login-submit')||document.querySelector('button[type="submit"]')||document.querySelector('.login-button')||document.querySelector('#dl'));const onLogin=href.indexOf('login')>=0||href.indexOf('sso.')>=0||hasYhm||hasCasUser;return JSON.stringify({href:href,ready:document.readyState,hasYhm:hasYhm,hasMm:hasMm,hasDl:hasDl,hasCasUser:hasCasUser,hasCasPass:hasCasPass,hasCasSubmit:hasCasSubmit,authed:!onLogin})})()`
	value, err := b.eval(targetID, expression)
	if err != nil {
		return browserLoginState{}, err
	}
	var state browserLoginState
	if err := json.Unmarshal([]byte(value), &state); err != nil {
		return browserLoginState{}, fmt.Errorf("解析浏览器登录状态失败：%w", err)
	}
	return state, nil
}

func (b *browserBridge) submitNewJWLogin(targetID, username, password string) error {
	userJSON, _ := json.Marshal(username)
	passJSON, _ := json.Marshal(password)
	expression := fmt.Sprintf(`(()=>{const u=document.querySelector('#yhm');const p=document.querySelector('#mm');const b=document.querySelector('#dl');if(!u||!p||!b)return 'missing';u.value=%s;p.focus();p.value=%s;u.dispatchEvent(new Event('input',{bubbles:true}));p.dispatchEvent(new Event('input',{bubbles:true}));b.click();return 'submitted'})()`, string(userJSON), string(passJSON))
	value, err := b.eval(targetID, expression)
	if err != nil {
		return fmt.Errorf("自动填写新教务登录表单失败：%w", err)
	}
	if value != "submitted" {
		return fmt.Errorf("自动填写新教务登录表单失败：页面缺少输入框或登录按钮（返回 %q）", value)
	}
	return nil
}

func (b *browserBridge) submitCASLogin(targetID, username, password string) error {
	userJSON, _ := json.Marshal(username)
	passJSON, _ := json.Marshal(password)
	expression := fmt.Sprintf(`(()=>{const u=document.querySelector('#username')||document.querySelector('input[name="username"]');const p=document.querySelector('#password')||document.querySelector('input[name="password"]');const b=document.querySelector('#login-submit')||document.querySelector('button[type="submit"]')||document.querySelector('.login-button')||document.querySelector('#dl');if(!u||!p||!b)return 'missing';u.value=%s;p.value=%s;u.dispatchEvent(new Event('input',{bubbles:true}));p.dispatchEvent(new Event('input',{bubbles:true}));b.click();return 'submitted'})()`, string(userJSON), string(passJSON))
	value, err := b.eval(targetID, expression)
	if err != nil {
		return fmt.Errorf("自动填写统一认证登录表单失败：%w", err)
	}
	if value != "submitted" {
		return fmt.Errorf("自动填写统一认证登录表单失败：页面缺少输入框或登录按钮（返回 %q）", value)
	}
	return nil
}

func (b *browserBridge) ensureAuthenticated(loginURL, username, password string) error {
	targetID, err := b.createTargetAt(loginURL)
	if err != nil {
		return err
	}
	deadline := time.Now().Add(browserLoginTimeout)
	newJWSubmitted := false
	casSubmitted := false
	for time.Now().Before(deadline) {
		state, stateErr := b.loginState(targetID)
		if stateErr != nil {
			if time.Now().Before(deadline.Add(-2 * time.Second)) {
				time.Sleep(500 * time.Millisecond)
				continue
			}
			return stateErr
		}
		if state.Authed {
			return nil
		}
		if state.Ready != "complete" {
			time.Sleep(500 * time.Millisecond)
			continue
		}
		if !newJWSubmitted && state.HasYhm && state.HasMm && state.HasDl {
			if err := b.submitNewJWLogin(targetID, username, password); err != nil {
				return err
			}
			newJWSubmitted = true
			continue
		}
		if !casSubmitted && state.HasCasUser && state.HasCasPass && state.HasCasSubmit {
			if err := b.submitCASLogin(targetID, username, password); err != nil {
				return err
			}
			casSubmitted = true
			continue
		}
		time.Sleep(1000 * time.Millisecond)
	}
	state, _ := b.loginState(targetID)
	if state.Href != "" {
		return fmt.Errorf("浏览器自动登录超时，当前页面：%s", state.Href)
	}
	return errors.New("浏览器自动登录超时")
}

func (e *exporter) loginViaBrowser(username, password string) error {
	if e.browser == nil {
		return errors.New("浏览器桥未初始化")
	}
	if strings.TrimSpace(username) == "" || strings.TrimSpace(password) == "" {
		return errors.New("浏览器自动登录需要账号和密码")
	}
	return e.browser.ensureAuthenticated(e.endpoints.NewJWLogin, username, password)
}
