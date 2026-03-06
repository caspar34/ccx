package gemini

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/BenedictKing/ccx/internal/config"
	"github.com/gin-gonic/gin"
)

func setupGeminiConfigManager(t *testing.T, upstream []config.UpstreamConfig) *config.ConfigManager {
	t.Helper()
	cfg := config.Config{GeminiUpstream: upstream}
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		t.Fatalf("序列化配置失败: %v", err)
	}
	tmpFile := filepath.Join(t.TempDir(), "config.json")
	if err := os.WriteFile(tmpFile, data, 0644); err != nil {
		t.Fatalf("写入配置文件失败: %v", err)
	}
	cm, err := config.NewConfigManager(tmpFile)
	if err != nil {
		t.Fatalf("创建配置管理器失败: %v", err)
	}
	t.Cleanup(func() { cm.Close() })
	return cm
}

func TestGetUpstreams_IncludesAdvancedOptionFields(t *testing.T) {
	cm := setupGeminiConfigManager(t, []config.UpstreamConfig{{
		Name:             "gemini-ch",
		ServiceType:      "gemini",
		BaseURL:          "https://api.example.com",
		APIKeys:          []string{"sk-1"},
		ModelMapping:     map[string]string{"gemini-2.5-pro": "gemini-2.5-flash"},
		ReasoningMapping: map[string]string{"gemini-2.5-pro": "high"},
		TextVerbosity:    "medium",
		FastMode:         true,
	}})

	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.GET("/gemini/channels", GetUpstreams(cm))

	req := httptest.NewRequest(http.MethodGet, "/gemini/channels", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", w.Code)
	}

	var resp struct {
		Channels []map[string]interface{} `json:"channels"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	ch := resp.Channels[0]
	if ch["textVerbosity"] != "medium" {
		t.Fatalf("textVerbosity = %v, want medium", ch["textVerbosity"])
	}
	if ch["fastMode"] != true {
		t.Fatalf("fastMode = %v, want true", ch["fastMode"])
	}
	rm, ok := ch["reasoningMapping"].(map[string]interface{})
	if !ok || rm["gemini-2.5-pro"] != "high" {
		t.Fatalf("reasoningMapping = %#v, want gemini-2.5-pro=high", ch["reasoningMapping"])
	}
}
