package pyroscope

import (
	"fmt"
	"log/slog"
)

// slogAdapter 用於適配 slog.Logger 以符合 pyroscope.Logger 接口
type slogAdapter struct {
	logger *slog.Logger
}

// NewSlogAdapter 創建一個新的 SlogAdapter
func NewSlogAdapter(logger *slog.Logger) *slogAdapter {
	return &slogAdapter{logger: logger}
}

// Debugf 實現 pyroscope.Logger 的 Debugf 方法
func (s slogAdapter) Debugf(format string, args ...interface{}) {
	s.logger.Debug(fmt.Sprintf(format, args...))
}

// Infof 實現 pyroscope.Logger 的 Infof 方法
func (s slogAdapter) Infof(format string, args ...interface{}) {
	s.logger.Info(fmt.Sprintf(format, args...))
}

// Warnf 實現 pyroscope.Logger 的 Warnf 方法
func (s slogAdapter) Warnf(format string, args ...interface{}) {
	s.logger.Warn(fmt.Sprintf(format, args...))
}

// Errorf 實現 pyroscope.Logger 的 Errorf 方法
func (s slogAdapter) Errorf(format string, args ...interface{}) {
	s.logger.Error(fmt.Sprintf(format, args...))
}
