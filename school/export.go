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
		return errors.New("扫码登录需要在学校登录页完成认证后再导出")
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
	if strings.Contains(text, "统一身份认证") {
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
		return "", "", errors.New("未获取到 CAS 登录配置")
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
		return errors.New("未获取到 csrftoken")
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
		return "", errors.New("未获取到公钥")
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
		return nil, errors.New("任务落实查询未开放")
	}

	var payload CoursePayload
	if err := json.Unmarshal(body, &payload); err != nil {
		return nil, err
	}
	if len(payload.Items) == 0 {
		return nil, errors.New("没有拿到课程数据")
	}

	raw := map[string]any{"items": payload.Items}
	textBytes, _ := json.MarshalIndent(raw, "", "  ")
	if err := os.WriteFile("course.json", textBytes, 0644); err != nil {
		return nil, err
	}

	return &ExportResult{
		Count:      len(payload.Items),
		CourseName: InferCourseName(payload.Items),
		FileName:   "course.json",
	}, nil
}

func (s *Service) RunExport(req ExportRequest) (*ExportResult, error) {
	exp := newExporter()
	if err := exp.login(req.Method, req.Username, req.Password); err != nil {
		s.mu.Lock()
		s.status = StatusResponse{Ready: false, Message: err.Error()}
		s.mu.Unlock()
		return nil, err
	}

	result, err := exp.exportCourse(req)
	if err != nil {
		s.mu.Lock()
		s.status = StatusResponse{Ready: false, Message: err.Error()}
		s.mu.Unlock()
		return nil, err
	}

	s.mu.Lock()
	s.status = StatusResponse{
		Ready:      true,
		Count:      result.Count,
		CourseName: result.CourseName,
		Message:    "课程导出完成",
	}
	s.mu.Unlock()
	return result, nil
}

func mustAtoi(v string) int {
	n, _ := strconv.Atoi(v)
	return n
}
