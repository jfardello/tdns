package middleware

import (
	"time"

	"github.com/jfardello/tdns/internal/sqliteutil"
	"github.com/jfardello/tdns/log"
	"github.com/jfardello/tdns/syncsqlite"
	"github.com/jmoiron/sqlx"
)

const (
	dashboardHistoryHours = 23
	dashboardWindowHours  = dashboardHistoryHours + 1
	dashboardMaxHours     = 24 * 14
	nanosecondsPerHour    = int64(time.Hour)
)

func dashboardHourBucket(now time.Time) int64 {
	return now.UTC().Unix() / int64(time.Hour/time.Second)
}

func dashboardHourStart(bucket int64) string {
	return time.Unix(bucket*int64(time.Hour/time.Second), 0).Local().Format("2006-01-02 15:04:05")
}

func dashboardSummary(points []DashboardHourlyPoint) DashboardSummary {
	var summary DashboardSummary
	for _, point := range points {
		summary.TotalQueries += point.TotalQueries
		summary.BlockedQueries += point.BlockedQueries
		summary.AllowedQueries += point.AllowedQueries
	}
	return summary
}

func (cs *DNSLog) queryDashboardBuckets(startBucket, endBucket int64) ([]DashboardHourlyPoint, error) {
	logger := log.GetLogger("DNSLog", "queryDashboardBuckets")
	rows := make([]DashboardHourlyPoint, 0, endBucket-startBucket)
	query := `
SELECT
	(dt / ?) AS hour_bucket,
	COUNT(*) AS total_queries,
	COALESCE(SUM(CASE WHEN blocked = 1 THEN 1 ELSE 0 END), 0) AS blocked_queries,
	COUNT(*) - COALESCE(SUM(CASE WHEN blocked = 1 THEN 1 ELSE 0 END), 0) AS allowed_queries
FROM tdnslog
WHERE dt >= ? AND dt < ?
GROUP BY (dt / ?)
ORDER BY hour_bucket`

	db := cs.se.GetConn()
	defer cs.se.FreeConn(db)
	dbx := sqlx.NewDb(db, sqliteutil.DriverName())
	dbl := &log.SQLLogger{Queryer: dbx, Logger: logger, DebugSql: log.IsDebugEnabled()}
	err := sqlx.Select(
		dbl,
		&rows,
		query,
		nanosecondsPerHour,
		startBucket*nanosecondsPerHour,
		endBucket*nanosecondsPerHour,
		nanosecondsPerHour,
	)
	if err != nil {
		return nil, err
	}

	byBucket := make(map[int64]DashboardHourlyPoint, len(rows))
	for _, row := range rows {
		byBucket[row.HourBucket] = row
	}
	points := make([]DashboardHourlyPoint, 0, endBucket-startBucket)
	for bucket := startBucket; bucket < endBucket; bucket++ {
		point := byBucket[bucket]
		point.HourBucket = bucket
		point.HourStart = dashboardHourStart(bucket)
		points = append(points, point)
	}
	return points, nil
}

func (cs *DNSLog) readDashboardCache(startBucket, endBucket int64) ([]DashboardHourlyPoint, error) {
	logger := log.GetLogger("DNSLog", "readDashboardCache")
	points := make([]DashboardHourlyPoint, 0, endBucket-startBucket)
	query := `
SELECT hour_bucket, total_queries, blocked_queries, allowed_queries
FROM dashboard_hourly_stats
WHERE hour_bucket >= ? AND hour_bucket < ?
ORDER BY hour_bucket`

	db := cs.se.GetConn()
	defer cs.se.FreeConn(db)
	dbx := sqlx.NewDb(db, sqliteutil.DriverName())
	dbl := &log.SQLLogger{Queryer: dbx, Logger: logger, DebugSql: log.IsDebugEnabled()}
	err := sqlx.Select(dbl, &points, query, startBucket, endBucket)
	if err != nil {
		return nil, err
	}
	for i := range points {
		points[i].HourStart = dashboardHourStart(points[i].HourBucket)
	}
	return points, nil
}

func dashboardCacheComplete(points []DashboardHourlyPoint, startBucket, endBucket int64) bool {
	if len(points) != int(endBucket-startBucket) {
		return false
	}
	for i, point := range points {
		if point.HourBucket != startBucket+int64(i) {
			return false
		}
	}
	return true
}

func dashboardCacheStatements(points []DashboardHourlyPoint, startBucket, endBucket int64, now time.Time) []*syncsqlite.ExecStmt {
	stmts := make([]*syncsqlite.ExecStmt, 0, len(points)+1)
	for _, point := range points {
		stmts = append(stmts, &syncsqlite.ExecStmt{
			Query: `
INSERT INTO dashboard_hourly_stats
	(hour_bucket, total_queries, blocked_queries, allowed_queries, computed_at)
VALUES (?, ?, ?, ?, ?)
ON CONFLICT(hour_bucket) DO UPDATE SET
	total_queries = excluded.total_queries,
	blocked_queries = excluded.blocked_queries,
	allowed_queries = excluded.allowed_queries,
	computed_at = excluded.computed_at`,
			Args: []any{
				point.HourBucket,
				point.TotalQueries,
				point.BlockedQueries,
				point.AllowedQueries,
				now.UTC().Unix(),
			},
		})
	}
	stmts = append(stmts, &syncsqlite.ExecStmt{
		Query: "DELETE FROM dashboard_hourly_stats WHERE hour_bucket < ? OR hour_bucket >= ?",
		Args:  []any{startBucket, endBucket},
	})
	return stmts
}

func (cs *DNSLog) loadDashboardHistoryAt(now time.Time) ([]DashboardHourlyPoint, error) {
	currentBucket := dashboardHourBucket(now)
	startBucket := currentBucket - dashboardHistoryHours
	writeCache := cs.IsEnabled()

	cs.dashboardMu.Lock()
	defer cs.dashboardMu.Unlock()

	points, err := cs.readDashboardCache(startBucket, currentBucket)
	if err != nil {
		return nil, err
	}
	if dashboardCacheComplete(points, startBucket, currentBucket) {
		return points, nil
	}

	points, err = cs.queryDashboardBuckets(startBucket, currentBucket)
	if err != nil {
		return nil, err
	}
	if !writeCache {
		return points, nil
	}
	if err := cs.se.SyncExecBulk(dashboardCacheStatements(points, startBucket, currentBucket, now)); err != nil {
		return nil, err
	}
	return points, nil
}

func (cs *DNSLog) refreshDashboardCacheAt(now time.Time) error {
	currentBucket := dashboardHourBucket(now)
	startBucket := currentBucket - dashboardHistoryHours

	cs.dashboardMu.Lock()
	defer cs.dashboardMu.Unlock()

	points, err := cs.readDashboardCache(startBucket, currentBucket)
	if err != nil {
		return err
	}
	if dashboardCacheComplete(points, startBucket, currentBucket) {
		points, err = cs.queryDashboardBuckets(currentBucket-1, currentBucket)
	} else {
		points, err = cs.queryDashboardBuckets(startBucket, currentBucket)
	}
	if err != nil {
		return err
	}
	return cs.se.SyncExecBulk(dashboardCacheStatements(points, startBucket, currentBucket, now))
}

func (cs *DNSLog) clearDashboardCacheLocked() error {
	_, err := cs.se.SyncExec("DELETE FROM dashboard_hourly_stats", nil)
	return err
}

func (cs *DNSLog) GetDashboardHistoryAt(now time.Time) (*DashboardStats, error) {
	points, err := cs.loadDashboardHistoryAt(now)
	if err != nil {
		return nil, err
	}
	return &DashboardStats{
		WindowHours: dashboardHistoryHours,
		Summary:     dashboardSummary(points),
		Hourly:      points,
	}, nil
}

func (cs *DNSLog) GetDashboardHistory() (*DashboardStats, error) {
	return cs.GetDashboardHistoryAt(time.Now())
}

func (cs *DNSLog) GetDashboardCurrentAt(now time.Time) (*DashboardStats, error) {
	currentBucket := dashboardHourBucket(now)
	points, err := cs.queryDashboardBuckets(currentBucket, currentBucket+1)
	if err != nil {
		return nil, err
	}
	return &DashboardStats{WindowHours: 1, Summary: dashboardSummary(points), Hourly: points}, nil
}

func (cs *DNSLog) GetDashboardCurrent() (*DashboardStats, error) {
	return cs.GetDashboardCurrentAt(time.Now())
}

func (cs *DNSLog) GetDashboardStatsAt(now time.Time, hours int) (*DashboardStats, error) {
	if hours <= 0 {
		hours = dashboardWindowHours
	}
	if hours > dashboardMaxHours {
		hours = dashboardMaxHours
	}
	if hours == dashboardWindowHours {
		history, err := cs.GetDashboardHistoryAt(now)
		if err != nil {
			return nil, err
		}
		current, err := cs.GetDashboardCurrentAt(now)
		if err != nil {
			return nil, err
		}
		points := append(history.Hourly, current.Hourly...)
		return &DashboardStats{WindowHours: hours, Summary: dashboardSummary(points), Hourly: points}, nil
	}

	currentBucket := dashboardHourBucket(now)
	points, err := cs.queryDashboardBuckets(currentBucket-int64(hours-1), currentBucket+1)
	if err != nil {
		return nil, err
	}
	return &DashboardStats{WindowHours: hours, Summary: dashboardSummary(points), Hourly: points}, nil
}

func (cs *DNSLog) GetDashboardStats(hours int) (*DashboardStats, error) {
	return cs.GetDashboardStatsAt(time.Now(), hours)
}
