package log

import (
	"database/sql"
	"github.com/jmoiron/sqlx"
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

type SQLLogger struct {
	Execer   sqlx.Execer
	Queryer  sqlx.Queryer
	Logger   *log.Entry
	DebugSql bool
}

func (p *SQLLogger) Exec(query string, args ...interface{}) (sql.Result, error) {
	if p.DebugSql {
		p.Logger.Debugf("%s %s", query, args)
	}
	return p.Execer.Exec(query, args...)
}

func (p *SQLLogger) Query(query string, args ...interface{}) (*sql.Rows, error) {
	if p.DebugSql {
		p.Logger.Debugf("%s %s", query, args)
	}
	return p.Queryer.Query(query, args...)
}

func (p *SQLLogger) Queryx(query string, args ...interface{}) (*sqlx.Rows, error) {
	if p.DebugSql {
		p.Logger.Debugf("%s %s", query, args)
	}
	return p.Queryer.Queryx(query, args...)
}

func (p *SQLLogger) QueryRowx(query string, args ...interface{}) *sqlx.Row {
	if p.DebugSql {
		p.Logger.Debugf("%s %s", query, args)
	}
	return p.Queryer.QueryRowx(query, args...)
}
