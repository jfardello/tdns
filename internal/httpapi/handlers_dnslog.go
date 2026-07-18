package httpapi

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/jfardello/tdns/middleware"
)

// Set a DNS client alias.
//
//	@Summary		Set a DNS client alias
//	@Description	Associate a client IP address with a display name.
//	@Tags			dns-log
//	@ID				dnsLogAliasSet
//	@Param			request	body	api.DNSLogAliasRequest	true	"Client alias"
//	@Security		BearerAuth
//	@Success		200	{object}	api.Response
//	@Failure		401	{string}	string	"Unauthorized"
//	@Router			/api/dns-log/alias [post]
func (api *v1) DNSLogAlias(w http.ResponseWriter, r *http.Request) {
	p := api.server.Middlewares["dns-log"].(*middleware.DNSLog)
	w.Header().Set("Content-Type", "application/json")
	jr := new(DNSLogAliasRequest)
	err := json.NewDecoder(r.Body).Decode(jr)
	if err != nil {
		writeJSON(Response{
			Kind:          DNSLogResponseKind,
			Message:       err.Error(),
			CurrentStatus: "Enabled",
		}, w)
		return
	}
	err = p.AddAlias(jr.Name, jr.Addr)
	if err != nil {
		writeJSON(Response{
			Kind:          DNSLogResponseKind,
			Message:       err.Error(),
			CurrentStatus: "Enabled",
		}, w)
		return
	}

	res := Response{
		Kind:          DNSLogResponseKind,
		Message:       MESSAGE_OK,
		CurrentStatus: "Enabled",
	}
	writeJSON(res, w)
}

// Rotate DNS log entries.
//
//	@Summary		Rotate DNS log entries
//	@Description	Delete DNS log entries selected by a relative duration.
//	@Tags			dns-log
//	@ID				dnsLogRotate
//	@Param			since	query	string	false	"Relative age such as 24h or 1w"
//	@Security		BearerAuth
//	@Success		200	{object}	api.Response
//	@Failure		401	{string}	string	"Unauthorized"
//	@Router			/api/dns-log/rotate [get]
func (api *v1) DNSLogRotate(w http.ResponseWriter, r *http.Request) {
	p := api.server.Middlewares["dns-log"].(*middleware.DNSLog)
	w.Header().Set("Content-Type", "application/json")

	since := r.URL.Query().Get("since")

	err := p.Rotate(since)
	if err != nil {
		writeJSON(Response{
			Kind:          DNSLogResponseKind,
			Message:       err.Error(),
			CurrentStatus: "Enabled",
		}, w)
		return
	}
	res := Response{
		Kind:          DNSLogResponseKind,
		Message:       MESSAGE_OK,
		CurrentStatus: "Rotate OK",
	}
	writeJSON(res, w)
}

// Get top queried domains.
//
//	@Summary		Get top queried domains
//	@Description	Return top domains with optional status and client filters.
//	@Tags			dns-log
//	@ID				dnsLogTop
//	@Param			top			path	int		true	"Maximum result count"	minimum(1)	maximum(50)
//	@Param			since		query	string	false	"Relative age such as 24h or 1w"
//	@Param			status		query	string	false	"Query disposition"	Enums(blocked,allowed)
//	@Param			client		query	string	false	"Client alias or IP address"
//	@Param			client_mode	query	string	false	"Client matching mode"	Enums(host,ip)
//	@Security		BearerAuth
//	@Success		200	{object}	api.Response
//	@Failure		401	{string}	string	"Unauthorized"
//	@Router			/api/dns-log/top/{top} [get]
func (api *v1) DNSLogTop(w http.ResponseWriter, r *http.Request) {
	p := api.server.Middlewares["dns-log"].(*middleware.DNSLog)
	w.Header().Set("Content-Type", "application/json")
	top, err := strconv.Atoi(r.PathValue("top"))
	since := r.URL.Query().Get("since")
	status := r.URL.Query().Get("status")
	client := r.URL.Query().Get("client")
	clientMode := r.URL.Query().Get("client_mode")

	if err != nil {
		writeJSON(Response{
			Kind:          DNSLogResponseKind,
			Message:       err.Error(),
			CurrentStatus: "Enabled",
		}, w)
		return
	}
	items, err := p.GetTop(top, since, status, client, clientMode)
	if err != nil {
		writeJSON(Response{
			Kind:          DNSLogResponseKind,
			Message:       err.Error(),
			CurrentStatus: "Enabled",
		}, w)
		return
	}
	res := Response{
		Kind:          DNSLogResponseKind,
		Message:       MESSAGE_OK,
		CurrentStatus: "Enabled",
		LogItems:      logDetailsDTOs(items)}
	writeJSON(res, w)
}

// Search DNS clients.
//
//	@Summary		Search DNS clients
//	@Description	Search observed clients by address or alias.
//	@Tags			dns-log
//	@ID				dnsLogClientsSearch
//	@Param			search	query	string	false	"Address or alias substring"
//	@Param			limit	query	int		false	"Maximum result count"	default(20)	minimum(1)	maximum(100)
//	@Security		BearerAuth
//	@Success		200	{object}	api.Response
//	@Failure		401	{string}	string	"Unauthorized"
//	@Router			/api/dns-log/clients [get]
func (api *v1) DNSLogClients(w http.ResponseWriter, r *http.Request) {
	p := api.server.Middlewares["dns-log"].(*middleware.DNSLog)
	w.Header().Set("Content-Type", "application/json")

	search := r.URL.Query().Get("search")
	limit := 20
	if raw := r.URL.Query().Get("limit"); raw != "" {
		parsed, err := strconv.Atoi(raw)
		if err != nil {
			writeJSON(Response{
				Kind:          DNSLogResponseKind,
				Message:       err.Error(),
				CurrentStatus: "Enabled",
			}, w)
			return
		}
		limit = parsed
	}

	items, err := p.SearchClients(search, limit)
	if err != nil {
		writeJSON(Response{
			Kind:          DNSLogResponseKind,
			Message:       err.Error(),
			CurrentStatus: "Enabled",
		}, w)
		return
	}

	writeJSON(Response{
		Kind:          DNSLogResponseKind,
		Message:       MESSAGE_OK,
		CurrentStatus: "Enabled",
		Clients:       clientCandidateDTOs(items),
	}, w)
}

// Get DNS log dashboard.
//
//	@Summary		Get DNS log dashboard
//	@Description	Return summary and hourly query statistics.
//	@Tags			dns-log
//	@ID				dnsLogDashboard
//	@Param			hours	query	int	false	"Dashboard window in hours"	default(24)	minimum(1)	maximum(336)
//	@Security		BearerAuth
//	@Success		200	{object}	api.Response
//	@Failure		401	{string}	string	"Unauthorized"
//	@Router			/api/dns-log/dashboard [get]
func (api *v1) DNSLogDashboard(w http.ResponseWriter, r *http.Request) {
	p := api.server.Middlewares["dns-log"].(*middleware.DNSLog)
	w.Header().Set("Content-Type", "application/json")

	hours := 24
	if raw := r.URL.Query().Get("hours"); raw != "" {
		parsed, err := strconv.Atoi(raw)
		if err != nil {
			writeJSON(Response{
				Kind:          DNSLogResponseKind,
				Message:       err.Error(),
				CurrentStatus: "Enabled",
			}, w)
			return
		}
		hours = parsed
	}

	stats, err := p.GetDashboardStats(hours)
	if err != nil {
		writeJSON(Response{
			Kind:          DNSLogResponseKind,
			Message:       err.Error(),
			CurrentStatus: "Enabled",
		}, w)
		return
	}

	cacheStats := middleware.GetCache().Status()
	stats.Summary.CacheHits = cacheStats.Hits
	stats.Summary.CacheMisses = cacheStats.Misses

	writeJSON(Response{
		Kind:          DNSLogResponseKind,
		Message:       MESSAGE_OK,
		CurrentStatus: "Enabled",
		WindowHours:   stats.WindowHours,
		Summary:       dashboardSummaryDTO(stats.Summary),
		Hourly:        dashboardHourlyDTOs(stats.Hourly),
	}, w)
}
