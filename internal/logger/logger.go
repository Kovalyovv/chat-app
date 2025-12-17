package logger

import "log"

type Logger struct{}

func New() *Logger { return &Logger{} }

func (*Logger) Info(msg string) { log.Println("[INFO] " + msg) }
func (*Logger) Infof(format string, v ...interface{}) {
	log.Printf("[INFO] "+format, v...)
}
func (*Logger) Fatalf(format string, v ...interface{}) {
	log.Fatalf("[FATAL] "+format, v...)
}
func (*Logger) Fatal(msg string) { log.Fatal("[FATAL] " + msg) }
