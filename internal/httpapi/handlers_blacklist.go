package httpapi

import (
	"encoding/json"
	"net/http"

	"github.com/jfardello/tdns/config"
	"github.com/jfardello/tdns/internal/overrides"
	"github.com/jfardello/tdns/middleware"
)

// Toggle blacklist filtering.
//
//	@Summary		Toggle blacklist filtering
//	@Description	Enable or disable blacklist filtering.
//	@Tags			blacklist
//	@ID				blacklistToggle
//	@Param			action	path	string	true	"Requested state"	Enums(start,stop)
//	@Security		BearerAuth
//	@Success		200	{object}	api.Response
//	@Failure		401	{string}	string	"Unauthorized"
//	@Failure		403	{string}	string	"Forbidden"
//	@Failure		400	{object}	api.Response
//	@Failure		500	{object}	api.Response
//	@Router			/api/blacklist/{action} [post]
func (api *v1) BlacklistToggle(w http.ResponseWriter, r *http.Request) {
	p := api.server.Middlewares["blacklist"].(*middleware.BlackList)
	action := r.PathValue("action")
	state, err := actionToBool(action)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		writeJSON(Response{
			Kind:          BlacklistResponseKind,
			Message:       err.Error(),
			CurrentStatus: formatBool(p.IsEnabled()),
		}, w)
		return
	}

	st := api.server.BlacklistToggle(state)
	status, err := p.Status()
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		writeJSON(Response{
			Kind:          BlacklistResponseKind,
			Message:       err.Error(),
			CurrentStatus: formatBool(st),
		}, w)
		return
	}
	status.Enabled = st

	resp := Response{
		Kind:          BlacklistResponseKind,
		Message:       MESSAGE_OK,
		CurrentStatus: formatBool(st),
		Blacklist:     blacklistStatusDTO(status),
	}
	writeJSON(resp, w)

}

// Get blacklist status.
//
//	@Summary		Get blacklist status
//	@Description	Return blacklist sources, exclusions, and runtime state.
//	@Tags			blacklist
//	@ID				blacklistStatus
//	@Security		BearerAuth
//	@Success		200	{object}	api.Response
//	@Failure		401	{string}	string	"Unauthorized"
//	@Failure		403	{string}	string	"Forbidden"
//	@Failure		500	{object}	api.Response
//	@Router			/api/blacklist [get]
func (api *v1) BlacklistStatus(w http.ResponseWriter, r *http.Request) {
	p := api.server.Middlewares["blacklist"].(*middleware.BlackList)
	status, err := p.Status()
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		writeJSON(Response{
			Kind:          BlacklistResponseKind,
			Message:       err.Error(),
			CurrentStatus: formatBool(p.IsEnabled()),
		}, w)
		return
	}

	writeJSON(Response{
		Kind:          BlacklistResponseKind,
		Message:       MESSAGE_OK,
		CurrentStatus: formatBool(status.Enabled),
		Blacklist:     blacklistStatusDTO(status),
	}, w)
}

// Add runtime whitelist domains.
//
//	@Summary		Add runtime whitelist domains
//	@Description	Add domains to the in-memory blacklist whitelist.
//	@Tags			blacklist
//	@ID				blacklistRuntimeWhitelistAdd
//	@Param			request	body	api.BlacklistWhitelistRequest	true	"Runtime whitelist domains"
//	@Security		BearerAuth
//	@Success		200	{object}	api.Response
//	@Failure		401	{string}	string	"Unauthorized"
//	@Failure		403	{string}	string	"Forbidden"
//	@Failure		400	{object}	api.Response
//	@Failure		500	{object}	api.Response
//	@Router			/api/blacklist/whitelist [post]
func (api *v1) BlacklistAddRuntimeWhitelist(w http.ResponseWriter, r *http.Request) {
	req := &BlacklistWhitelistRequest{}
	if err := json.NewDecoder(r.Body).Decode(req); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		writeJSON(Response{
			Kind:          BlacklistResponseKind,
			Message:       err.Error(),
			CurrentStatus: formatBool(api.server.Middlewares["blacklist"].(*middleware.BlackList).IsEnabled()),
		}, w)
		return
	}

	p := api.server.Middlewares["blacklist"].(*middleware.BlackList)
	if err := p.AddRuntimeWhitelist(req.Domains); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		writeJSON(Response{
			Kind:          BlacklistResponseKind,
			Message:       err.Error(),
			CurrentStatus: formatBool(p.IsEnabled()),
		}, w)
		return
	}

	status, err := p.Status()
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		writeJSON(Response{
			Kind:          BlacklistResponseKind,
			Message:       err.Error(),
			CurrentStatus: formatBool(p.IsEnabled()),
		}, w)
		return
	}

	writeJSON(Response{
		Kind:          BlacklistResponseKind,
		Message:       MESSAGE_OK,
		CurrentStatus: formatBool(status.Enabled),
		Blacklist:     blacklistStatusDTO(status),
	}, w)
}

// Replace persisted blacklist hosts.
//
//	@Summary		Replace persisted blacklist hosts
//	@Description	Replace additional blocked hosts stored in configuration overrides.
//	@Tags			blacklist
//	@ID				blacklistPersistedHostsReplace
//	@Param			request	body	api.BlacklistHostsRequest	true	"Persisted blacklist hosts"
//	@Security		BearerAuth
//	@Success		200	{object}	api.Response
//	@Failure		401	{string}	string	"Unauthorized"
//	@Failure		403	{string}	string	"Forbidden"
//	@Failure		400	{object}	api.Response
//	@Failure		500	{object}	api.Response
//	@Router			/api/blacklist/persisted/hosts [post]
func (api *v1) BlacklistReplacePersistedHosts(w http.ResponseWriter, r *http.Request) {
	p := api.server.Middlewares["blacklist"].(*middleware.BlackList)
	req := &BlacklistHostsRequest{}
	if err := json.NewDecoder(r.Body).Decode(req); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		writeJSON(Response{
			Kind:          BlacklistResponseKind,
			Message:       err.Error(),
			CurrentStatus: formatBool(p.IsEnabled()),
		}, w)
		return
	}

	hosts := overrides.NormalizeDomains(req.Hosts)
	store, err := api.overrideStore(r.Context())
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		writeJSON(Response{
			Kind:          BlacklistResponseKind,
			Message:       err.Error(),
			CurrentStatus: formatBool(p.IsEnabled()),
		}, w)
		return
	}
	defer func() { _ = store.Close() }()

	if err := replaceOverrideValues(r.Context(), store, overrides.OverrideBlacklistHost, hosts); err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		writeJSON(Response{
			Kind:          BlacklistResponseKind,
			Message:       err.Error(),
			CurrentStatus: formatBool(p.IsEnabled()),
		}, w)
		return
	}

	conf := config.GetRunningConfig()
	conf.Blacklist.ExtraHosts = copyStrings(hosts)
	config.SetRunningConfig(conf)
	if err := p.ReplacePersistedHosts(hosts); err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		writeJSON(Response{
			Kind:          BlacklistResponseKind,
			Message:       err.Error(),
			CurrentStatus: formatBool(p.IsEnabled()),
		}, w)
		return
	}

	status, err := p.Status()
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		writeJSON(Response{
			Kind:          BlacklistResponseKind,
			Message:       err.Error(),
			CurrentStatus: formatBool(p.IsEnabled()),
		}, w)
		return
	}

	writeJSON(Response{
		Kind:          BlacklistResponseKind,
		Message:       MESSAGE_OK,
		CurrentStatus: formatBool(status.Enabled),
		Blacklist:     blacklistStatusDTO(status),
	}, w)
}

// Replace persisted blacklist exclusions.
//
//	@Summary		Replace persisted blacklist exclusions
//	@Description	Replace blacklist exclusion selectors stored in configuration overrides.
//	@Tags			blacklist
//	@ID				blacklistPersistedExcludesReplace
//	@Param			request	body	api.BlacklistExcludesRequest	true	"Persisted blacklist exclusions"
//	@Security		BearerAuth
//	@Success		200	{object}	api.Response
//	@Failure		401	{string}	string	"Unauthorized"
//	@Failure		403	{string}	string	"Forbidden"
//	@Failure		400	{object}	api.Response
//	@Failure		500	{object}	api.Response
//	@Router			/api/blacklist/persisted/excludes [post]
func (api *v1) BlacklistReplacePersistedExcludes(w http.ResponseWriter, r *http.Request) {
	p := api.server.Middlewares["blacklist"].(*middleware.BlackList)
	req := &BlacklistExcludesRequest{}
	if err := json.NewDecoder(r.Body).Decode(req); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		writeJSON(Response{
			Kind:          BlacklistResponseKind,
			Message:       err.Error(),
			CurrentStatus: formatBool(p.IsEnabled()),
		}, w)
		return
	}

	excludes := overrides.NormalizeSelectors(req.Excludes)
	store, err := api.overrideStore(r.Context())
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		writeJSON(Response{
			Kind:          BlacklistResponseKind,
			Message:       err.Error(),
			CurrentStatus: formatBool(p.IsEnabled()),
		}, w)
		return
	}
	defer func() { _ = store.Close() }()

	if err := replaceOverrideValues(r.Context(), store, overrides.OverrideBlacklistExclude, excludes); err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		writeJSON(Response{
			Kind:          BlacklistResponseKind,
			Message:       err.Error(),
			CurrentStatus: formatBool(p.IsEnabled()),
		}, w)
		return
	}

	conf := config.GetRunningConfig()
	conf.Blacklist.PersistedExcludes = copyStrings(excludes)
	config.SetRunningConfig(conf)
	if err := p.ReplacePersistedExcludes(excludes); err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		writeJSON(Response{
			Kind:          BlacklistResponseKind,
			Message:       err.Error(),
			CurrentStatus: formatBool(p.IsEnabled()),
		}, w)
		return
	}

	status, err := p.Status()
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		writeJSON(Response{
			Kind:          BlacklistResponseKind,
			Message:       err.Error(),
			CurrentStatus: formatBool(p.IsEnabled()),
		}, w)
		return
	}

	writeJSON(Response{
		Kind:          BlacklistResponseKind,
		Message:       MESSAGE_OK,
		CurrentStatus: formatBool(status.Enabled),
		Blacklist:     blacklistStatusDTO(status),
	}, w)
}
