import type { components } from '../generated/api'

type Schema<Name extends keyof components['schemas']> = components['schemas'][Name]

export type DashboardSummary = Required<Schema<'api.DashboardSummary'>>
export type DashboardHourlyPoint = Required<Schema<'api.DashboardHourlyPoint'>>

export interface DashboardSegment {
  window_hours: number
  summary: DashboardSummary
  hourly: DashboardHourlyPoint[]
}

export function dashboardSegmentsHaveGap(
  history: DashboardSegment,
  current: DashboardSegment
): boolean {
  const lastHistory = history.hourly.at(-1)
  const firstCurrent = current.hourly[0]
  return Boolean(
    lastHistory
    && firstCurrent
    && firstCurrent.hour_bucket > lastHistory.hour_bucket + 1
  )
}

export function mergeDashboardSegments(
  history: DashboardSegment,
  current?: DashboardSegment
): DashboardSegment {
  const points = new Map<number, DashboardHourlyPoint>()
  for (const point of [...history.hourly, ...(current?.hourly ?? [])]) {
    points.set(point.hour_bucket, point)
  }
  const hourly = [...points.values()].sort((left, right) => left.hour_bucket - right.hour_bucket)
  const sourceSummary = current?.summary ?? history.summary
  const summary = hourly.reduce<DashboardSummary>((result, point) => {
    result.total_queries += point.total_queries
    result.blocked_queries += point.blocked_queries
    result.allowed_queries += point.allowed_queries
    return result
  }, {
    total_queries: 0,
    blocked_queries: 0,
    allowed_queries: 0,
    cache_hits: sourceSummary.cache_hits,
    cache_misses: sourceSummary.cache_misses
  })

  return {
    window_hours: history.window_hours + (current ? current.window_hours : 0),
    summary,
    hourly
  }
}
