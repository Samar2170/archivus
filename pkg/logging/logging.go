package logging

import (
	"archivus/config"
	"path/filepath"

	"github.com/natefinch/lumberjack"
	"github.com/rs/zerolog"
)

var Errorlogger zerolog.Logger
var AuditLogger zerolog.Logger

func init() {
	auditLogFilePath := filepath.Join(config.Config.LogsDir, "audit.log")
	errorLogFilePath := filepath.Join(config.Config.LogsDir, "error.log")
	auditLogFile := &lumberjack.Logger{
		Filename:   auditLogFilePath,
		MaxSize:    10,
		MaxBackups: 3,
		MaxAge:     28,
		Compress:   false,
	}
	logFile := &lumberjack.Logger{
		Filename:   errorLogFilePath,
		MaxSize:    10,
		MaxBackups: 3,
		MaxAge:     28,
		Compress:   false,
	}

	Errorlogger = zerolog.New(logFile).With().Timestamp().Logger()
	AuditLogger = zerolog.New(auditLogFile).With().Timestamp().Logger()
}

func HandleError(err error) error {
	if err != nil {
		Errorlogger.Error().Err(err).Msg("An error occurred")
	}
	return err
}
