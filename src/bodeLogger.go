package bodeLogger

import (
	"context"
	"log/slog"
	"os"
	"time"
	"net/http"
	"sync"
)

var LoggerKeyName string = "logger"


type OrderedLogger struct {
	*slog.Logger
	Counter int
	Mu      sync.Mutex
}

func (orderedLogger *OrderedLogger) Log(ctx context.Context, level slog.Level, msg string, attrs ...slog.Attr) {
	orderedLogger.Mu.Lock()
	orderedLogger.Counter++
	orderedLogger.Mu.Unlock()
	attrs = append(attrs, slog.Int("order_index", orderedLogger.Counter))
	anyAttrs := make([]any, len(attrs))
	for i, attr := range attrs {
		anyAttrs[i] = attr
	}
	orderedLogger.Logger.Log(ctx, level, msg, anyAttrs...)
}


func increment(orderedLogger *OrderedLogger) {
	orderedLogger.Mu.Lock()
	orderedLogger.Counter++
	orderedLogger.Mu.Unlock()
}


func (orderedLogger *OrderedLogger) Debug(msg string, args ...any) {
	orderedLogger.Logger.Debug(msg, append(args, slog.Int("order_index", orderedLogger.Counter))...)
	increment(orderedLogger)
}

func (orderedLogger *OrderedLogger) Info(msg string, args ...any) {
	orderedLogger.Logger.Info(msg, append(args, slog.Int("order_index", orderedLogger.Counter))...)
	increment(orderedLogger)
}

func (orderedLogger *OrderedLogger) Warn(msg string, args ...any) {
	orderedLogger.Logger.Warn(msg, append(args, slog.Int("order_index", orderedLogger.Counter))...)
	increment(orderedLogger)
}

func (orderedLogger *OrderedLogger) Error(msg string, args ...any) {
	orderedLogger.Logger.Error(msg, append(args, slog.Int("order_index", orderedLogger.Counter))...)
	increment(orderedLogger)
}


func InitLogger(request *http.Request) *OrderedLogger {
	jsonHandler := slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		ReplaceAttr: func(groups []string, a slog.Attr) slog.Attr {
			if a.Key == slog.LevelKey {
				a.Key = "severity"
			}
			if a.Key == "msg" {
				a.Key = "message"
			}
			if a.Key == "time" {
				timeAsString := a.Value.Any().(time.Time).UTC().Format(time.RFC3339Nano)
				a.Value = slog.StringValue(timeAsString)
			}
			return a
		},
	})
	
	return &OrderedLogger{
		Logger: slog.New(jsonHandler).With(
			slog.String("session_id", request.Header.Get("X-Session-Id")),
			slog.String("test_id", request.Header.Get("X-Test-Id")),
			slog.String("request_id", request.Header.Get("X-Request-Id")),
		),
	}
}

func GetLogger(request *http.Request) (*OrderedLogger, bool) {
    logger, loggerOk := request.Context().Value(LoggerKeyName).(*OrderedLogger)
	if !loggerOk { return nil, false }
    return logger, true
}

func WithLogger() func(next http.Handler) http.Handler {
    return func(next http.Handler) http.Handler {
        return http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
            ctx := request.Context()
			logger := InitLogger(request)
			newRequest := request.WithContext(context.WithValue(ctx, LoggerKeyName, logger))
            next.ServeHTTP(writer, newRequest)
			logger.Info("end-of-request")
        })
    }
}