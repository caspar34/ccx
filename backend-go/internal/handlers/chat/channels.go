// Package chat 提供 Chat Completions API 的渠道管理
package chat

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/BenedictKing/ccx/internal/config"
	"github.com/BenedictKing/ccx/internal/httpclient"
	"github.com/BenedictKing/ccx/internal/scheduler"
	"github.com/BenedictKing/ccx/internal/utils"
	"github.com/gin-gonic/gin"
)

// GetUpstreams 获取 Chat 上游列表
func GetUpstreams(cfgManager *config.ConfigManager) gin.HandlerFunc {
	return func(c *gin.Context) {
		cfg := cfgManager.GetConfig()

		upstreams := make([]gin.H, len(cfg.ChatUpstream))
		for i, up := range cfg.ChatUpstream {
			status := config.GetChannelStatus(&up)
			priority := config.GetChannelPriority(&up, i)

			upstreams[i] = gin.H{
				"index":              i,
				"name":               up.Name,
				"serviceType":        up.ServiceType,
				"baseUrl":            up.BaseURL,
				"baseUrls":           up.BaseURLs,
				"apiKeys":            up.APIKeys,
				"description":        up.Description,
				"website":            up.Website,
				"insecureSkipVerify": up.InsecureSkipVerify,
				"modelMapping":       up.ModelMapping,
				"latency":            nil,
				"status":             status,
				"priority":           priority,
				"promotionUntil":     up.PromotionUntil,
				"lowQuality":         up.LowQuality,
				"customHeaders":      up.CustomHeaders,
				"proxyUrl":           up.ProxyURL,
				"supportedModels":    up.SupportedModels,
			}
		}

		c.JSON(200, gin.H{
			"channels": upstreams,
		})
	}
}

// AddUpstream 添加 Chat 上游
func AddUpstream(cfgManager *config.ConfigManager) gin.HandlerFunc {
	return func(c *gin.Context) {
		var upstream config.UpstreamConfig
		if err := c.ShouldBindJSON(&upstream); err != nil {
			c.JSON(400, gin.H{"error": err.Error()})
			return
		}

		if err := cfgManager.AddChatUpstream(upstream); err != nil {
			c.JSON(500, gin.H{"error": err.Error()})
			return
		}

		c.JSON(200, gin.H{"message": "Chat upstream added successfully"})
	}
}

// UpdateUpstream 更新 Chat 上游
func UpdateUpstream(cfgManager *config.ConfigManager, sch *scheduler.ChannelScheduler) gin.HandlerFunc {
	return func(c *gin.Context) {
		idStr := c.Param("id")
		id, err := strconv.Atoi(idStr)
		if err != nil {
			c.JSON(400, gin.H{"error": "Invalid upstream ID"})
			return
		}

		var updates config.UpstreamUpdate
		if err := c.ShouldBindJSON(&updates); err != nil {
			c.JSON(400, gin.H{"error": err.Error()})
			return
		}

		shouldResetMetrics, err := cfgManager.UpdateChatUpstream(id, updates)
		if err != nil {
			c.JSON(500, gin.H{"error": err.Error()})
			return
		}

		// 单 key 更换时重置熔断状态
		if shouldResetMetrics {
			sch.ResetChannelMetrics(id, scheduler.ChannelKindChat)
		}

		c.JSON(200, gin.H{"message": "Chat upstream updated successfully"})
	}
}

// DeleteUpstream 删除 Chat 上游
func DeleteUpstream(cfgManager *config.ConfigManager, channelScheduler *scheduler.ChannelScheduler) gin.HandlerFunc {
	return func(c *gin.Context) {
		idStr := c.Param("id")
		id, err := strconv.Atoi(idStr)
		if err != nil {
			c.JSON(400, gin.H{"error": "Invalid upstream ID"})
			return
		}

		_, err = cfgManager.RemoveChatUpstream(id)
		if err != nil {
			if strings.Contains(err.Error(), "无效的") {
				c.JSON(404, gin.H{"error": "Upstream not found"})
			} else {
				c.JSON(500, gin.H{"error": err.Error()})
			}
			return
		}

		channelScheduler.GetChannelLogStore(scheduler.ChannelKindChat).ClearAll()

		c.JSON(200, gin.H{"message": "Chat upstream deleted successfully"})
	}
}

// AddApiKey 添加 Chat 渠道 API 密钥
func AddApiKey(cfgManager *config.ConfigManager) gin.HandlerFunc {
	return func(c *gin.Context) {
		idStr := c.Param("id")
		id, err := strconv.Atoi(idStr)
		if err != nil {
			c.JSON(400, gin.H{"error": "Invalid upstream ID"})
			return
		}

		var req struct {
			APIKey string `json:"apiKey"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(400, gin.H{"error": "Invalid request body"})
			return
		}

		if err := cfgManager.AddChatAPIKey(id, req.APIKey); err != nil {
			if strings.Contains(err.Error(), "无效的上游索引") {
				c.JSON(404, gin.H{"error": "Upstream not found"})
			} else if strings.Contains(err.Error(), "API密钥已存在") {
				c.JSON(400, gin.H{"error": "API密钥已存在"})
			} else {
				c.JSON(500, gin.H{"error": "Failed to save config"})
			}
			return
		}

		c.JSON(200, gin.H{
			"message": "API密钥已添加",
			"success": true,
		})
	}
}

// DeleteApiKey 删除 Chat 渠道 API 密钥
func DeleteApiKey(cfgManager *config.ConfigManager) gin.HandlerFunc {
	return func(c *gin.Context) {
		idStr := c.Param("id")
		id, err := strconv.Atoi(idStr)
		if err != nil {
			c.JSON(400, gin.H{"error": "Invalid upstream ID"})
			return
		}

		apiKey := c.Param("apiKey")
		if apiKey == "" {
			c.JSON(400, gin.H{"error": "API key is required"})
			return
		}

		if err := cfgManager.RemoveChatAPIKey(id, apiKey); err != nil {
			if strings.Contains(err.Error(), "无效的上游索引") {
				c.JSON(404, gin.H{"error": "Upstream not found"})
			} else if strings.Contains(err.Error(), "API密钥不存在") {
				c.JSON(404, gin.H{"error": "API key not found"})
			} else {
				c.JSON(500, gin.H{"error": "Failed to save config"})
			}
			return
		}

		c.JSON(200, gin.H{
			"message": "API密钥已删除",
		})
	}
}

// MoveApiKeyToTop 将 Chat 渠道 API 密钥移到最前面
func MoveApiKeyToTop(cfgManager *config.ConfigManager) gin.HandlerFunc {
	return func(c *gin.Context) {
		id, _ := strconv.Atoi(c.Param("id"))
		apiKey := c.Param("apiKey")

		if err := cfgManager.MoveChatAPIKeyToTop(id, apiKey); err != nil {
			c.JSON(400, gin.H{"error": err.Error()})
			return
		}
		c.JSON(200, gin.H{"message": "API密钥已置顶"})
	}
}

// MoveApiKeyToBottom 将 Chat 渠道 API 密钥移到最后面
func MoveApiKeyToBottom(cfgManager *config.ConfigManager) gin.HandlerFunc {
	return func(c *gin.Context) {
		id, _ := strconv.Atoi(c.Param("id"))
		apiKey := c.Param("apiKey")

		if err := cfgManager.MoveChatAPIKeyToBottom(id, apiKey); err != nil {
			c.JSON(400, gin.H{"error": err.Error()})
			return
		}
		c.JSON(200, gin.H{"message": "API密钥已置底"})
	}
}

// ReorderChannels 重新排序 Chat 渠道优先级
func ReorderChannels(cfgManager *config.ConfigManager) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req struct {
			Order []int `json:"order"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(400, gin.H{"error": "Invalid request body"})
			return
		}

		if err := cfgManager.ReorderChatUpstreams(req.Order); err != nil {
			c.JSON(400, gin.H{"error": err.Error()})
			return
		}

		c.JSON(200, gin.H{
			"success": true,
			"message": "Chat 渠道优先级已更新",
		})
	}
}

// SetChannelStatus 设置 Chat 渠道状态
func SetChannelStatus(cfgManager *config.ConfigManager) gin.HandlerFunc {
	return func(c *gin.Context) {
		idStr := c.Param("id")
		id, err := strconv.Atoi(idStr)
		if err != nil {
			c.JSON(400, gin.H{"error": "Invalid channel ID"})
			return
		}

		var req struct {
			Status string `json:"status"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(400, gin.H{"error": "Invalid request body"})
			return
		}

		if err := cfgManager.SetChatChannelStatus(id, req.Status); err != nil {
			if strings.Contains(err.Error(), "无效的上游索引") {
				c.JSON(404, gin.H{"error": "Channel not found"})
			} else {
				c.JSON(400, gin.H{"error": err.Error()})
			}
			return
		}

		c.JSON(200, gin.H{
			"success": true,
			"message": "Chat 渠道状态已更新",
			"status":  req.Status,
		})
	}
}

// SetChannelPromotion 设置 Chat 渠道促销期
func SetChannelPromotion(cfgManager *config.ConfigManager) gin.HandlerFunc {
	return func(c *gin.Context) {
		idStr := c.Param("id")
		id, err := strconv.Atoi(idStr)
		if err != nil {
			c.JSON(400, gin.H{"error": "Invalid channel ID"})
			return
		}

		var req struct {
			Duration int `json:"duration"` // 促销期时长（秒），0 表示清除
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(400, gin.H{"error": "Invalid request body"})
			return
		}

		duration := time.Duration(req.Duration) * time.Second
		if err := cfgManager.SetChatChannelPromotion(id, duration); err != nil {
			c.JSON(400, gin.H{"error": err.Error()})
			return
		}

		if req.Duration <= 0 {
			c.JSON(200, gin.H{
				"success": true,
				"message": "Chat 渠道促销期已清除",
			})
		} else {
			c.JSON(200, gin.H{
				"success":  true,
				"message":  "Chat 渠道促销期已设置",
				"duration": req.Duration,
			})
		}
	}
}

// PingChannel 测试 Chat 渠道连通性
func PingChannel(cfgManager *config.ConfigManager) gin.HandlerFunc {
	return func(c *gin.Context) {
		idStr := c.Param("id")
		id, err := strconv.Atoi(idStr)
		if err != nil {
			c.JSON(400, gin.H{"error": "Invalid channel ID"})
			return
		}

		cfg := cfgManager.GetConfig()
		if id < 0 || id >= len(cfg.ChatUpstream) {
			c.JSON(404, gin.H{"error": "Channel not found"})
			return
		}

		upstream := cfg.ChatUpstream[id]
		baseURL := upstream.GetEffectiveBaseURL()
		if baseURL == "" {
			c.JSON(400, gin.H{"error": "No base URL configured"})
			return
		}

		client := httpclient.GetManager().GetStandardClient(10*time.Second, upstream.InsecureSkipVerify, upstream.ProxyURL)

		// 根据 serviceType 选择不同的健康检查端点
		var testURL string
		var req *http.Request
		switch upstream.ServiceType {
		case "claude":
			// Claude API 没有 /v1/models，使用 /v1/messages 的 OPTIONS 请求
			testURL = fmt.Sprintf("%s/v1/messages", strings.TrimRight(baseURL, "/"))
			req, _ = http.NewRequest("OPTIONS", testURL, nil)
			if len(upstream.APIKeys) > 0 {
				utils.SetAuthenticationHeader(req.Header, upstream.APIKeys[0])
				req.Header.Set("anthropic-version", "2023-06-01")
			}
		default:
			// OpenAI / Gemini / Responses 等使用 /v1/models
			testURL = fmt.Sprintf("%s/v1/models", strings.TrimRight(baseURL, "/"))
			req, _ = http.NewRequest("GET", testURL, nil)
			if len(upstream.APIKeys) > 0 {
				utils.SetAuthenticationHeader(req.Header, upstream.APIKeys[0])
			}
		}

		start := time.Now()
		resp, err := client.Do(req)
		latency := time.Since(start).Milliseconds()

		if err != nil {
			c.JSON(200, gin.H{
				"success": false,
				"error":   err.Error(),
				"latency": latency,
			})
			return
		}
		defer resp.Body.Close()

		c.JSON(200, gin.H{
			"success":    resp.StatusCode >= 200 && resp.StatusCode < 400,
			"statusCode": resp.StatusCode,
			"latency":    latency,
		})
	}
}

// PingAllChannels 测试所有 Chat 渠道连通性
func PingAllChannels(cfgManager *config.ConfigManager) gin.HandlerFunc {
	return func(c *gin.Context) {
		cfg := cfgManager.GetConfig()
		results := make([]gin.H, len(cfg.ChatUpstream))

		for i, upstream := range cfg.ChatUpstream {
			baseURL := upstream.GetEffectiveBaseURL()
			if baseURL == "" {
				results[i] = gin.H{
					"index":   i,
					"name":    upstream.Name,
					"success": false,
					"error":   "No base URL configured",
				}
				continue
			}

			client := httpclient.GetManager().GetStandardClient(10*time.Second, upstream.InsecureSkipVerify, upstream.ProxyURL)

			// 根据 serviceType 选择不同的健康检查端点
			var testURL string
			var req *http.Request
			switch upstream.ServiceType {
			case "claude":
				testURL = fmt.Sprintf("%s/v1/messages", strings.TrimRight(baseURL, "/"))
				req, _ = http.NewRequest("OPTIONS", testURL, nil)
				if len(upstream.APIKeys) > 0 {
					utils.SetAuthenticationHeader(req.Header, upstream.APIKeys[0])
					req.Header.Set("anthropic-version", "2023-06-01")
				}
			default:
				testURL = fmt.Sprintf("%s/v1/models", strings.TrimRight(baseURL, "/"))
				req, _ = http.NewRequest("GET", testURL, nil)
				if len(upstream.APIKeys) > 0 {
					utils.SetAuthenticationHeader(req.Header, upstream.APIKeys[0])
				}
			}

			start := time.Now()
			resp, err := client.Do(req)
			latency := time.Since(start).Milliseconds()

			if err != nil {
				results[i] = gin.H{
					"index":   i,
					"name":    upstream.Name,
					"success": false,
					"error":   err.Error(),
					"latency": latency,
				}
				continue
			}
			resp.Body.Close()

			results[i] = gin.H{
				"index":      i,
				"name":       upstream.Name,
				"success":    resp.StatusCode >= 200 && resp.StatusCode < 400,
				"statusCode": resp.StatusCode,
				"latency":    latency,
			}
		}

		c.JSON(200, gin.H{
			"channels": results,
		})
	}
}
