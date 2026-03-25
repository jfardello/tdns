import dns from 'k6/x/dns';
import exec from 'k6/execution';
import { SharedArray } from 'k6/data';
import { Counter, Trend } from 'k6/metrics';

function parseNumberEnv(name, fallback) {
    const raw = __ENV[name];
    if (!raw) {
        return fallback;
    }
    const parsed = Number(raw);
    if (!Number.isFinite(parsed) || parsed <= 0) {
        throw new Error(`${name} must be a positive number, got ${raw}`);
    }
    return parsed;
}

function metricValue(data, name, key, fallback = 0) {
    const metric = data.metrics[name];
    if (!metric || !metric.values || metric.values[key] === undefined) {
        return fallback;
    }
    return metric.values[key];
}

function formatMetric(value, digits = 2) {
    if (!Number.isFinite(value)) {
        return '0';
    }
    return value.toFixed(digits);
}

const domainsByGroup = new SharedArray('domains', () => JSON.parse(open('./data.json')).test);
const domains = [];
for (const group of domainsByGroup) {
    for (const domain of group.domains) {
        domains.push(domain);
    }
}

if (domains.length === 0) {
    throw new Error('test/data.json does not contain any benchmark domains');
}

const target = __ENV.TDNS_BENCH_SERVER || '127.0.0.1:8053';
const queryType = __ENV.TDNS_BENCH_RECORD_TYPE || 'A';
const targetQps = parseNumberEnv('TDNS_BENCH_RATE', 250);
const duration = __ENV.TDNS_BENCH_DURATION || '2m';
const preAllocatedVUs = parseNumberEnv('TDNS_BENCH_PREALLOCATED_VUS', Math.max(50, targetQps));
const maxVUs = parseNumberEnv('TDNS_BENCH_MAX_VUS', Math.max(preAllocatedVUs * 2, targetQps * 2));

const dnsQueryDuration = new Trend('dns_query_duration', true);
const dnsQuerySuccess = new Counter('dns_query_success');
const dnsQueryError = new Counter('dns_query_error');

export const options = {
    scenarios: {
        dns_benchmark: {
            executor: 'constant-arrival-rate',
            rate: targetQps,
            timeUnit: '1s',
            duration: duration,
            preAllocatedVUs: preAllocatedVUs,
            maxVUs: maxVUs,
        },
    },
    summaryTrendStats: ['avg', 'med', 'p(95)', 'p(99)', 'min', 'max'],
};

export default async function () {
    const iteration = exec.scenario.iterationInTest;
    const domain = domains[iteration % domains.length];
    const started = Date.now();

    try {
        const answer = await dns.resolve(domain, queryType, target);
        dnsQueryDuration.add(Date.now() - started);
        if (!answer || answer.length === 0) {
            dnsQueryError.add(1);
            return;
        }
        dnsQuerySuccess.add(1);
    } catch (error) {
        dnsQueryDuration.add(Date.now() - started);
        dnsQueryError.add(1);
    }
}

export function handleSummary(data) {
    const successCount = metricValue(data, 'dns_query_success', 'count');
    const errorCount = metricValue(data, 'dns_query_error', 'count');
    const achievedQps = metricValue(data, 'dns_query_success', 'rate');
    const iterationRate = metricValue(data, 'iterations', 'rate');
    const avgMs = metricValue(data, 'dns_query_duration', 'avg');
    const medMs = metricValue(data, 'dns_query_duration', 'med');
    const p95Ms = metricValue(data, 'dns_query_duration', 'p(95)');
    const p99Ms = metricValue(data, 'dns_query_duration', 'p(99)');
    const droppedIterations = metricValue(data, 'dropped_iterations', 'count');

    const humanSummary = [
        'TDNS sqlite benchmark summary',
        `target           : ${target}`,
        `record type      : ${queryType}`,
        `configured rate  : ${targetQps.toFixed(2)} qps`,
        `successful rate  : ${formatMetric(achievedQps)} qps`,
        `iteration rate   : ${formatMetric(iterationRate)} qps`,
        `successes        : ${formatMetric(successCount, 0)}`,
        `errors           : ${formatMetric(errorCount, 0)}`,
        `dropped iters    : ${formatMetric(droppedIterations, 0)}`,
        `latency avg      : ${formatMetric(avgMs)} ms`,
        `latency median   : ${formatMetric(medMs)} ms`,
        `latency p95      : ${formatMetric(p95Ms)} ms`,
        `latency p99      : ${formatMetric(p99Ms)} ms`,
    ].join('\n') + '\n';

    const envSummary = [
        `TARGET=${target}`,
        `QUERY_TYPE=${queryType}`,
        `TARGET_QPS=${targetQps.toFixed(6)}`,
        `SUCCESS_COUNT=${formatMetric(successCount, 0)}`,
        `ERROR_COUNT=${formatMetric(errorCount, 0)}`,
        `DROPPED_ITERATIONS=${formatMetric(droppedIterations, 0)}`,
        `ACHIEVED_QPS=${formatMetric(achievedQps, 6)}`,
        `ITERATION_RATE=${formatMetric(iterationRate, 6)}`,
        `AVG_MS=${formatMetric(avgMs, 6)}`,
        `MED_MS=${formatMetric(medMs, 6)}`,
        `P95_MS=${formatMetric(p95Ms, 6)}`,
        `P99_MS=${formatMetric(p99Ms, 6)}`,
    ].join('\n') + '\n';

    const outputs = {};
    if ((__ENV.TDNS_BENCH_STDOUT_SUMMARY || 'true').toLowerCase() !== 'false') {
        outputs.stdout = humanSummary;
    }
    if (__ENV.TDNS_BENCH_SUMMARY_TXT) {
        outputs[__ENV.TDNS_BENCH_SUMMARY_TXT] = humanSummary;
    }
    if (__ENV.TDNS_BENCH_SUMMARY_ENV) {
        outputs[__ENV.TDNS_BENCH_SUMMARY_ENV] = envSummary;
    }
    if (__ENV.TDNS_BENCH_SUMMARY_JSON) {
        outputs[__ENV.TDNS_BENCH_SUMMARY_JSON] = JSON.stringify(data, null, 2);
    }
    return outputs;
}
