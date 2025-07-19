package monitoring

import "github.com/sirupsen/logrus"

// Logger defines the interface for logging operations
type Logger interface {
	Debug(msg string, fields ...interface{})
	Info(msg string, fields ...interface{})
	Warn(msg string, fields ...interface{})
	Error(msg string, fields ...interface{})
}

// LogrusAdapter adapts logrus.Logger to the Logger interface
type LogrusAdapter struct {
	logger *logrus.Logger
}

// NewLogrusAdapter creates a new logrus adapter
func NewLogrusAdapter(logger *logrus.Logger) Logger {
	return &LogrusAdapter{logger: logger}
}

// NewLogger creates a new logger with the specified level
func NewLogger(name, level string) Logger {
	logger := logrus.New()
	
	// Set log level
	switch level {
	case "debug":
		logger.SetLevel(logrus.DebugLevel)
	case "info":
		logger.SetLevel(logrus.InfoLevel)
	case "warn":
		logger.SetLevel(logrus.WarnLevel)
	case "error":
		logger.SetLevel(logrus.ErrorLevel)
	default:
		logger.SetLevel(logrus.InfoLevel)
	}
	
	// Set JSON formatter for structured logging
	logger.SetFormatter(&logrus.JSONFormatter{
		TimestampFormat: "2006-01-02T15:04:05.000Z07:00",
		FieldMap: logrus.FieldMap{
			logrus.FieldKeyTime:  "timestamp",
			logrus.FieldKeyLevel: "level",
			logrus.FieldKeyMsg:   "message",
		},
	})
	
	return &LogrusAdapter{logger: logger}
}

// Debug logs a debug message
func (l *LogrusAdapter) Debug(msg string, fields ...interface{}) {
	entry := l.logger.WithFields(l.fieldsToLogrusFields(fields))
	entry.Debug(msg)
}

// Info logs an info message
func (l *LogrusAdapter) Info(msg string, fields ...interface{}) {
	entry := l.logger.WithFields(l.fieldsToLogrusFields(fields))
	entry.Info(msg)
}

// Warn logs a warning message
func (l *LogrusAdapter) Warn(msg string, fields ...interface{}) {
	entry := l.logger.WithFields(l.fieldsToLogrusFields(fields))
	entry.Warn(msg)
}

// Error logs an error message
func (l *LogrusAdapter) Error(msg string, fields ...interface{}) {
	entry := l.logger.WithFields(l.fieldsToLogrusFields(fields))
	entry.Error(msg)
}

// fieldsToLogrusFields converts key-value pairs to logrus.Fields
func (l *LogrusAdapter) fieldsToLogrusFields(fields []interface{}) logrus.Fields {
	logrusFields := make(logrus.Fields)
	
	for i := 0; i < len(fields); i += 2 {
		if i+1 < len(fields) {
			if key, ok := fields[i].(string); ok {
				logrusFields[key] = fields[i+1]
			}
		}
	}
	
	return logrusFields
}