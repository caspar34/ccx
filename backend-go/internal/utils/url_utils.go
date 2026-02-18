package utils

import (
	"net/url"
	"regexp"
)

// reURLPassword 匹配 URL 中 user:password@ 的密码部分
var reURLPassword = regexp.MustCompile(`(://[^:@/]+:)[^@]+(@)`)

// RedactURLCredentials 对 URL 中的用户名和密码进行脱敏处理
// 例如: http://user:pass@host:port -> http://user:***@host:port
// 若 URL 解析失败，使用正则兜底替换，避免凭证泄露
func RedactURLCredentials(rawURL string) string {
	if rawURL == "" {
		return rawURL
	}

	u, err := url.Parse(rawURL)
	if err != nil {
		// 解析失败时用正则兜底，避免凭证明文出现在日志中
		return reURLPassword.ReplaceAllString(rawURL, "${1}***${2}")
	}

	if u.User != nil {
		username := u.User.Username()
		// 构建脱敏后的 Userinfo
		u.User = url.UserPassword(username, "***")
		return u.String()
	}

	return rawURL
}
