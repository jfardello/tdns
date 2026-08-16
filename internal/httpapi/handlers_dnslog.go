package httpapi

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"

	"github.com/jfardello/tdns/config"
	"github.com/jfardello/tdns/internal/overrides"
	"github.com/jfardello/tdns/middleware"
)

const dnsLogUnavailableMessage = "DNS-log middleware is unavailable"

func (api *v1) dnsLogMiddleware(w http.ResponseWriter) (*middleware.DNSLog, bool) {
	if api.server != nil {
		if dnsLog, ok := api.server.Middlewares["dns-log"].(*middleware.DNSLog); ok && dnsLog != nil {
			return dnsLog, true
		}
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusServiceUnavailable)
	writeJSON(Response{
		Kind:          DNSLogResponseKind,
		Message:       dnsLogUnavailableMessage,
		CurrentStatus: "Unavailable",
	}, w)
	return nil, false
}

// Get DNS-log status.
//
//	@Summary		Get DNS-log status
//	@Description	Return runtime state, queued events, and pseudonymization readiness.
//	@Tags			dns-log
//	@ID				dnsLogStatus
//	@Security		BearerAuth
//	@Security		CookieAuth
//	@Success		200	{object}	api.Response
//	@Failure		401	{string}	string	"Unauthorized"
//	@Failure		403	{string}	string	"Forbidden"
//	@Failure		503	{object}	api.Response
//	@Router			/api/dns-log [get]
func (api *v1) DNSLogStatus(w http.ResponseWriter, _ *http.Request) {
	p, ok := api.dnsLogMiddleware(w)
	if !ok {
		return
	}
	status := p.Status()
	writeJSON(Response{
		Kind:          DNSLogResponseKind,
		Message:       MESSAGE_OK,
		CurrentStatus: formatBool(status.Enabled),
		DNSLog:        dnsLogStatusDTO(status),
	}, w)
}

// Start or stop DNS logging.
//
//	@Summary		Start or stop DNS logging
//	@Description	Change the runtime state and persist it as a configuration override. Stop flushes all accepted events before returning.
//	@Tags			dns-log
//	@ID				dnsLogToggle
//	@Param			action	path	string	true	"Requested state"	Enums(start,stop)
//	@Security		BearerAuth
//	@Security		CookieAuth
//	@Success		200	{object}	api.Response
//	@Failure		400	{object}	api.Response
//	@Failure		401	{string}	string	"Unauthorized"
//	@Failure		403	{string}	string	"Forbidden"
//	@Failure		409	{object}	api.Response
//	@Failure		500	{object}	api.Response
//	@Failure		503	{object}	api.Response
//	@Router			/api/dns-log/{action} [post]
func (api *v1) DNSLogToggle(w http.ResponseWriter, r *http.Request) {
	api.dnsLogMutationMu.Lock()
	defer api.dnsLogMutationMu.Unlock()
	p, ok := api.dnsLogMiddleware(w)
	if !ok {
		return
	}
	state, err := actionToBool(r.PathValue("action"))
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		api.writeDNSLogStatus(w, p, err.Error())
		return
	}
	if state && p.Status().RequiresClear {
		w.WriteHeader(http.StatusConflict)
		api.writeDNSLogStatus(w, p, middleware.ErrDNSLogRequiresClear.Error())
		return
	}

	store, err := api.overrideStore(r.Context())
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		api.writeDNSLogStatus(w, p, err.Error())
		return
	}
	defer func() { _ = store.Close() }()
	if err := store.Upsert(r.Context(), overrides.OverrideDNSLogEnabled, overrides.OverrideSet, "enabled", strconv.FormatBool(state)); err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		api.writeDNSLogStatus(w, p, err.Error())
		return
	}

	if state {
		err = p.StartLogging()
	} else {
		err = p.StopLogging()
	}
	if err != nil {
		if state {
			_ = store.Upsert(r.Context(), overrides.OverrideDNSLogEnabled, overrides.OverrideSet, "enabled", "false")
		}
		w.WriteHeader(http.StatusInternalServerError)
		api.writeDNSLogStatus(w, p, err.Error())
		return
	}
	conf := config.GetRunningConfig()
	conf.DNSLog.Enabled = state
	config.SetRunningConfig(conf)
	api.writeDNSLogStatus(w, p, MESSAGE_OK)
}

// Clear all DNS-log data.
//
//	@Summary		Clear all DNS-log data
//	@Description	Delete events, dashboard aggregates, aliases, sequence state, and queued data. DNS logging must be stopped first.
//	@Tags			dns-log
//	@ID				dnsLogClear
//	@Security		BearerAuth
//	@Security		CookieAuth
//	@Success		200	{object}	api.Response
//	@Failure		401	{string}	string	"Unauthorized"
//	@Failure		403	{string}	string	"Forbidden"
//	@Failure		409	{object}	api.Response
//	@Failure		500	{object}	api.Response
//	@Failure		503	{object}	api.Response
//	@Router			/api/dns-log [delete]
func (api *v1) DNSLogClear(w http.ResponseWriter, _ *http.Request) {
	api.dnsLogMutationMu.Lock()
	defer api.dnsLogMutationMu.Unlock()
	p, ok := api.dnsLogMiddleware(w)
	if !ok {
		return
	}
	if err := p.Clear(); err != nil {
		if errors.Is(err, middleware.ErrDNSLogRunning) {
			w.WriteHeader(http.StatusConflict)
		} else {
			w.WriteHeader(http.StatusInternalServerError)
		}
		api.writeDNSLogStatus(w, p, err.Error())
		return
	}
	api.writeDNSLogStatus(w, p, MESSAGE_OK)
}

func (api *v1) writeDNSLogStatus(w http.ResponseWriter, dnsLog *middleware.DNSLog, message string) {
	status := dnsLog.Status()
	writeJSON(Response{
		Kind:          DNSLogResponseKind,
		Message:       message,
		CurrentStatus: formatBool(status.Enabled),
		DNSLog:        dnsLogStatusDTO(status),
	}, w)
}

// Set a DNS client alias.
//
//	@Summary		Set a DNS client alias
//	@Description	Associate a client IP address with a display name.
//	@Tags			dns-log
//	@ID				dnsLogAliasSet
//	@Param			request	body	api.DNSLogAliasRequest	true	"Client alias"
//	@Security		BearerAuth
//	@Security		CookieAuth
//	@Success		200	{object}	api.Response
//	@Failure		401	{string}	string	"Unauthorized"
//	@Failure		403	{string}	string	"Forbidden"
//	@Failure		503	{object}	api.Response
//	@Router			/api/dns-log/alias [post]
func (api *v1) DNSLogAlias(w http.ResponseWriter, r *http.Request) {
	p, ok := api.dnsLogMiddleware(w)
	if !ok {
		return
	}
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
//	@Security		CookieAuth
//	@Success		200	{object}	api.Response
//	@Failure		401	{string}	string	"Unauthorized"
//	@Failure		403	{string}	string	"Forbidden"
//	@Failure		503	{object}	api.Response
//	@Router			/api/dns-log/rotate [post]
func (api *v1) DNSLogRotate(w http.ResponseWriter, r *http.Request) {
	p, ok := api.dnsLogMiddleware(w)
	if !ok {
		return
	}
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
//	@Security		CookieAuth
//	@Success		200	{object}	api.Response
//	@Failure		401	{string}	string	"Unauthorized"
//	@Failure		403	{string}	string	"Forbidden"
//	@Failure		503	{object}	api.Response
//	@Router			/api/dns-log/top/{top} [get]
func (api *v1) DNSLogTop(w http.ResponseWriter, r *http.Request) {
	p, ok := api.dnsLogMiddleware(w)
	if !ok {
		return
	}
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
//	@Security		CookieAuth
//	@Success		200	{object}	api.Response
//	@Failure		401	{string}	string	"Unauthorized"
//	@Failure		403	{string}	string	"Forbidden"
//	@Failure		503	{object}	api.Response
//	@Router			/api/dns-log/clients [get]
func (api *v1) DNSLogClients(w http.ResponseWriter, r *http.Request) {
	p, ok := api.dnsLogMiddleware(w)
	if !ok {
		return
	}
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
//	@Security		CookieAuth
//	@Success		200	{object}	api.Response
//	@Failure		401	{string}	string	"Unauthorized"
//	@Failure		403	{string}	string	"Forbidden"
//	@Failure		503	{object}	api.Response
//	@Router			/api/dns-log/dashboard [get]
func (api *v1) DNSLogDashboard(w http.ResponseWriter, r *http.Request) {
	p, ok := api.dnsLogMiddleware(w)
	if !ok {
		return
	}
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

	api.writeDNSLogDashboard(w, stats)
}

// Get completed DNS log dashboard hours.
//
//	@Summary		Get completed dashboard hours
//	@Description	Return the cached statistics for the previous 23 completed UTC hours.
//	@Tags			dns-log
//	@ID				dnsLogDashboardHistory
//	@Security		BearerAuth
//	@Security		CookieAuth
//	@Success		200	{object}	api.Response
//	@Failure		401	{string}	string	"Unauthorized"
//	@Failure		403	{string}	string	"Forbidden"
//	@Failure		503	{object}	api.Response
//	@Router			/api/dns-log/dashboard/history [get]
func (api *v1) DNSLogDashboardHistory(w http.ResponseWriter, _ *http.Request) {
	p, ok := api.dnsLogMiddleware(w)
	if !ok {
		return
	}
	stats, err := p.GetDashboardHistory()
	if err != nil {
		api.writeDNSLogDashboardError(w, err)
		return
	}
	api.writeDNSLogDashboard(w, stats)
}

// Get the current DNS log dashboard hour.
//
//	@Summary		Get current dashboard hour
//	@Description	Calculate statistics for the current partial UTC hour.
//	@Tags			dns-log
//	@ID				dnsLogDashboardCurrent
//	@Security		BearerAuth
//	@Security		CookieAuth
//	@Success		200	{object}	api.Response
//	@Failure		401	{string}	string	"Unauthorized"
//	@Failure		403	{string}	string	"Forbidden"
//	@Failure		503	{object}	api.Response
//	@Router			/api/dns-log/dashboard/current [get]
func (api *v1) DNSLogDashboardCurrent(w http.ResponseWriter, _ *http.Request) {
	p, ok := api.dnsLogMiddleware(w)
	if !ok {
		return
	}
	stats, err := p.GetDashboardCurrent()
	if err != nil {
		api.writeDNSLogDashboardError(w, err)
		return
	}
	api.writeDNSLogDashboard(w, stats)
}

func (api *v1) writeDNSLogDashboardError(w http.ResponseWriter, err error) {
	writeJSON(Response{
		Kind:          DNSLogResponseKind,
		Message:       err.Error(),
		CurrentStatus: "Enabled",
	}, w)
}

func (api *v1) writeDNSLogDashboard(w http.ResponseWriter, stats *middleware.DashboardStats) {
	cacheStats := middleware.GetCache().Status()
	stats.Summary.CacheHits = cacheStats.Hits
	stats.Summary.CacheMisses = cacheStats.Misses
	w.Header().Set("Content-Type", "application/json")
	writeJSON(Response{
		Kind:          DNSLogResponseKind,
		Message:       MESSAGE_OK,
		CurrentStatus: "Enabled",
		WindowHours:   stats.WindowHours,
		Summary:       dashboardSummaryDTO(stats.Summary),
		Hourly:        dashboardHourlyDTOs(stats.Hourly),
	}, w)
}
