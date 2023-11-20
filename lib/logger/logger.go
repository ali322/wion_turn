package logger

import (
	"os"
	"path/filepath"
	"time"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"gopkg.in/natefinch/lumberjack.v2"
)

func New(path string) *zap.Logger {
	encoder := getEncoder()
	writeSyncer := getWriteSyncer(path)
	core := zapcore.NewCore(encoder, writeSyncer, zapcore.DebugLevel)
	logger := zap.New(core, zap.AddCaller())
	zap.ReplaceGlobals(logger)
	return logger
}

func getEncoder() zapcore.Encoder {
	conf := zap.NewProductionEncoderConfig()
	conf.EncodeTime = func(t time.Time, pae zapcore.PrimitiveArrayEncoder) {
		pae.AppendString(t.Format("2006-01-02 15:04:05"))
	}
	conf.EncodeLevel = zapcore.CapitalLevelEncoder
	conf.EncodeDuration = zapcore.SecondsDurationEncoder
	conf.EncodeCaller = zapcore.ShortCallerEncoder
	conf.TimeKey = "time"
	return zapcore.NewConsoleEncoder(conf)
	// return zapcore.NewJSONEncoder(zap.NewProductionEncoderConfig())
}

func getWriteSyncer(path string) zapcore.WriteSyncer {
	workDir, _ := os.Getwd()
	p := filepath.Join(workDir, path)
	lumberjackLogger := &lumberjack.Logger{
		Filename:   p,
		MaxSize:    500, // mb
		MaxBackups: 3,
		MaxAge:     28,
		LocalTime:  true,
	}
	return zapcore.AddSync(lumberjackLogger)
}
