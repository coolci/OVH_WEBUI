package auth

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// MinAPIKeyLength 开启鉴权时允许的最小主密钥长度。
const MinAPIKeyLength = 24

// ValidateStartupAPIKey 校验生产启动时的主密钥强度，拒绝空口令、常见默认弱口令及过短密钥。
func ValidateStartupAPIKey(key string) error {
	trimmed := strings.TrimSpace(key)
	if trimmed == "" {
		return errors.New("API_SECRET_KEY 不能为空")
	}
	lower := strings.ToLower(trimmed)
	switch lower {
	case "123456", "12345678", "1234567890", "password", "admin", "secret", "changeme", "default", "ovh-webui-dev-key":
		return fmt.Errorf("检测到高危默认弱口令 %q，请在配置文件中置空以自动生成随机密钥或设置至少 %d 位强随机密钥", trimmed, MinAPIKeyLength)
	}
	for _, weakPrefix := range []string{"change-me", "your-api-key", "replace-me"} {
		if strings.HasPrefix(lower, weakPrefix) {
			return fmt.Errorf("检测到示例占位弱口令 %q，请更换为至少 %d 位强随机密钥", trimmed, MinAPIKeyLength)
		}
	}
	if len(trimmed) < MinAPIKeyLength {
		return fmt.Errorf("API_SECRET_KEY 长度不足（当前 %d 位，至少需要 %d 位强随机字符）", len(trimmed), MinAPIKeyLength)
	}
	return nil
}

// ResolveAPIKey preserves configured keys and persists a random key for an unconfigured installation.
func ResolveAPIKey(dataDir, configured string) (string, bool, error) {
	if strings.TrimSpace(configured) != "" {
		return configured, false, nil
	}
	path := filepath.Join(dataDir, ".api-secret")
	deadline := time.Now().Add(time.Second)
	for {
		file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
		if err == nil {
			key := make([]byte, 32)
			if _, err = rand.Read(key); err == nil {
				_, err = file.WriteString(hex.EncodeToString(key) + "\n")
			}
			if err == nil {
				err = file.Sync()
			}
			closeErr := file.Close()
			if err == nil {
				err = closeErr
			}
			if err != nil {
				_ = os.Remove(path)
				return "", false, fmt.Errorf("保存 API 密钥失败: %w", err)
			}
			return hex.EncodeToString(key), true, nil
		}
		if !errors.Is(err, os.ErrExist) {
			return "", false, fmt.Errorf("创建 API 密钥失败: %w", err)
		}
		info, err := os.Lstat(path)
		if err != nil {
			return "", false, err
		}
		if !info.Mode().IsRegular() {
			return "", false, fmt.Errorf("API 密钥路径必须是普通文件: %s", path)
		}
		body, err := os.ReadFile(path)
		if err != nil {
			return "", false, err
		}
		value := strings.TrimSpace(string(body))
		key, err := hex.DecodeString(value)
		if err == nil && len(key) == 32 {
			if err := os.Chmod(path, 0600); err != nil {
				return "", false, err
			}
			return value, false, nil
		}
		// A concurrent first start may have created the file but not finished writing it.
		if time.Now().After(deadline) {
			return "", false, fmt.Errorf("API 密钥文件无效,请恢复 %s 或配置 API_SECRET_KEY", path)
		}
		time.Sleep(10 * time.Millisecond)
	}
}
