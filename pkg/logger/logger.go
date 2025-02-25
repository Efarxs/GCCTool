package logger

import (
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"
	"jwxt/model"
	"strings"
	"sync"
	"time"
)

type Logger struct {
	LogLabel  *widget.Label
	LogScroll *container.Scroll
	logBuffer chan string
	mu        sync.Mutex
}

func NewLogger(ui *model.UIComponents) *Logger {
	logger := &Logger{
		LogLabel:  ui.LogLabel,
		LogScroll: ui.LogScroll,
		logBuffer: make(chan string, 1000), // 缓冲通道，容量为 1000
	}

	// 启动日志处理 Goroutine
	go logger.processLogs()

	return logger
}

func (l *Logger) AppendLog(message string) {
	// 将日志消息发送到缓冲通道
	l.logBuffer <- message
}

func (l *Logger) processLogs() {
	var logs []string
	ticker := time.NewTicker(100 * time.Millisecond) // 每 100 毫秒更新一次 UI

	for {
		select {
		case message := <-l.logBuffer:
			// 收集日志消息
			logs = append(logs, message)

			// 限制日志行数
			maxLines := 100
			if len(logs) > maxLines {
				logs = logs[len(logs)-maxLines:]
			}

		case <-ticker.C:
			// 定期更新 UI
			l.mu.Lock()
			l.LogLabel.SetText(strings.Join(logs, "\n"))
			l.LogScroll.ScrollToBottom()
			l.mu.Unlock()
		}
	}
}
