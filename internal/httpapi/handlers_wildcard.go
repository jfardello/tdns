package httpapi

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/jfardello/tdns/config"
	"github.com/jfardello/tdns/internal/overrides"
	"github.com/jfardello/tdns/middleware"
)

const wildcardUnavailableMessage = "wildcard middleware is not configured"

// Get wildcard DNS status.
//
//	@Summary		Get wildcard DNS status
//	@Description	Return the wildcard middleware state and configured domains.
//	@Tags			wildcard
//	@ID				wildcardStatus
//	@Security		BearerAuth
//	@Security		CookieAuth
//	@Success		200	{object}	api.Response
//	@Failure		401	{string}	string	"Unauthorized"
//	@Failure		403	{string}	string	"Forbidden"
//	@Failure		503	{object}	api.Response
//	@Router			/api/wildcard [get]
func (api *v1) WildcardStatus(w http.ResponseWriter, _ *http.Request) {
	wildcard, ok := api.wildcardMiddleware(w)
	if !ok {
		return
	}
	api.writeWildcardStatus(wildcard, w)
}

// Toggle wildcard DNS resolution.
//
//	@Summary		Toggle wildcard DNS resolution
//	@Description	Enable or disable wildcard DNS resolution and persist the selected state.
//	@Tags			wildcard
//	@ID				wildcardToggle
//	@Param			action	path	string	true	"Requested state"	Enums(start,stop)
//	@Security		BearerAuth
//	@Security		CookieAuth
//	@Success		200	{object}	api.Response
//	@Failure		400	{object}	api.Response
//	@Failure		401	{string}	string	"Unauthorized"
//	@Failure		403	{string}	string	"Forbidden"
//	@Failure		500	{object}	api.Response
//	@Failure		503	{object}	api.Response
//	@Router			/api/wildcard/{action} [post]
func (api *v1) WildcardToggle(w http.ResponseWriter, r *http.Request) {
	wildcard, ok := api.wildcardMiddleware(w)
	if !ok {
		return
	}
	state, err := actionToBool(r.PathValue("action"))
	if err != nil {
		writeWildcardError(w, http.StatusBadRequest, err, wildcard)
		return
	}

	store, err := api.overrideStore(r.Context())
	if err != nil {
		writeWildcardError(w, http.StatusInternalServerError, err, wildcard)
		return
	}
	defer func() { _ = store.Close() }()
	if err := store.Upsert(r.Context(), overrides.OverrideWildcardEnabled, overrides.OverrideSet, "enabled", strconv.FormatBool(state)); err != nil {
		writeWildcardError(w, http.StatusInternalServerError, err, wildcard)
		return
	}

	wildcard.SetEnabled(state)
	conf := config.GetRunningConfig()
	conf.Wildcard.Enabled = state
	config.SetRunningConfig(conf)
	api.writeWildcardStatus(wildcard, w)
}

// Replace enabled wildcard domains.
//
//	@Summary		Replace enabled wildcard domains
//	@Description	Replace and persist the additional wildcard domains enabled from the configured allowlist.
//	@Tags			wildcard
//	@ID				wildcardDomainsReplace
//	@Param			request	body	api.WildcardDomainsRequest	true	"Enabled additional domains"
//	@Security		BearerAuth
//	@Security		CookieAuth
//	@Success		200	{object}	api.Response
//	@Failure		400	{object}	api.Response
//	@Failure		401	{string}	string	"Unauthorized"
//	@Failure		403	{string}	string	"Forbidden"
//	@Failure		500	{object}	api.Response
//	@Failure		503	{object}	api.Response
//	@Router			/api/wildcard/domains [put]
func (api *v1) WildcardDomainsReplace(w http.ResponseWriter, r *http.Request) {
	wildcard, ok := api.wildcardMiddleware(w)
	if !ok {
		return
	}
	request := &WildcardDomainsRequest{}
	if err := json.NewDecoder(r.Body).Decode(request); err != nil {
		writeWildcardError(w, http.StatusBadRequest, err, wildcard)
		return
	}
	domains, err := wildcard.ValidateEnabledExtraDomains(request.Domains)
	if err != nil {
		writeWildcardError(w, http.StatusBadRequest, err, wildcard)
		return
	}
	encoded, err := json.Marshal(domains)
	if err != nil {
		writeWildcardError(w, http.StatusInternalServerError, err, wildcard)
		return
	}

	store, err := api.overrideStore(r.Context())
	if err != nil {
		writeWildcardError(w, http.StatusInternalServerError, err, wildcard)
		return
	}
	defer func() { _ = store.Close() }()
	if err := store.Upsert(r.Context(), overrides.OverrideWildcardDomains, overrides.OverrideSet, "enabled", string(encoded)); err != nil {
		writeWildcardError(w, http.StatusInternalServerError, err, wildcard)
		return
	}

	if err := wildcard.ReplaceEnabledExtraDomains(domains); err != nil {
		writeWildcardError(w, http.StatusInternalServerError, err, wildcard)
		return
	}
	conf := config.GetRunningConfig()
	conf.Wildcard.EnabledExtraDomains = copyStrings(domains)
	config.SetRunningConfig(conf)
	api.writeWildcardStatus(wildcard, w)
}

func (api *v1) wildcardMiddleware(w http.ResponseWriter) (*middleware.Wildcard, bool) {
	if api.server != nil {
		wildcard, ok := api.server.Middlewares["wildcard"].(*middleware.Wildcard)
		if ok {
			return wildcard, true
		}
	}
	w.WriteHeader(http.StatusServiceUnavailable)
	writeJSON(Response{Kind: WildcardResponseKind, Message: wildcardUnavailableMessage, CurrentStatus: formatBool(false)}, w)
	return nil, false
}

func (api *v1) writeWildcardStatus(wildcard *middleware.Wildcard, w http.ResponseWriter) {
	status := wildcard.Status()
	writeJSON(Response{
		Kind: WildcardResponseKind, Message: MESSAGE_OK,
		CurrentStatus: formatBool(status.Enabled), Wildcard: wildcardStatusDTO(status),
	}, w)
}

func writeWildcardError(w http.ResponseWriter, statusCode int, err error, wildcard *middleware.Wildcard) {
	status := wildcard.Status()
	w.WriteHeader(statusCode)
	writeJSON(Response{
		Kind: WildcardResponseKind, Message: err.Error(),
		CurrentStatus: formatBool(status.Enabled), Wildcard: wildcardStatusDTO(status),
	}, w)
}
