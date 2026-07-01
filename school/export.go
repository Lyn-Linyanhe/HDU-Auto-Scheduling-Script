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
	"io"
	"math/big"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"
)

const courseURL = "https://newjw.hdu.edu.cn/jwglxt/rwlscx/rwlscx_cxRwlsIndex.html?doType=query&gnmkdm=N1548"
const userAgent = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/143.0.0.0 Safari/537.36 Edg/143.0.0.0"

type ExportResult struct {
	Count      int    `json:"count"`
	CourseName string `json:"courseName"`
	FileName   string `json:"fileName"`
	OutputPath string `json:"outputPath"`
}

type exporter struct {
	client *http.Client
}

type publicKeyPayload struct {
	Modulus  string `json:"modulus"`
	Exponent string `json:"exponent"`
}

func newExporter() *exporter {
	jar, _ := cookiejar.New(nil)
	return &exporter{
		client: &http.Client{
			Jar:     jar,
			Timeout: 90 * time.Second,
		},
	}
}

func (e *exporter) login(method, username, password string) error {
	switch strings.ToLower(strings.TrimSpace(method)) {
	case "qr":
		return e.loginCAS("qr", username, password)
	case "password", "":
		if err := e.loginNewJW(username, password); err == nil {
			return nil
		}
		return e.loginCAS("password", username, password)
	default:
		if err := e.loginNewJW(username, password); err == nil {
			return nil
		}
		return e.loginCAS("password", username, password)
	}
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

	req, err := http.NewRequest(http.MethodPost, "https://sso.hdu.edu.cn/login", strings.NewReader(form.Encode()))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("User-Agent", userAgent)

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

	return e.finishCASLogin()
}

func (e *exporter) getCASLoginConfig() (execution, croypto string, err error) {
	resp, err := e.client.Get("https://sso.hdu.edu.cn/login")
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

func (e *exporter) finishCASLogin() error {
	resp, err := e.client.Get("https://sso.hdu.edu.cn/login?service=http://newjw.hdu.edu.cn/sso/driot4login")
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	_, _ = io.ReadAll(resp.Body)
	return nil
}

func (e *exporter) loginNewJW(username, password string) error {
	resp, err := e.client.Get("https://newjw.hdu.edu.cn/jwglxt/xtgl/login_slogin.html")
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

	req, err := http.NewRequest(http.MethodPost, "https://newjw.hdu.edu.cn/jwglxt/xtgl/login_slogin.html", strings.NewReader(form.Encode()))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("User-Agent", userAgent)

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
	resp, err := e.client.Get("https://newjw.hdu.edu.cn/jwglxt/xtgl/login_getPublicKey.html?time=" + strconv.FormatInt(time.Now().UnixMilli(), 10))
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
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
	resp, err := e.client.Get("https://newjw.hdu.edu.cn/jwglxt/kbcx/xskbcx_cxXsgrkb.html?gnmkdm=N2151&xnm=2022&xqm=3")
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
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

func (e *exporter) exportCourse(req ExportRequest) (*ExportResult, error) {
	xueNian := strings.TrimSpace(req.XueNian)
	if xueNian == "" {
		xueNian = "2025"
	}
	xueQi := strings.TrimSpace(req.XueQi)
	if xueQi == "" {
		xueQi = "2"
	}
	xqm := "12"
	if xueQi == "1" {
		xqm = "3"
	}

	form := url.Values{}
	form.Set("xnmc", xueNian+"-"+strconv.Itoa(mustAtoi(xueNian)+1))
	form.Set("xqmc", xueQi)
	form.Set("xnm", xueNian)
	form.Set("xqm", xqm)
	form.Set("_search", "false")
	form.Set("nd", strconv.FormatInt(time.Now().Unix(), 10))
	form.Set("queryModel.showCount", "9999")
	form.Set("queryModel.currentPage", "1")
	form.Set("queryModel.sortOrder", "asc")
	form.Set("time", "0")
	form.Set("jxbmc", "")

	reqHTTP, err := http.NewRequest(http.MethodPost, courseURL, strings.NewReader(form.Encode()))
	if err != nil {
		return nil, err
	}
	reqHTTP.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	reqHTTP.Header.Set("User-Agent", userAgent)

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

	text := string(body)
	if strings.Contains(text, "统一身份认证") {
		return nil, errors.New("登录已失效，请重新登录")
	}
	if strings.Contains(text, "无功能权限") {
		return nil, errors.New("任务落实查询未开放或当前账号没有权限")
	}

	var payload CoursePayload
	if err := json.Unmarshal(body, &payload); err != nil {
		return nil, errors.New("课程接口返回内容不是可解析的 JSON，可能仍停留在登录页")
	}
	if len(payload.Items) == 0 {
		return nil, errors.New("没有拿到课程数据，请确认学年学期是否正确")
	}

	raw := map[string]any{"items": payload.Items}
	textBytes, _ := json.MarshalIndent(raw, "", "  ")
	if err := os.WriteFile("course.json", textBytes, 0644); err != nil {
		return nil, err
	}
	outputPath, _ := filepath.Abs("course.json")

	return &ExportResult{
		Count:      len(payload.Items),
		CourseName: InferCourseName(payload.Items),
		FileName:   "course.json",
		OutputPath: outputPath,
	}, nil
}

func (s *Service) RunExport(req ExportRequest) (*ExportResult, error) {
	if err := ValidateExportRequest(req); err != nil {
		s.setError("validate", err)
		return nil, err
	}
	if !s.beginRun() {
		return nil, errors.New("已有导出任务正在进行，请等待当前任务完成")
	}
	defer s.endRun()

	s.setStatus("validating", "validate", "参数检查通过，准备登录学校系统。", false, nil)
	exp := newExporter()

	s.setStatus("login", "login", "正在登录新教务；如果直登失败，会自动尝试统一身份认证。", false, nil)
	if err := exp.login(req.Method, req.Username, req.Password); err != nil {
		err = explainExportError(err)
		s.setError("login", err)
		return nil, err
	}

	s.setStatus("exporting", "query", "登录成功，正在读取任务落实课程数据。", false, nil)
	result, err := exp.exportCourse(req)
	if err != nil {
		err = explainExportError(err)
		s.setError("export", err)
		return nil, err
	}

	s.setStatus("success", "done", "课程导出完成", true, result)
	return result, nil
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
