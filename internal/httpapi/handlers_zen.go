package httpapi

import (
	"encoding/json"
	"net/http"

	"github.com/jfardello/tdns/config"
	"github.com/jfardello/tdns/internal/overrides"
	"github.com/jfardello/tdns/log"
	"github.com/jfardello/tdns/middleware"
	"github.com/sirupsen/logrus"
)

// Replace runtime zen domains.
//
//	@Summary		Replace runtime zen domains
//	@Description	Replace the in-memory domains blocked by zen mode.
//	@Tags			zen-mode
//	@ID				zenModeDomainsReplace
//	@Param			request	body	api.ZenReplaceRequest	true	"Zen mode domains"
//	@Security		BearerAuth
//	@Security		CookieAuth
//	@Success		200	{object}	api.Response
//	@Failure		401	{string}	string	"Unauthorized"
//	@Failure		403	{string}	string	"Forbidden"
//	@Failure		400	{object}	api.Response
//	@Failure		500	{object}	api.Response
//	@Router			/api/zen-mode [post]
func (api *v1) ZenDomainsReplace(w http.ResponseWriter, r *http.Request) {
	l := log.GetLogger("serve", "api-server")
	logger := l.WithFields(logrus.Fields{"Method": "ZenDomainsReplace"})
	zreq := &ZenReplaceRequest{}
	err := json.NewDecoder(r.Body).Decode(&zreq)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	p := api.server.Middlewares["zen-mode"]
	st := p.(*middleware.ZenMode)
	logger.Debug("About to replace domain for zen mode.")
	h := map[string]string{}
	for _, each := range zreq.ZenDomains {
		h[each] = middleware.ZenModeIP
		logger.WithFields(logrus.Fields{"domain": each}).Debug("Adding domain to zen mode.")
	}
	err = st.ReplaceDomains(h)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		writeJSON(Response{
			Kind:          ZenModeResponseKind,
			Message:       err.Error(),
			CurrentStatus: formatBool(st.Status()),
		}, w)
		return
	}
	status, err := st.StatusView()
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		writeJSON(Response{
			Kind:          ZenModeResponseKind,
			Message:       err.Error(),
			CurrentStatus: formatBool(st.Status()),
		}, w)
		return
	}
	writeJSON(Response{
		Kind:          ZenModeResponseKind,
		Message:       MESSAGE_OK,
		CurrentStatus: formatBool(st.Status()),
		Items:         st.GetDomains(),
		ZenMode:       zenModeStatusDTO(status),
	}, w)

}

// Replace persisted zen domains.
//
//	@Summary		Replace persisted zen domains
//	@Description	Replace zen domains stored in configuration overrides.
//	@Tags			zen-mode
//	@ID				zenModePersistedDomainsReplace
//	@Param			request	body	api.ZenReplaceRequest	true	"Persisted zen mode domains"
//	@Security		BearerAuth
//	@Security		CookieAuth
//	@Success		200	{object}	api.Response
//	@Failure		401	{string}	string	"Unauthorized"
//	@Failure		403	{string}	string	"Forbidden"
//	@Failure		400	{object}	api.Response
//	@Failure		500	{object}	api.Response
//	@Router			/api/zen-mode/persisted/domains [post]
func (api *v1) ZenPersistedDomainsReplace(w http.ResponseWriter, r *http.Request) {
	p := api.server.Middlewares["zen-mode"].(*middleware.ZenMode)
	req := &ZenReplaceRequest{}
	if err := json.NewDecoder(r.Body).Decode(req); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		writeJSON(Response{
			Kind:          ZenModeResponseKind,
			Message:       err.Error(),
			CurrentStatus: formatBool(p.Status()),
		}, w)
		return
	}

	domains := overrides.NormalizeDomains(req.ZenDomains)
	store, err := api.overrideStore(r.Context())
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		writeJSON(Response{
			Kind:          ZenModeResponseKind,
			Message:       err.Error(),
			CurrentStatus: formatBool(p.Status()),
		}, w)
		return
	}
	defer func() { _ = store.Close() }()

	if err := replaceOverrideValues(r.Context(), store, overrides.OverrideZenDomain, domains); err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		writeJSON(Response{
			Kind:          ZenModeResponseKind,
			Message:       err.Error(),
			CurrentStatus: formatBool(p.Status()),
		}, w)
		return
	}

	conf := config.GetRunningConfig()
	conf.ZenMode.PersistedDomains = copyStrings(domains)
	config.SetRunningConfig(conf)
	p.ReplacePersistedDomains(domains)

	status, err := p.StatusView()
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		writeJSON(Response{
			Kind:          ZenModeResponseKind,
			Message:       err.Error(),
			CurrentStatus: formatBool(p.Status()),
		}, w)
		return
	}

	writeJSON(Response{
		Kind:          ZenModeResponseKind,
		Message:       MESSAGE_OK,
		CurrentStatus: formatBool(status.Enabled),
		ZenMode:       zenModeStatusDTO(status),
	}, w)
}

// Replace persisted zen exclusions.
//
//	@Summary		Replace persisted zen exclusions
//	@Description	Replace zen exclusions stored in configuration overrides.
//	@Tags			zen-mode
//	@ID				zenModePersistedExcludesReplace
//	@Param			request	body	api.ZenExcludesRequest	true	"Persisted zen mode exclusions"
//	@Security		BearerAuth
//	@Security		CookieAuth
//	@Success		200	{object}	api.Response
//	@Failure		401	{string}	string	"Unauthorized"
//	@Failure		403	{string}	string	"Forbidden"
//	@Failure		400	{object}	api.Response
//	@Failure		500	{object}	api.Response
//	@Router			/api/zen-mode/persisted/excludes [post]
func (api *v1) ZenPersistedExcludesReplace(w http.ResponseWriter, r *http.Request) {
	p := api.server.Middlewares["zen-mode"].(*middleware.ZenMode)
	req := &ZenExcludesRequest{}
	if err := json.NewDecoder(r.Body).Decode(req); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		writeJSON(Response{
			Kind:          ZenModeResponseKind,
			Message:       err.Error(),
			CurrentStatus: formatBool(p.Status()),
		}, w)
		return
	}

	excludes := overrides.NormalizeSelectors(req.Excludes)
	store, err := api.overrideStore(r.Context())
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		writeJSON(Response{
			Kind:          ZenModeResponseKind,
			Message:       err.Error(),
			CurrentStatus: formatBool(p.Status()),
		}, w)
		return
	}
	defer func() { _ = store.Close() }()

	if err := replaceOverrideValues(r.Context(), store, overrides.OverrideZenExclude, excludes); err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		writeJSON(Response{
			Kind:          ZenModeResponseKind,
			Message:       err.Error(),
			CurrentStatus: formatBool(p.Status()),
		}, w)
		return
	}

	conf := config.GetRunningConfig()
	conf.ZenMode.PersistedExcludes = copyStrings(excludes)
	config.SetRunningConfig(conf)
	p.ReplacePersistedExcludes(excludes)

	status, err := p.StatusView()
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		writeJSON(Response{
			Kind:          ZenModeResponseKind,
			Message:       err.Error(),
			CurrentStatus: formatBool(p.Status()),
		}, w)
		return
	}

	writeJSON(Response{
		Kind:          ZenModeResponseKind,
		Message:       MESSAGE_OK,
		CurrentStatus: formatBool(status.Enabled),
		ZenMode:       zenModeStatusDTO(status),
	}, w)
}

// Start zen mode.
//
//	@Summary		Start zen mode
//	@Description	Start a zen mode period using the current configuration.
//	@Tags			zen-mode
//	@ID				zenModeStart
//	@Security		BearerAuth
//	@Security		CookieAuth
//	@Success		200	{object}	api.Response
//	@Failure		401	{string}	string	"Unauthorized"
//	@Failure		403	{string}	string	"Forbidden"
//	@Failure		500	{object}	api.Response
//	@Router			/api/zen-mode/start [post]
func (api *v1) ZenModeStart(w http.ResponseWriter, r *http.Request) {
	p := api.server.Middlewares["zen-mode"]
	z := p.(*middleware.ZenMode)
	z.Start()
	status, err := z.StatusView()
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		writeJSON(Response{
			Kind:          ZenModeResponseKind,
			Message:       err.Error(),
			CurrentStatus: formatBool(z.Status()),
		}, w)
		return
	}
	res := Response{
		Kind:          ZenModeResponseKind,
		Message:       MESSAGE_OK,
		CurrentStatus: formatBool(status.Enabled),
		Items:         z.GetDomains(),
		ZenMode:       zenModeStatusDTO(status),
	}
	writeJSON(res, w)
}

// Get zen mode status.
//
//	@Summary		Get zen mode status
//	@Description	Return zen mode configuration, runtime state, and timing.
//	@Tags			zen-mode
//	@ID				zenModeStatus
//	@Security		BearerAuth
//	@Security		CookieAuth
//	@Success		200	{object}	api.Response
//	@Failure		401	{string}	string	"Unauthorized"
//	@Failure		403	{string}	string	"Forbidden"
//	@Failure		500	{object}	api.Response
//	@Router			/api/zen-mode [get]
func (api *v1) ZenModeStatus(w http.ResponseWriter, r *http.Request) {
	p := api.server.Middlewares["zen-mode"].(*middleware.ZenMode)
	status, err := p.StatusView()
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		writeJSON(Response{
			Kind:          ZenModeResponseKind,
			Message:       err.Error(),
			CurrentStatus: formatBool(p.Status()),
		}, w)
		return
	}

	writeJSON(Response{
		Kind:          ZenModeResponseKind,
		Message:       MESSAGE_OK,
		CurrentStatus: formatBool(status.Enabled),
		ZenMode:       zenModeStatusDTO(status),
	}, w)
}
