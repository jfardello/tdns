import assert from 'node:assert/strict'
import test from 'node:test'

import {
  dashboardSegmentsHaveGap,
  mergeDashboardSegments,
  type DashboardSegment
} from './dashboardStats.ts'

function segment(hour: number, total: number, blocked: number): DashboardSegment {
  return {
    window_hours: 1,
    summary: {
      total_queries: total,
      blocked_queries: blocked,
      allowed_queries: total - blocked,
      cache_hits: hour,
      cache_misses: hour + 1
    },
    hourly: [{
      hour_bucket: hour,
      hour_start: `hour-${hour}`,
      total_queries: total,
      blocked_queries: blocked,
      allowed_queries: total - blocked
    }]
  }
}

test('renders cached history before the current hour is available', () => {
  const history = segment(10, 5, 2)
  assert.deepEqual(mergeDashboardSegments(history), history)
})

test('merges and orders the current hour without duplicating buckets', () => {
  const history = segment(10, 5, 2)
  const current = segment(11, 3, 1)
  const merged = mergeDashboardSegments(history, current)

  assert.equal(merged.window_hours, 2)
  assert.deepEqual(merged.hourly.map(point => point.hour_bucket), [10, 11])
  assert.deepEqual(merged.summary, {
    total_queries: 8,
    blocked_queries: 3,
    allowed_queries: 5,
    cache_hits: 11,
    cache_misses: 12
  })

  const replaced = mergeDashboardSegments(history, segment(10, 7, 4))
  assert.equal(replaced.hourly.length, 1)
  assert.equal(replaced.summary.total_queries, 7)
})

test('detects an hour boundary crossed between history and current requests', () => {
  assert.equal(dashboardSegmentsHaveGap(segment(10, 1, 0), segment(11, 1, 0)), false)
  assert.equal(dashboardSegmentsHaveGap(segment(10, 1, 0), segment(12, 1, 0)), true)
})
