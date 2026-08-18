package school

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
)

const loginConfigEnv = "HDU_LOGIN_CONFIG"

type loginConfigFile struct {
	NewJWLogin struct {
		Username string `json:"username"`
		Password string `json:"password"`
	} `json:"newjw_login"`
	CASLogin struct {
		Username string `json:"username"`
		Password string `json:"password"`
	} `json:"cas_login"`
}

// LoadLoginCredentials discovers a local HDU login config and returns the
// username/password it contains. Password values are never logged.
func LoadLoginCredentials() (string, string, error) {
	if configured := strings.TrimSpace(os.Getenv(loginConfigEnv)); configured != "" {
		username, password, err := readLoginCredentials(configured)
		if err != nil {
			return "", "", err
		}
		if strings.TrimSpace(username) == "" || strings.TrimSpace(password) == "" {
			return "", "", errors.New("登录配置文件缺少账号或密码")
		}
		return strings.TrimSpace(username), password, nil
	}
	candidates := make([]string, 0, 4)
	if exe, err := os.Executable(); err == nil {
		exeDir := filepath.Dir(exe)
		candidates = append(candidates,
			filepath.Join(exeDir, "login-config.json"),
			filepath.Join(exeDir, "..", "选课脚本", "config.json"),
		)
	}
	if wd, err := os.Getwd(); err == nil {
		candidates = append(candidates,
			filepath.Join(wd, "login-config.json"),
			filepath.Join(wd, "..", "选课脚本", "config.json"),
		)
	}
	var lastErr error
	for _, candidate := range candidates {
		if candidate == "" {
			continue
		}
		username, password, err := readLoginCredentials(candidate)
		if err != nil {
			lastErr = err
			continue
		}
		if strings.TrimSpace(username) != "" && strings.TrimSpace(password) != "" {
			return strings.TrimSpace(username), password, nil
		}
	}
	if lastErr == nil {
		lastErr = errors.New("未找到可用的登录配置文件（login-config.json 或 选课脚本/config.json）")
	}
	return "", "", lastErr
}

func readLoginCredentials(path string) (string, string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", "", err
	}
	var config loginConfigFile
	if err := json.Unmarshal(data, &config); err != nil {
		return "", "", err
	}
	if strings.TrimSpace(config.NewJWLogin.Username) != "" {
		return strings.TrimSpace(config.NewJWLogin.Username), config.NewJWLogin.Password, nil
	}
	return strings.TrimSpace(config.CASLogin.Username), config.CASLogin.Password, nil
}
