package config

import "testing"

func TestSupportsModel(t *testing.T) {
	tests := []struct {
		name            string
		supportedModels []string
		model           string
		want            bool
	}{
		{"空列表匹配所有", nil, "gpt-4o", true},
		{"空列表匹配空模型", nil, "", true},
		{"精确匹配", []string{"gpt-4o"}, "gpt-4o", true},
		{"精确不匹配", []string{"gpt-4o"}, "gpt-4-turbo", false},
		{"通配符匹配", []string{"gpt-4*"}, "gpt-4o", true},
		{"通配符匹配turbo", []string{"gpt-4*"}, "gpt-4-turbo", true},
		{"通配符不匹配", []string{"gpt-4*"}, "o3", false},
		{"多模式匹配第一个", []string{"gpt-4*", "claude-*"}, "gpt-4o", true},
		{"多模式匹配第二个", []string{"gpt-4*", "claude-*"}, "claude-3-opus", true},
		{"多模式都不匹配", []string{"gpt-4*", "claude-*"}, "o3", false},
		{"精确和通配符混合", []string{"o3", "gpt-4*"}, "o3", true},
		{"通配符星号本身", []string{"*"}, "anything", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			u := &UpstreamConfig{SupportedModels: tt.supportedModels}
			if got := u.SupportsModel(tt.model); got != tt.want {
				t.Errorf("SupportsModel(%q) = %v, want %v", tt.model, got, tt.want)
			}
		})
	}
}
