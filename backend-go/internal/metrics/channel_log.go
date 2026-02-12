package metrics

import (
	"sync"
	"time"
)

// ChannelLog 单次上游请求日志
type ChannelLog struct {
	Timestamp     time.Time `json:"timestamp"`
	Model         string    `json:"model"`                   // 实际使用的模型（重定向后）
	OriginalModel string    `json:"originalModel,omitempty"` // 原始请求模型（仅当重定向时有值）
	StatusCode    int       `json:"statusCode"`
	DurationMs    int64     `json:"durationMs"`
	Success       bool      `json:"success"`
	KeyMask       string    `json:"keyMask"`
	BaseURL       string    `json:"baseUrl"`
	ErrorInfo     string    `json:"errorInfo"`
	IsRetry       bool      `json:"isRetry"`
}

const maxChannelLogs = 50

// ChannelLogStore 渠道日志存储（内存环形缓冲区）
type ChannelLogStore struct {
	mu   sync.RWMutex
	logs map[int][]*ChannelLog // key: channelIndex
}

func NewChannelLogStore() *ChannelLogStore {
	return &ChannelLogStore{logs: make(map[int][]*ChannelLog)}
}

func (s *ChannelLogStore) Record(channelIndex int, log *ChannelLog) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.logs[channelIndex] = append(s.logs[channelIndex], log)
	if len(s.logs[channelIndex]) > maxChannelLogs {
		s.logs[channelIndex] = s.logs[channelIndex][len(s.logs[channelIndex])-maxChannelLogs:]
	}
}

// ClearAll 清除所有渠道日志（渠道删除导致索引变化时调用）
func (s *ChannelLogStore) ClearAll() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.logs = make(map[int][]*ChannelLog)
}

func (s *ChannelLogStore) Get(channelIndex int) []*ChannelLog {
	s.mu.RLock()
	defer s.mu.RUnlock()
	src := s.logs[channelIndex]
	if len(src) == 0 {
		return nil
	}
	// 返回副本，按时间倒序（最新在前）
	result := make([]*ChannelLog, len(src))
	for i, j := 0, len(src)-1; j >= 0; i, j = i+1, j-1 {
		result[i] = src[j]
	}
	return result
}
