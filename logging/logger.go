package logging

import (
	"log/slog"
	"os"
	"strings"
)

// NewLogger는 애플리케이션의 중앙 로거를 생성하고 설정합니다.
func NewLogger(logLevel string) *slog.Logger {
	var level slog.Level

	// 설정 파일의 로그 레벨 문자열을 slog.Level 타입으로 변환합니다.
	switch strings.ToUpper(logLevel) {
	case "DEBUG":
		level = slog.LevelDebug
	case "INFO":
		level = slog.LevelInfo
	case "WARN":
		level = slog.LevelWarn
	case "ERROR":
		level = slog.LevelError
	default:
		level = slog.LevelInfo
	}

	// JSON 형식으로 로그를 출력하는 핸들러를 설정합니다.
	opts := &slog.HandlerOptions{
		Level: level,
		// 소스 코드 위치를 로그에 포함시켜 디버깅을 용이하게 합니다.
		AddSource: true,
	}
	handler := slog.NewJSONHandler(os.Stdout, opts)

	return slog.New(handler)
}
