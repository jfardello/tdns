package httpapi

import contractapi "github.com/jfardello/tdns/api"

const (
	MessageOK                = contractapi.MessageOK
	MESSAGE_OK               = MessageOK
	StubResolverResponseKind = contractapi.StubResolverResponseKind
	ZenModeResponseKind      = contractapi.ZenModeResponseKind
	BlacklistResponseKind    = contractapi.BlacklistResponseKind
	StaticResponseKind       = contractapi.StaticResponseKind
	DNSLogResponseKind       = contractapi.DNSLogResponseKind
	TaggerResponseKind       = contractapi.TaggerResponseKind
	CacheResponseKind        = contractapi.CacheResponseKind
)

type Response = contractapi.Response
type LogDetails = contractapi.LogDetails
type ClientCandidate = contractapi.ClientCandidate
type DashboardSummary = contractapi.DashboardSummary
type DashboardHourlyPoint = contractapi.DashboardHourlyPoint
type BlacklistStatus = contractapi.BlacklistStatus
type ZenModeStatus = contractapi.ZenModeStatus
type HostEntry = contractapi.HostEntry
type StaticResponseStatus = contractapi.StaticResponseStatus
type StubResolverStatus = contractapi.StubResolverStatus
type CacheStatus = contractapi.CacheStatus
type TagMember = contractapi.TagMember
type KnownHost = contractapi.KnownHost
type StubReplaceRequest = contractapi.StubReplaceRequest
type ZenReplaceRequest = contractapi.ZenReplaceRequest
type ZenExcludesRequest = contractapi.ZenExcludesRequest
type BlacklistWhitelistRequest = contractapi.BlacklistWhitelistRequest
type BlacklistHostsRequest = contractapi.BlacklistHostsRequest
type BlacklistExcludesRequest = contractapi.BlacklistExcludesRequest
type StaticReplaceRequest = contractapi.StaticReplaceRequest
type CacheExcludeRequest = contractapi.CacheExcludeRequest
type DNSLogAliasRequest = contractapi.DNSLogAliasRequest
type AddTagRequest = contractapi.AddTagRequest
type AddMemberRequest = contractapi.AddMemberRequest
type MemberLabelsRequest = contractapi.MemberLabelsRequest
type ReplaceMemberLabelsRequest = contractapi.ReplaceMemberLabelsRequest
