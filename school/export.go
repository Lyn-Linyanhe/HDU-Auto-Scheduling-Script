package school

import (
	"bytes"
	"context"
	"crypto/aes"
	"crypto/rand"
	"crypto/rsa"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/big"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"
)

const courseURL = "https://newjw.hdu.edu.cn/jwglxt/rwlscx/rwlscx_cxRwlsIndex.html?gnmkdm=N1548&layout=default"
const personalScheduleURL = "https://newjw.hdu.edu.cn/jwglxt/kbcx/xskbcx_cxXsgrkb.html?gnmkdm=N2151"
const userAgent = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/143.0.0.0 Safari/537.36 Edg/143.0.0.0"

// ExporterEndpoints names the external pages required for a course export.
// Production uses DefaultExporterEndpoints. Test overrides are accepted only
// through RunExportWithTestEndpoints after a loopback-only safety check.
type ExporterEndpoints struct {
	CASLogin         string
	CASService       string
	NewJWLogin       string
	PublicKey        string
	Course           string
	PersonalSchedule string
}

func DefaultExporterEndpoints() ExporterEndpoints {
	return ExporterEndpoints{
		CASLogin:         "https://sso.hdu.edu.cn/login",
		CASService:       "https://newjw.hdu.edu.cn/sso/driot4login",
		NewJWLogin:       "https://newjw.hdu.edu.cn/jwglxt/xtgl/login_slogin.html",
		PublicKey:        "https://newjw.hdu.edu.cn/jwglxt/xtgl/login_getPublicKey.html",
		Course:           courseURL,
		PersonalSchedule: personalScheduleURL,
	}
}

type ExportResult struct {
	Count               int    `json:"count"`
	CourseName          string `json:"courseName"`
	CourseSource        string `json:"courseSource,omitempty"`
	FileName            string `json:"fileName"`
	OutputPath          string `json:"outputPath"`
	PersonalCount       int    `json:"personalCount"`
	PersonalFileName    string `json:"personalFileName,omitempty"`
	PersonalOutputPath  string `json:"personalOutputPath,omitempty"`
	PersonalExported    bool   `json:"personalExported"`
	PersonalExportError string `json:"personalExportError,omitempty"`
}

type courseResponseDiagnosis struct {
	Term        string         `json:"term"`
	XueNian     string         `json:"xueNian"`
	XueQi       string         `json:"xueQi"`
	Xqm         string         `json:"xqm"`
	StatusCode  int            `json:"statusCode"`
	BodyBytes   int            `json:"bodyBytes"`
	TopKeys     []string       `json:"topKeys,omitempty"`
	ArrayCounts map[string]int `json:"arrayCounts,omitempty"`
	ShapeDrift  string         `json:"shapeDrift,omitempty"`
	Preview     string         `json:"preview,omitempty"`
	SavedAt     string         `json:"savedAt"`
}

type exporter struct {
	client    *http.Client
	endpoints ExporterEndpoints
	browser   *browserBridge
}

type publicKeyPayload struct {
	Modulus  string `json:"modulus"`
	Exponent string `json:"exponent"`
}

type termParams struct {
	XueNian string
	XueQi   string
	Xqm     string
}

func newExporter() *exporter {
	return newExporterWithEndpoints(DefaultExporterEndpoints(), 90*time.Second)
}

func newExporterWithEndpoints(endpoints ExporterEndpoints, timeout time.Duration) *exporter {
	jar, _ := cookiejar.New(nil)
	if timeout <= 0 {
		timeout = 90 * time.Second
	}
	return &exporter{
		client: &http.Client{
			Jar:     jar,
			Timeout: timeout,
		},
		endpoints: endpoints,
	}
}

// ValidateTestExporterEndpoints guarantees test credentials and cookies can
// only be sent to a local mock system. It intentionally does not resolve DNS:
// only literal loopback hosts and localhost are accepted.
func ValidateTestExporterEndpoints(endpoints ExporterEndpoints) error {
	values := map[string]string{
		"CAS login":          endpoints.CASLogin,
		"CAS service":        endpoints.CASService,
		"NewJW login":        endpoints.NewJWLogin,
		"NewJW public key":   endpoints.PublicKey,
		"course query":       endpoints.Course,
		"personal timetable": endpoints.PersonalSchedule,
	}
	for name, value := range values {
		parsed, err := url.ParseRequestURI(strings.TrimSpace(value))
		if err != nil || parsed.Scheme != "http" || parsed.Host == "" || parsed.User != nil || parsed.Fragment != "" {
			return fmt.Errorf("test %s endpoint must be an absolute loopback HTTP URL", name)
		}
		host := strings.ToLower(parsed.Hostname())
		if host != "127.0.0.1" && host != "localhost" && host != "::1" {
			return fmt.Errorf("test %s endpoint must use a loopback host, got %q", name, parsed.Hostname())
		}
	}
	return nil
}

func (e *exporter) login(method, username, password string) error {
	switch strings.ToLower(strings.TrimSpace(method)) {
	case "qr":
		return e.loginCAS("qr", username, password)
	case "password", "":
		return e.loginPassword(username, password)
	default:
		return e.loginPassword(username, password)
	}
}

func (e *exporter) loginPassword(username, password string) error {
	directErr := e.loginNewJW(username, password)
	if directErr == nil {
		return nil
	}
	casErr := e.loginCAS("password", username, password)
	if casErr == nil {
		return nil
	}
	return fmt.Errorf("新教务直登失败：%v；CAS 回退失败：%w", directErr, casErr)
}

func (e *exporter) loginCAS(mode, username, password string) error {
	execution, croypto, err := e.getCASLoginConfig()
	if err != nil {
		return err
	}
	if mode == "qr" {
		return errors.New("扫码登录暂未实现，请先使用账号密码登录")
	}

	encryptedPassword, err := aesEncrypt(croypto, password)
	if err != nil {
		return err
	}

	form := url.Values{}
	form.Set("username", username)
	form.Set("type", "UsernamePassword")
	form.Set("_eventId", "submit")
	form.Set("geolocation", "")
	form.Set("execution", execution)
	form.Set("captcha_code", "")
	form.Set("croypto", croypto)
	form.Set("password", encryptedPassword)

	loginURL, err := e.casLoginURL()
	if err != nil {
		return err
	}
	req, err := http.NewRequest(http.MethodPost, loginURL, strings.NewReader(form.Encode()))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	setNewJWPageHeaders(req, loginURL)
	setOriginHeader(req, loginURL)

	resp, err := e.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}

	text := string(body)
	if containsAny(text, "用户名或密码错误", "用户名称或密码错误", "账号或密码错误") {
		return errors.New("CAS 登录提示账号或密码不正确")
	}
	if strings.Contains(text, "统一身份认证") && !strings.Contains(text, "service=") {
		return errors.New("CAS 登录未完成统一身份认证")
	}

	return validateCASLoginResponse(resp, body, e.endpoints.CASLogin)
}

func (e *exporter) getCASLoginConfig() (execution, croypto string, err error) {
	loginURL, err := e.casLoginURL()
	if err != nil {
		return "", "", err
	}
	pageReq, err := http.NewRequest(http.MethodGet, loginURL, nil)
	if err != nil {
		return "", "", err
	}
	setNewJWPageHeaders(pageReq, loginURL)
	resp, err := e.client.Do(pageReq)
	if err != nil {
		return "", "", err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", "", err
	}
	text := string(body)

	execution = firstNonEmpty(
		extractBetween(text, `id="login-page-flowkey">`, `<`),
		extractValue(text, "login-page-flowkey"),
		extractValue(text, "execution"),
	)
	croypto = firstNonEmpty(
		extractBetween(text, `id="login-croypto">`, `<`),
		extractValue(text, "login-croypto"),
		extractValue(text, "croypto"),
	)
	if execution == "" || croypto == "" {
		return "", "", errors.New("未获取到 CAS 登录配置，学校登录页可能已调整")
	}
	return execution, croypto, nil
}

func (e *exporter) casLoginURL() (string, error) {
	loginURL, err := url.Parse(e.endpoints.CASLogin)
	if err != nil {
		return "", err
	}
	query := loginURL.Query()
	query.Set("service", e.endpoints.CASService)
	loginURL.RawQuery = query.Encode()
	return loginURL.String(), nil
}

func (e *exporter) finishCASLogin() error {
	loginURL, err := e.casLoginURL()
	if err != nil {
		return err
	}
	parsedLoginURL, err := url.Parse(loginURL)
	if err != nil {
		return err
	}
	resp, err := e.client.Get(loginURL)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	if statusErr := responseStatusError("CAS", resp.StatusCode); statusErr != nil {
		return statusErr
	}
	if resp.Request != nil && resp.Request.URL != nil {
		finalURL := resp.Request.URL
		if strings.EqualFold(finalURL.Hostname(), parsedLoginURL.Hostname()) &&
			strings.TrimRight(finalURL.Path, "/") == strings.TrimRight(parsedLoginURL.Path, "/") {
			return errors.New("CAS 登录未完成，服务器仍返回统一身份认证登录页，请确认账号密码")
		}
	}
	bodyText := string(body)
	if strings.Contains(bodyText, `name="login-page-flowkey"`) ||
		(strings.Contains(bodyText, `name="username"`) && strings.Contains(bodyText, `name="password"`)) {
		return errors.New("CAS 登录未完成，服务器仍返回统一身份认证登录页，请确认账号密码")
	}
	return nil
}

func validateCASLoginResponse(resp *http.Response, body []byte, loginURL string) error {
	if statusErr := responseStatusError("CAS", resp.StatusCode); statusErr != nil {
		return statusErr
	}
	parsedLoginURL, err := url.Parse(loginURL)
	if err != nil {
		return err
	}
	if resp.Request != nil && resp.Request.URL != nil {
		finalURL := resp.Request.URL
		if strings.EqualFold(finalURL.Hostname(), parsedLoginURL.Hostname()) &&
			strings.TrimRight(finalURL.Path, "/") == strings.TrimRight(parsedLoginURL.Path, "/") {
			return errors.New("CAS 登录未完成，服务器仍返回统一身份认证登录页，请确认账号密码")
		}
	}
	bodyText := string(body)
	if strings.Contains(bodyText, `name="login-page-flowkey"`) ||
		(strings.Contains(bodyText, `name="username"`) && strings.Contains(bodyText, `name="password"`)) {
		return errors.New("CAS 登录未完成，服务器仍返回统一身份认证登录页，请确认账号密码")
	}
	return nil
}

func (e *exporter) loginNewJW(username, password string) error {
	loginPageReq, err := http.NewRequest(http.MethodGet, e.endpoints.NewJWLogin, nil)
	if err != nil {
		return err
	}
	setNewJWPageHeaders(loginPageReq, e.endpoints.NewJWLogin)
	resp, err := e.client.Do(loginPageReq)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}

	text := string(body)
	csrftoken := firstNonEmpty(
		extractValue(text, "csrftoken"),
		extractBetween(text, `name="csrftoken" value="`, `"`),
	)
	if csrftoken == "" {
		return errors.New("未获取到新教务登录令牌，学校登录页可能已调整")
	}

	key, err := e.getPublicKey()
	if err != nil {
		return err
	}
	encryptedPassword, err := rsaEncrypt(key, password)
	if err != nil {
		return err
	}

	form := url.Values{}
	form.Set("csrftoken", csrftoken)
	form.Set("yhm", username)
	form.Set("mm", encryptedPassword)

	req, err := http.NewRequest(http.MethodPost, e.endpoints.NewJWLogin, strings.NewReader(form.Encode()))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	setNewJWPageHeaders(req, e.endpoints.NewJWLogin)
	setOriginHeader(req, e.endpoints.NewJWLogin)

	resp, err = e.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	out, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	result := string(out)
	if containsAny(result, "用户名或密码错误", "用户名称或密码错误", "账号或密码错误") {
		return errors.New("新教务登录提示账号或密码不正确")
	}
	if strings.Contains(result, "统一身份认证") {
		return errors.New("新教务登录已重定向到统一身份认证")
	}
	return e.warmupNewJW()
}

func (e *exporter) getPublicKey() (string, error) {
	publicKeyURL, err := url.Parse(e.endpoints.PublicKey)
	if err != nil {
		return "", err
	}
	query := publicKeyURL.Query()
	query.Set("time", strconv.FormatInt(time.Now().UnixMilli(), 10))
	publicKeyURL.RawQuery = query.Encode()
	keyReq, err := http.NewRequest(http.MethodGet, publicKeyURL.String(), nil)
	if err != nil {
		return "", err
	}
	setNewJWAjaxHeaders(keyReq, e.endpoints.NewJWLogin)
	resp, err := e.client.Do(keyReq)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	if statusErr := responseStatusError("newjw public key", resp.StatusCode); statusErr != nil {
		return "", statusErr
	}
	var payload publicKeyPayload
	if err := json.Unmarshal(body, &payload); err != nil {
		return "", err
	}
	if strings.TrimSpace(payload.Modulus) == "" {
		return "", errors.New("未获取到新教务登录公钥")
	}
	return payload.Modulus, nil
}

func (e *exporter) warmupNewJW() error {
	params := termFromRequest(ExportRequest{})
	apiURL := withTermQuery(e.endpoints.PersonalSchedule, params)
	req, err := http.NewRequest(http.MethodGet, apiURL, nil)
	if err != nil {
		return err
	}
	setNewJWPageHeaders(req, e.endpoints.PersonalSchedule)
	resp, err := e.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	if statusErr := responseStatusError("personal schedule", resp.StatusCode); statusErr != nil {
		return statusErr
	}
	if strings.Contains(string(body), "统一身份认证") {
		return errors.New("登录后会话初始化失败，请重新登录")
	}
	return nil
}

func rsaEncrypt(publicKeyB64, data string) (string, error) {
	pubKey, err := base64.StdEncoding.DecodeString(publicKeyB64)
	if err != nil {
		return "", err
	}
	pub := &rsa.PublicKey{N: new(big.Int), E: 65537}
	pub.N.SetString(hexString(pubKey), 16)
	cipherText, err := rsa.EncryptPKCS1v15(rand.Reader, pub, []byte(data))
	if err != nil {
		return "", err
	}
	return base64.StdEncoding.EncodeToString(cipherText), nil
}

func hexString(data []byte) string {
	const hex = "0123456789abcdef"
	var buf bytes.Buffer
	buf.Grow(len(data) * 2)
	for _, b := range data {
		buf.WriteByte(hex[b>>4])
		buf.WriteByte(hex[b&0x0f])
	}
	return buf.String()
}

func aesEncrypt(key, plainText string) (string, error) {
	keyBytes, err := base64.StdEncoding.DecodeString(key)
	if err != nil {
		return "", err
	}
	block, err := aes.NewCipher(keyBytes)
	if err != nil {
		return "", err
	}
	padded := pkcs7Padding([]byte(plainText), block.BlockSize())
	cipherText := make([]byte, len(padded))
	for bs, be := 0, block.BlockSize(); bs < len(padded); bs, be = bs+block.BlockSize(), be+block.BlockSize() {
		block.Encrypt(cipherText[bs:be], padded[bs:be])
	}
	return base64.StdEncoding.EncodeToString(cipherText), nil
}

func pkcs7Padding(data []byte, blockSize int) []byte {
	padLen := blockSize - len(data)%blockSize
	padding := bytes.Repeat([]byte{byte(padLen)}, padLen)
	return append(data, padding...)
}

func extractValue(text, id string) string {
	patterns := []string{
		`id="` + regexp.QuoteMeta(id) + `"[^>]*value="([^"]+)"`,
		`name="` + regexp.QuoteMeta(id) + `"[^>]*value="([^"]+)"`,
	}
	for _, pattern := range patterns {
		re := regexp.MustCompile(pattern)
		match := re.FindStringSubmatch(text)
		if len(match) >= 2 {
			return strings.TrimSpace(match[1])
		}
	}
	return ""
}

func extractBetween(text, left, right string) string {
	start := strings.Index(text, left)
	if start < 0 {
		return ""
	}
	start += len(left)
	end := strings.Index(text[start:], right)
	if end < 0 {
		return strings.TrimSpace(text[start:])
	}
	return strings.TrimSpace(text[start : start+end])
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if strings.TrimSpace(v) != "" {
			return strings.TrimSpace(v)
		}
	}
	return ""
}

func containsAny(text string, values ...string) bool {
	for _, v := range values {
		if strings.Contains(text, v) {
			return true
		}
	}
	return false
}

func responseStatusError(endpoint string, statusCode int) error {
	if statusCode < http.StatusBadRequest {
		return nil
	}
	statusText := http.StatusText(statusCode)
	if statusText == "" {
		statusText = "unknown status"
	}
	if statusCode == http.StatusUnauthorized || statusCode == http.StatusForbidden {
		return &httpStatusError{Endpoint: endpoint, StatusCode: statusCode, StatusText: statusText}
	}
	return fmt.Errorf("学校%s接口返回 HTTP %d %s", endpoint, statusCode, statusText)
}

type httpStatusError struct {
	Endpoint   string
	StatusCode int
	StatusText string
}

func (e *httpStatusError) Error() string {
	switch e.Endpoint {
	case "course":
		return fmt.Sprintf("学校课程接口拒绝访问（HTTP %d %s），可能是登录会话失效、接口权限或学校系统拦截，不是学期参数问题", e.StatusCode, e.StatusText)
	case "personal schedule":
		return fmt.Sprintf("学校个人课表接口拒绝访问（HTTP %d %s），可能是登录会话失效、接口权限或学校系统拦截；浏览器手动登录的 Cookie 不会自动共享给导出器，请在导出页重新输入当前密码", e.StatusCode, e.StatusText)
	default:
		return fmt.Sprintf("学校%s接口拒绝访问（HTTP %d %s），可能是登录会话失效、接口权限或学校系统拦截", e.Endpoint, e.StatusCode, e.StatusText)
	}
}

func isSessionInvalidError(err error) bool {
	if err == nil {
		return false
	}
	var statusErr *httpStatusError
	if errors.As(err, &statusErr) && (statusErr.StatusCode == http.StatusUnauthorized || statusErr.StatusCode == http.StatusForbidden) {
		return true
	}
	message := err.Error()
	return strings.Contains(message, "登录已失效") || strings.Contains(message, "登录会话失效") || strings.Contains(message, "登录会话已失效")
}

func (e *exporter) exportCourse(req ExportRequest) (*ExportResult, error) {
	params := termFromRequest(req)
	if e.browser != nil {
		return e.exportCourseFromBrowser(req, params)
	}
	_ = e.warmupCourseQuery()

	form := url.Values{}
	form.Set("xnmc", params.XueNian+"-"+strconv.Itoa(mustAtoi(params.XueNian)+1))
	form.Set("xqmc", params.XueQi)
	form.Set("xnm", params.XueNian)
	form.Set("xqm", params.Xqm)
	form.Set("_search", "false")
	form.Set("nd", strconv.FormatInt(time.Now().Unix(), 10))
	form.Set("queryModel.showCount", "9999")
	form.Set("queryModel.currentPage", "1")
	form.Set("queryModel.sortOrder", "asc")
	form.Set("time", "0")
	form.Set("jxbmc", "")

	reqHTTP, err := http.NewRequest(http.MethodPost, e.endpoints.Course, strings.NewReader(form.Encode()))
	if err != nil {
		return nil, err
	}
	reqHTTP.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	setNewJWAjaxHeaders(reqHTTP, e.endpoints.Course)

	resp, err := e.client.Do(reqHTTP)
	if err != nil {
		if errors.Is(err, context.DeadlineExceeded) || strings.Contains(err.Error(), "Client.Timeout") {
			return nil, errors.New("课程接口响应超时，可能是学校系统较慢，请稍后重试")
		}
		return nil, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if statusErr := responseStatusError("course", resp.StatusCode); statusErr != nil {
		path := writeCourseDiagnosis(params, resp.StatusCode, body, "")
		if path != "" {
			return nil, fmt.Errorf("%w；诊断文件：%s", statusErr, path)
		}
		return nil, statusErr
	}

	text := string(body)
	if strings.Contains(text, "统一身份认证") {
		return nil, errors.New("登录已失效，请重新登录")
	}
	if strings.Contains(text, "无功能权限") {
		return nil, errors.New("任务落实查询未开放或当前账号没有权限")
	}

	items, err := extractCourseItems(body)
	if err != nil {
		path := writeCourseDiagnosis(params, resp.StatusCode, body, "")
		if path != "" {
			return nil, fmt.Errorf("课程接口返回内容不是可解析的 JSON，可能仍停留在登录页；诊断文件：%s", path)
		}
		return nil, errors.New("课程接口返回内容不是可解析的 JSON，可能仍停留在登录页")
	}
	if len(items) == 0 {
		path := writeCourseDiagnosis(params, resp.StatusCode, body, "")
		suffix := ""
		if path != "" {
			suffix = "；诊断文件：" + path
		}
		return nil, fmt.Errorf("没有拿到课程数据，请确认学年学期是否正确：当前查询 %s-%d 第%s学期，接口参数 xnm=%s xqm=%s%s",
			params.XueNian,
			mustAtoi(params.XueNian)+1,
			params.XueQi,
			params.XueNian,
			params.Xqm,
			suffix,
		)
	}
	if drift, drifted := courseShapeDrift(items); drifted {
		path := writeCourseDiagnosis(params, resp.StatusCode, body, drift)
		suffix := ""
		if path != "" {
			suffix = "；诊断文件：" + path
		}
		return nil, fmt.Errorf("课程接口返回内容疑似改版（关键字段缺失）：%s%s", drift, suffix)
	}

	raw := map[string]any{
		"schemaVersion": CourseSchemaVersion,
		"items":         items,
		"term":          params.XueNian + "-" + strconv.Itoa(mustAtoi(params.XueNian)+1) + "-" + params.XueQi,
		"source":        "task-course",
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

	return &ExportResult{
		Count:        len(items),
		CourseName:   InferCourseName(items),
		CourseSource: "school",
		FileName:     "course.json",
		OutputPath:   outputPath,
	}, nil
}

func (e *exporter) warmupCourseQuery() error {
	req, err := http.NewRequest(http.MethodGet, e.endpoints.Course, nil)
	if err != nil {
		return err
	}
	setNewJWPageHeaders(req, e.endpoints.Course)
	resp, err := e.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	_, _ = io.Copy(io.Discard, resp.Body)
	return nil
}

func setNewJWPageHeaders(req *http.Request, referer string) {
	req.Header.Set("User-Agent", userAgent)
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8")
	req.Header.Set("Accept-Language", "zh-CN,zh;q=0.9")
	if strings.TrimSpace(referer) != "" {
		req.Header.Set("Referer", referer)
	}
}

func setOriginHeader(req *http.Request, rawURL string) {
	parsed, err := url.Parse(rawURL)
	if err == nil && parsed.Scheme != "" && parsed.Host != "" {
		req.Header.Set("Origin", parsed.Scheme+"://"+parsed.Host)
	}
}

func setNewJWAjaxHeaders(req *http.Request, referer string) {
	req.Header.Set("User-Agent", userAgent)
	req.Header.Set("Accept", "application/json, text/javascript, */*; q=0.01")
	req.Header.Set("Accept-Language", "zh-CN,zh;q=0.9")
	req.Header.Set("Origin", "https://newjw.hdu.edu.cn")
	req.Header.Set("X-Requested-With", "XMLHttpRequest")
	if strings.TrimSpace(referer) != "" {
		req.Header.Set("Referer", referer)
	}
}

func extractCourseItems(body []byte) ([]map[string]any, error) {
	var raw any
	if err := json.Unmarshal(body, &raw); err != nil {
		return nil, err
	}
	return findCourseItems(raw), nil
}

func findCourseItems(value any) []map[string]any {
	switch typed := value.(type) {
	case []any:
		return mapSlice(typed)
	case []map[string]any:
		return typed
	case map[string]any:
		for _, key := range []string{"items", "rows", "list", "data"} {
			if items := findCourseItems(typed[key]); len(items) > 0 {
				return items
			}
		}
		for _, nested := range typed {
			if items := findCourseItems(nested); len(items) > 0 {
				return items
			}
		}
	}
	return nil
}

func writeCourseDiagnosis(params termParams, statusCode int, body []byte, shapeDrift string) string {
	diagnosis := courseResponseDiagnosis{
		Term:        params.XueNian + "-" + strconv.Itoa(mustAtoi(params.XueNian)+1) + "-" + params.XueQi,
		XueNian:     params.XueNian,
		XueQi:       params.XueQi,
		Xqm:         params.Xqm,
		StatusCode:  statusCode,
		BodyBytes:   len(body),
		ArrayCounts: map[string]int{},
		ShapeDrift:  strings.TrimSpace(shapeDrift),
		Preview:     safePreview(body, 1000),
		SavedAt:     time.Now().Format(time.RFC3339),
	}
	var raw any
	if err := json.Unmarshal(body, &raw); err == nil {
		diagnosis.TopKeys = topLevelKeys(raw)
		diagnosis.ArrayCounts = collectArrayCounts(raw)
	}
	data, err := json.MarshalIndent(diagnosis, "", "  ")
	if err != nil {
		return ""
	}
	path, err := EnsureOutputFilePath("course-export-diagnosis.json")
	if err != nil {
		return ""
	}
	if err := os.WriteFile(path, data, 0644); err != nil {
		return ""
	}
	return path
}

// courseShapeRequiredFields are the fields every task-course row is expected to
// carry. When a large share of extracted rows lack them, the school likely
// changed the response shape and we should fail loudly instead of exporting
// silently broken data.
var courseShapeRequiredFields = []string{"jxbmc", "kcmc"}

// courseShapeDrift reports a human-readable message when fewer than 90% of the
// extracted rows carry all required fields. It returns (message, false) when the
// shape looks intact or there are no rows to judge.
func courseShapeDrift(items []map[string]any) (string, bool) {
	if len(items) == 0 {
		return "", false
	}
	complete := 0
	for _, item := range items {
		ok := true
		for _, field := range courseShapeRequiredFields {
			value, exists := item[field]
			if !exists || strings.TrimSpace(fmt.Sprint(value)) == "" {
				ok = false
				break
			}
		}
		if ok {
			complete++
		}
	}
	if float64(complete)/float64(len(items)) >= 0.9 {
		return "", false
	}
	return fmt.Sprintf("%d/%d 条教学班记录缺少关键字段 %s",
		len(items)-complete, len(items), strings.Join(courseShapeRequiredFields, "/")), true
}

func topLevelKeys(value any) []string {
	mapped, ok := value.(map[string]any)
	if !ok {
		return nil
	}
	keys := make([]string, 0, len(mapped))
	for key := range mapped {
		keys = append(keys, key)
	}
	return keys
}

func collectArrayCounts(value any) map[string]int {
	out := map[string]int{}
	collectArrayCountsInto(out, "$", value)
	return out
}

func collectArrayCountsInto(out map[string]int, path string, value any) {
	switch typed := value.(type) {
	case []any:
		out[path] = len(typed)
	case map[string]any:
		for key, nested := range typed {
			collectArrayCountsInto(out, path+"."+key, nested)
		}
	}
}

var sensitiveValuePattern = regexp.MustCompile(`(?i)(["']?(?:password|passwd|pwd|token|csrf(?:token)?|cookie|authorization|secret|loginpwd|mm)["']?\s*[:=]\s*["']?)([^"'&,\s}\r\n>]+)`)
var sensitiveNameValuePattern = regexp.MustCompile(`(?i)(\bname\s*=\s*["']?(?:password|passwd|pwd|token|csrf(?:token)?|cookie|authorization|secret|loginpwd|mm)["']?[^>]*?\bvalue\s*=\s*["']?)([^"'\s>]+)`)

func safePreview(body []byte, limit int) string {
	text := strings.TrimSpace(string(body))
	text = sensitiveValuePattern.ReplaceAllString(text, "$1***")
	text = sensitiveNameValuePattern.ReplaceAllString(text, "$1***")
	if len(text) <= limit {
		return text
	}
	return text[:limit]
}

func (e *exporter) exportPersonalSchedule(req ExportRequest) (*ExportResult, error) {
	params := termFromRequest(req)
	if e.browser != nil {
		return e.exportPersonalScheduleFromBrowser(req, params)
	}
	apiURL := withTermQuery(e.endpoints.PersonalSchedule, params)
	pageReq, err := http.NewRequest(http.MethodGet, apiURL, nil)
	if err != nil {
		return nil, err
	}
	setNewJWPageHeaders(pageReq, e.endpoints.PersonalSchedule)
	resp, err := e.client.Do(pageReq)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if statusErr := responseStatusError("personal schedule", resp.StatusCode); statusErr != nil {
		return nil, statusErr
	}
	text := string(body)
	if strings.Contains(text, "统一身份认证") {
		return nil, errors.New("个人课表接口提示登录已失效")
	}
	if strings.Contains(text, "无功能权限") {
		return nil, errors.New("当前账号没有个人课表查询权限")
	}

	raw, items, err := decodePersonalScheduleBody(body)
	if err != nil {
		return nil, err
	}
	payload := map[string]any{
		"schemaVersion": CourseSchemaVersion,
		"source":        "personal-schedule",
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

	return &ExportResult{
		PersonalCount:      len(items),
		PersonalFileName:   "personal-schedule.json",
		PersonalOutputPath: outputPath,
		PersonalExported:   true,
	}, nil
}

func decodePersonalScheduleBody(body []byte) (map[string]any, []map[string]any, error) {
	var value any
	if err := json.Unmarshal(body, &value); err != nil {
		return nil, nil, errors.New("个人课表接口返回内容不是可解析的 JSON")
	}
	switch typed := value.(type) {
	case nil:
		return nil, nil, errors.New("个人课表接口返回空 JSON，未更新上一次成功课表")
	case map[string]any:
		items := extractPersonalScheduleItems(typed)
		if items == nil {
			items = []map[string]any{}
		}
		return typed, items, nil
	case []any:
		items := mapSlice(typed)
		if items == nil {
			items = []map[string]any{}
		}
		return map[string]any{"items": items}, normalizePersonalItems(items), nil
	default:
		return nil, nil, errors.New("个人课表接口返回 JSON 顶层类型不受支持")
	}
}

func extractPersonalScheduleItems(raw map[string]any) []map[string]any {
	for _, key := range []string{"kbList", "items", "list", "rows", "data"} {
		if items := mapSlice(raw[key]); len(items) > 0 {
			return normalizePersonalItems(items)
		}
	}
	for _, value := range raw {
		if nested, ok := value.(map[string]any); ok {
			if items := extractPersonalScheduleItems(nested); len(items) > 0 {
				return items
			}
		}
	}
	return nil
}

func normalizePersonalItems(items []map[string]any) []map[string]any {
	out := make([]map[string]any, 0, len(items))
	for index, item := range items {
		next := map[string]any{}
		for key, value := range item {
			next[key] = value
		}
		courseName := textAny(firstExisting(item, "kcmc", "courseName", "name"))
		sectionName := firstNonEmpty(
			textAny(firstExisting(item, "jxbmc", "sectionName", "jxbmc_name")),
			courseName,
			fmt.Sprintf("个人课表课程-%d", index+1),
		)
		next["kcmc"] = firstNonEmpty(courseName, sectionName)
		next["courseName"] = next["kcmc"]
		next["jxbmc"] = sectionName
		next["sectionName"] = sectionName
		next["jzgxx"] = firstNonEmpty(textAny(firstExisting(item, "xm", "jsxm", "jzgxx", "teacher")), textAny(item["jsxx"]))
		next["jxdd"] = firstNonEmpty(textAny(firstExisting(item, "cdmc", "jxdd", "location")), textAny(item["lh"]))
		next["sksj"] = firstNonEmpty(textAny(firstExisting(item, "sksj", "time", "schedule")), scheduleTextFromPersonal(item))
		next["jxb_id"] = firstNonEmpty(textAny(firstExisting(item, "jxb_id", "sectionId", "id")), fmt.Sprintf("personal-%d", index+1))
		next["id"] = next["jxb_id"]
		next["source"] = "personal-schedule"
		out = append(out, next)
	}
	return MergePersonalScheduleItems(out)
}

// MergePersonalScheduleItems merges rows that belong to the same teaching
// class (same jxb_id) so one teaching class is counted once. Weekly sessions
// are combined into a single sksj string, keeping location changes visible.
func MergePersonalScheduleItems(items []map[string]any) []map[string]any {
	result := make([]map[string]any, 0, len(items))
	byID := make(map[string]int, len(items))
	for _, item := range items {
		key := strings.TrimSpace(textAny(firstExisting(item, "jxb_id", "id", "sectionId")))
		if key == "" {
			key = strings.TrimSpace(textAny(item["jxbmc"]))
		}
		if idx, ok := byID[key]; ok {
			existing := result[idx]
			mergePersonalText(existing, item, []string{"sksj", "timeText", "time", "schedule"})
			mergePersonalText(existing, item, []string{"jxdd", "location", "cdlbmc"})
			continue
		}
		byID[key] = len(result)
		result = append(result, item)
	}
	return result
}

func mergePersonalText(existing, next map[string]any, keys []string) {
	for _, key := range keys {
		left := strings.TrimSpace(textAny(existing[key]))
		right := strings.TrimSpace(textAny(next[key]))
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

func scheduleTextFromPersonal(item map[string]any) string {
	day := textAny(firstExisting(item, "xqjmc", "xqj", "day"))
	if day == "" {
		return ""
	}
	dayLabel := day
	if number, err := strconv.Atoi(day); err == nil && number >= 1 && number <= 7 {
		dayLabel = []string{"星期一", "星期二", "星期三", "星期四", "星期五", "星期六", "星期日"}[number-1]
	}
	period := firstNonEmpty(textAny(firstExisting(item, "jc", "jcs", "period")), textAny(item["jcstr"]))
	weeks := firstNonEmpty(textAny(firstExisting(item, "zcd", "zcdxx", "weeks")), "1-17周")
	location := textAny(firstExisting(item, "cdmc", "jxdd", "location"))
	text := strings.TrimSpace(dayLabel + "第" + period + "节{" + weeks + "}")
	if location != "" {
		text += location
	}
	return text
}

func mapSlice(value any) []map[string]any {
	switch typed := value.(type) {
	case []any:
		out := make([]map[string]any, 0, len(typed))
		for _, item := range typed {
			if mapped, ok := item.(map[string]any); ok {
				out = append(out, mapped)
			}
		}
		return out
	case []map[string]any:
		return typed
	default:
		return nil
	}
}

func firstExisting(item map[string]any, keys ...string) any {
	for _, key := range keys {
		if value, ok := item[key]; ok {
			if textAny(value) != "" {
				return value
			}
		}
	}
	return nil
}

func textAny(value any) string {
	if value == nil {
		return ""
	}
	return strings.TrimSpace(fmt.Sprint(value))
}

func withTermQuery(rawURL string, params termParams) string {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return rawURL
	}
	query := parsed.Query()
	query.Set("xnm", params.XueNian)
	query.Set("xqm", params.Xqm)
	parsed.RawQuery = query.Encode()
	return parsed.String()
}

func termFromRequest(req ExportRequest) termParams {
	xueNian := strings.TrimSpace(req.XueNian)
	if xueNian == "" {
		xueNian = "2026"
	}
	xueQi := strings.TrimSpace(req.XueQi)
	if xueQi == "" {
		xueQi = "1"
	}
	xqm := "12"
	if xueQi == "1" {
		xqm = "3"
	}
	return termParams{XueNian: xueNian, XueQi: xueQi, Xqm: xqm}
}

func (s *Service) RunExport(req ExportRequest) (*ExportResult, error) {
	return s.runExport(req, newExporter())
}

// RefreshPersonalSchedule reuses the exporter session established by RunExport.
// The session and request stay in memory only; credentials are never persisted.
func (s *Service) RefreshPersonalSchedule() (*ExportResult, error) {
	if err := s.ensureLoginForRefresh(); err != nil {
		return nil, err
	}
	exp, req, err := s.authenticatedSession()
	if err != nil {
		return nil, err
	}
	if !s.beginRun() {
		return nil, errors.New("已有导出任务正在进行，请等待当前任务完成")
	}
	defer s.endRun()
	return s.refreshPersonalScheduleWithSession(exp, req)
}

// StartPersonalScheduleRefresh starts a refresh without blocking the HTTP caller.
// It validates the in-memory session and running state before launching work.
func (s *Service) StartPersonalScheduleRefresh() error {
	if err := s.ensureLoginForRefresh(); err != nil {
		return err
	}
	exp, req, err := s.authenticatedSession()
	if err != nil {
		return err
	}
	if !s.beginRun() {
		return errors.New("已有导出任务正在进行，请等待当前任务完成")
	}
	s.setStatus("exporting", "personal", "正在使用已登录会话刷新个人课表。", false, nil)
	task := func() {
		defer s.endRun()
		_, _ = s.refreshPersonalScheduleWithSession(exp, req)
	}
	launch := s.launch
	if launch == nil {
		launch = func(fn func()) { go fn() }
	}
	launch(task)
	return nil
}

// ensureLoginForRefresh reuses an existing session when available; otherwise
// it attempts an automatic browser login using the local HDU login config.
func (s *Service) ensureLoginForRefresh() error {
	s.mu.RLock()
	hasSession := s.authenticated != nil
	s.mu.RUnlock()
	if hasSession {
		return nil
	}
	username, password, err := LoadLoginCredentials()
	if err != nil {
		return errors.New("请先完成登录后再刷新个人课表；自动登录未启用：" + err.Error())
	}
	browserExp, browserErr := newBrowserExporter(DefaultExporterEndpoints())
	if browserErr != nil {
		return fmt.Errorf("请先完成登录后再刷新个人课表；浏览器自动登录不可用：%w", browserErr)
	}
	if loginErr := browserExp.loginViaBrowser(username, password); loginErr != nil {
		return fmt.Errorf("请先完成登录后再刷新个人课表；浏览器自动登录失败：%w", loginErr)
	}
	s.setAuthenticatedSession(browserExp, ExportRequest{Method: "browser", Username: username, XueNian: "2026", XueQi: "1"})
	return nil
}
func (s *Service) authenticatedSession() (*exporter, ExportRequest, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.authenticated == nil {
		return nil, ExportRequest{}, errors.New("请先完成登录后再刷新个人课表")
	}
	return s.authenticated, s.loginRequest, nil
}

func (s *Service) refreshPersonalScheduleWithSession(exp *exporter, req ExportRequest) (*ExportResult, error) {
	s.setStatus("exporting", "personal", "正在使用已登录会话刷新个人课表。", false, nil)
	result, err := exp.exportPersonalSchedule(req)
	if err != nil {
		if isSessionInvalidError(err) {
			s.clearAuthenticatedSession()
		}
		err = explainExportError(err)
		s.setError("personal", err)
		return nil, err
	}
	s.setStatus("success", "personal", "个人课表刷新完成", true, result)
	return result, nil
}

// RunExportWithTestEndpoints runs the normal export flow against a local test
// server. It is deliberately separate from RunExport so the production GUI
// never accepts user-controlled endpoint URLs.
func (s *Service) RunExportWithTestEndpoints(req ExportRequest, endpoints ExporterEndpoints, timeout time.Duration) (*ExportResult, error) {
	if err := ValidateTestExporterEndpoints(endpoints); err != nil {
		return nil, err
	}
	return s.runExport(req, newExporterWithEndpoints(endpoints, timeout))
}

func (s *Service) runExport(req ExportRequest, exp *exporter) (*ExportResult, error) {
	if err := ValidateExportRequest(req); err != nil {
		s.setError("validate", err)
		return nil, err
	}
	if !s.beginRun() {
		return nil, errors.New("已有导出任务正在进行，请等待当前任务完成")
	}
	defer s.endRun()

	s.setStatus("validating", "validate", "参数检查通过，准备登录学校系统。", false, nil)

	s.clearAuthenticatedSession()
	method := normalizeExportMethod(req.Method)
	if method == "browser" {
		browserExp, browserErr := newBrowserExporter(exp.endpoints)
		if browserErr != nil {
			err := fmt.Errorf("无法使用浏览器登录会话：%w", browserErr)
			s.setError("login", err)
			return nil, err
		}
		exp = browserExp
		s.setStatus("login", "login", "正在使用已授权浏览器中的新教务登录会话。", false, nil)
	} else {
		s.setStatus("login", "login", "正在登录新教务；如果直登失败，会自动尝试统一身份认证或浏览器登录。", false, nil)
		if err := exp.login(req.Method, req.Username, req.Password); err != nil {
			primaryErr := explainExportError(err)
			wrongCredential := strings.Contains(primaryErr.Error(), "密码不正确") || strings.Contains(primaryErr.Error(), "账号或密码错误") || strings.Contains(primaryErr.Error(), "用户名或密码错误")
			if method != "password" || wrongCredential {
				s.setError("login", primaryErr)
				return nil, primaryErr
			}
			browserExp, browserErr := newBrowserExporter(exp.endpoints)
			if browserErr != nil {
				combined := fmt.Errorf("%v；自动切换浏览器登录不可用：%w", primaryErr, browserErr)
				s.setError("login", combined)
				return nil, combined
			}
			if loginErr := browserExp.loginViaBrowser(req.Username, req.Password); loginErr != nil {
				combined := fmt.Errorf("%v；浏览器自动登录失败：%w", primaryErr, loginErr)
				s.setError("login", combined)
				return nil, combined
			}
			exp = browserExp
			s.setStatus("login", "login", "密码直连被学校系统拦截，已在浏览器中完成登录。", false, nil)
		}
	}
	s.setAuthenticatedSession(exp, req)

	s.setStatus("exporting", "query", "登录成功，正在读取全校任务落实课程数据。", false, nil)
	result, err := exp.exportCourse(req)
	if err != nil {
		err = explainExportError(err)
		s.setError("query", err)
		return nil, err
	}

	s.setStatus("exporting", "personal", "全校课表已保存，正在读取个人课表。", false, result)
	personal, err := exp.exportPersonalSchedule(req)
	if err != nil {
		result.PersonalExportError = explainExportError(err).Error()
	} else {
		result.PersonalCount = personal.PersonalCount
		result.PersonalFileName = personal.PersonalFileName
		result.PersonalOutputPath = personal.PersonalOutputPath
		result.PersonalExported = true
	}

	message := "课程导出完成"
	if result.PersonalExported {
		message = "全校课表和个人课表均已导出完成"
	} else if result.PersonalExportError != "" {
		message = "全校课表已导出，个人课表导出失败：" + result.PersonalExportError
	}
	if result.CourseSource == "local-cache" {
		message = "全校课程接口暂不可用，已复用当前学期本地课表快照"
		if result.PersonalExported {
			message += "，个人课表已通过浏览器会话刷新"
		}
	}
	s.setStatus("success", "done", message, true, result)
	return result, nil
}

func (s *Service) setAuthenticatedSession(exp *exporter, req ExportRequest) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.authenticated = exp
	s.loginRequest = req
}

func (s *Service) clearAuthenticatedSession() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.authenticated = nil
	s.loginRequest = ExportRequest{}
}

func explainExportError(err error) error {
	if err == nil {
		return nil
	}
	text := err.Error()
	lower := strings.ToLower(text)
	switch {
	case strings.Contains(text, "账号") || strings.Contains(text, "密码"):
		return errors.New(text + "。如果确认密码正确，可能需要先在浏览器完成统一身份认证，或学校登录页已经更新。")
	case strings.Contains(lower, "timeout") || strings.Contains(text, "超时"):
		return errors.New("连接学校系统超时，请稍后重试。")
	case strings.Contains(lower, "no such host") || strings.Contains(lower, "connection refused"):
		return errors.New("无法连接学校系统，请检查网络、校园网或代理设置。")
	case strings.Contains(text, "无功能权限") || strings.Contains(text, "未开放"):
		return errors.New(text + "，请确认当前时间学校是否开放任务落实查询。")
	default:
		return errors.New(text)
	}
}

func mustAtoi(v string) int {
	n, _ := strconv.Atoi(v)
	return n
}
