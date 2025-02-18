package log

import (
	log "github.com/sirupsen/logrus"
)

var loggerLevel log.Level = log.InfoLevel

func SetLevel(level log.Level) {
	loggerLevel = level
}

func GetLogger(cmd string, action string) *log.Entry {

	logger := log.WithFields(log.Fields{"entity": cmd, "action": action})
	logger.Logger.SetLevel(loggerLevel)
	//log.SetReportCaller(true)
	return logger

}

func init() {
	customFormatter := new(log.TextFormatter)
	customFormatter.TimestampFormat = "2006-01-02 15:04:05"
	customFormatter.FullTimestamp = true
	log.SetFormatter(customFormatter)
}
