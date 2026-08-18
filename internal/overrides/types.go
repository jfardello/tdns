package overrides

import "time"

type Kind int

const (
	OverrideStaticHost Kind = iota + 1
	OverrideZenDomain
	OverrideZenExclude
	OverrideBlacklistHost
	OverrideBlacklistExclude
	OverrideCacheEnabled
	OverrideCacheExclude
	OverrideDNSLogEnabled
	OverrideDNSLogDomainsPseudonymized
	OverrideDNSLogClientsPseudonymized
)

type Op int

const (
	OverrideUpsert Op = iota + 1
	OverrideDelete
	OverrideSet
)

type Row struct {
	ID        int64
	Kind      Kind
	Op        Op
	Target    string
	Value     string
	CreatedAt time.Time
	UpdatedAt time.Time
}
