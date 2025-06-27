package logger

import (
	"fmt"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"
	"github.com/atotto/clipboard"
	"jwxt/model"
	"log"
	"os"
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

// 复制日志内容到剪切板
func (l *Logger) Copy() bool {
	l.mu.Lock()
	msg := l.LogLabel.Text
	err := clipboard.WriteAll(msg)
	l.mu.Unlock()
	if err != nil {
		return false
	}
	return true
}

// WriteToFile 写日志到文件
func (l *Logger) WriteToFile(message string) {
	// 打开文件，如果文件不存在则创建
	file, err := os.OpenFile("./app-"+time.Now().Format("20060102")+".log", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		log.Fatalf("无法打开文件: %v", err)
	}
	defer file.Close()

	// 获取当前时间
	currentTime := time.Now().Format("2006-01-02 15:04:05")

	// 格式化日志信息
	logMessage := fmt.Sprintf("[%s] %s\n", currentTime, message)

	// 写入日志
	if _, err = file.WriteString(logMessage); err != nil {
		l.AppendLog("无法写入日志文件: " + err.Error())
	}
}
