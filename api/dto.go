package api

const (
	MessageOK                = "Status OK"
	StubResolverResponseKind = "api.tdns/stub-resolver/response"
	ZenModeResponseKind      = "api.tdns/zen-mode/response"
	BlacklistResponseKind    = "api.tdns/blacklist/response"
	StaticResponseKind       = "api.tdns/static-response/response"
	DNSLogResponseKind       = "api.tdns/dns-log/response"
	TaggerResponseKind       = "api.tdns/tagger/response"
	CacheResponseKind        = "api.tdns/cache/response"
)

// MESSAGE_OK is retained for compatibility with existing API consumers.
const MESSAGE_OK = MessageOK

type Response struct {
	Kind          string                 `json:"kind" example:"api.tdns/cache/response"`
	Message       string                 `json:"message" example:"Status OK"`
	CurrentStatus string                 `json:"current_status" example:"enabled"`
	WindowHours   int                    `json:"window_hours,omitempty" minimum:"1" maximum:"336"`
	Items         []string               `json:"items,omitempty"`
	LogItems      []LogDetails           `json:"log_items,omitempty"`
	Summary       *DashboardSummary      `json:"summary,omitempty"`
	Hourly        []DashboardHourlyPoint `json:"hourly,omitempty"`
	Clients       []ClientCandidate      `json:"clients,omitempty"`
	Blacklist     *BlacklistStatus       `json:"blacklist,omitempty"`
	ZenMode       *ZenModeStatus         `json:"zen_mode,omitempty"`
	Static        *StaticResponseStatus  `json:"static_response,omitempty"`
	StubResolver  *StubResolverStatus    `json:"stub_resolver,omitempty"`
	Cache         *CacheStatus           `json:"cache,omitempty"`
	TagMembers    []TagMember            `json:"tag_members,omitempty"`
	KnownHosts    []KnownHost            `json:"known_hosts,omitempty"`
}

type LogDetails struct {
	Domain  string `json:"domain" example:"example.com."`
	Counter int    `json:"counter" example:"42" minimum:"0"`
	Host    string `json:"host" example:"office"`
}

type ClientCandidate struct {
	Address string `json:"address" example:"192.0.2.10"`
	Host    string `json:"host" example:"office"`
}

type DashboardSummary struct {
	TotalQueries   int   `json:"total_queries"`
	BlockedQueries int   `json:"blocked_queries"`
	AllowedQueries int   `json:"allowed_queries"`
	CacheHits      int64 `json:"cache_hits"`
	CacheMisses    int64 `json:"cache_misses"`
}

type DashboardHourlyPoint struct {
	HourBucket     int64  `json:"hour_bucket"`
	HourStart      string `json:"hour_start"`
	TotalQueries   int    `json:"total_queries"`
	BlockedQueries int    `json:"blocked_queries"`
	AllowedQueries int    `json:"allowed_queries"`
}

type BlacklistStatus struct {
	Enabled               bool     `json:"enabled"`
	File                  string   `json:"file"`
	ExternalFile          string   `json:"external_file,omitempty"`
	ExternalRepo          string   `json:"external_repo,omitempty"`
	ExternalRepoBranch    string   `json:"external_repo_branch,omitempty"`
	ExternalPullPeriod    string   `json:"external_pull_period,omitempty"`
	Excludes              []string `json:"excludes,omitempty"`
	PersistedExcludes     []string `json:"persisted_excludes,omitempty"`
	PersistedHosts        []string `json:"persisted_hosts,omitempty"`
	RuntimeWhitelist      []string `json:"runtime_whitelist,omitempty"`
	BlockfileTotalEntries int      `json:"blockfile_total_entries"`
}

type ZenModeStatus struct {
	Enabled            bool     `json:"enabled"`
	File               string   `json:"file,omitempty"`
	DurationMinutes    int      `json:"duration_minutes"`
	ConfiguredDomains  []string `json:"configured_domains,omitempty"`
	PersistedDomains   []string `json:"persisted_domains,omitempty"`
	ConfiguredExcludes []string `json:"configured_excludes,omitempty"`
	PersistedExcludes  []string `json:"persisted_excludes,omitempty"`
	Labels             []string `json:"labels,omitempty"`
	RuntimeDomains     []string `json:"runtime_domains,omitempty"`
	StartedAt          string   `json:"started_at,omitempty"`
	EndsAt             string   `json:"ends_at,omitempty"`
	RemainingSeconds   int64    `json:"remaining_seconds"`
}

type HostEntry struct {
	Domain  string `json:"domain" example:"example.com"`
	Address string `json:"address" example:"192.0.2.10"`
}

type StaticResponseStatus struct {
	Enabled         bool        `json:"enabled"`
	File            string      `json:"file,omitempty"`
	Labels          []string    `json:"labels,omitempty"`
	ConfiguredHosts []HostEntry `json:"configured_hosts,omitempty"`
	PersistedHosts  []HostEntry `json:"persisted_hosts,omitempty"`
	RuntimeHosts    []HostEntry `json:"runtime_hosts,omitempty"`
}

type StubResolverStatus struct {
	Enabled         bool     `json:"enabled"`
	ConfiguredStubs []string `json:"configured_stubs,omitempty"`
	RuntimeStubs    []string `json:"runtime_stubs,omitempty"`
}

type CacheStatus struct {
	Enabled  bool     `json:"enabled"`
	Ttl      int      `json:"ttl"`
	Excludes []string `json:"excludes,omitempty"`
	Hits     int64    `json:"hits"`
	Misses   int64    `json:"misses"`
}

type TagMember struct {
	Address      string `json:"address" example:"192.0.2.10"`
	Host         string `json:"host,omitempty" example:"office"`
	HasHostAlias bool   `json:"has_host_alias"`
}

type KnownHost struct {
	Address string `json:"address" example:"192.0.2.10"`
	Host    string `json:"host" example:"office"`
}

type StubReplaceRequest struct {
	Stubs []string `json:"stubs"`
}

type ZenReplaceRequest struct {
	ZenDomains []string `json:"zen_domains"`
}

type ZenExcludesRequest struct {
	Excludes []string `json:"excludes"`
}

type BlacklistWhitelistRequest struct {
	Domains []string `json:"domains"`
}

type BlacklistHostsRequest struct {
	Hosts []string `json:"hosts"`
}

type BlacklistExcludesRequest struct {
	Excludes []string `json:"excludes"`
}

type StaticReplaceRequest struct {
	Hosts []string `json:"hosts"`
}

type CacheExcludeRequest struct {
	Excludes []string `json:"excludes"`
}

type DNSLogAliasRequest struct {
	Name string `json:"name" example:"office"`
	Addr string `json:"addr" example:"192.0.2.10"`
}

type AddTagRequest struct {
	Name string `json:"name" example:"trusted"`
}

type AddMemberRequest struct {
	Members []string `json:"members"`
}

type MemberLabelsRequest struct {
	Address string   `json:"address" example:"192.0.2.10"`
	Tags    []string `json:"tags"`
}

type ReplaceMemberLabelsRequest struct {
	Tags []string `json:"tags"`
}

type BrowserCodeExchangeRequest struct {
	Code string `json:"code"`
}

type BrowserPasswordLoginRequest struct {
	Username string `json:"username" example:"admin"`
	Password string `json:"password" format:"password"`
}

type BrowserSessionResponse struct {
	Subject   string `json:"subject"`
	Scope     string `json:"scope"`
	ExpiresAt string `json:"expires_at" format:"date-time"`
	CSRFToken string `json:"csrf_token"`
}

type ErrorResponse struct {
	Error string `json:"error"`
}
