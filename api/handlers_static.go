package api

import (
	"encoding/json"
	"net/http"

	"github.com/jfardello/tdns/config"
	"github.com/jfardello/tdns/internal/overrides"
	"github.com/jfardello/tdns/middleware"
)

func (api *v1) StaticResponseToggle(w http.ResponseWriter, r *http.Request) {
	p, ok := api.server.Middlewares["static-response"].(*middleware.StaticResponse)
	if !ok {
		w.WriteHeader(http.StatusBadRequest)
		writeJSON(Response{
			Kind:          StaticResponseKind,
			Message:       "static response middleware is not configured",
			CurrentStatus: formatBool(false),
			Static:        staticResponseStatusFromConfig(),
		}, w)
		return
	}
	action := r.PathValue("action")
	state, err := actionToBool(action)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		writeJSON(Response{
			Kind:          StaticResponseKind,
			Message:       err.Error(),
			CurrentStatus: formatBool(p.IsEnabled()),
		}, w)
		return
	}
	p.SetEnabled(state)
	status, err := p.Status()
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		writeJSON(Response{
			Kind:          StaticResponseKind,
			Message:       err.Error(),
			CurrentStatus: formatBool(p.IsEnabled()),
		}, w)
		return
	}

	resp := Response{
		Kind:          StaticResponseKind,
		Message:       MESSAGE_OK,
		CurrentStatus: formatBool(status.Enabled),
		Static:        staticResponseStatusDTO(status),
	}
	writeJSON(resp, w)

}

func (api *v1) StaticResponseStatus(w http.ResponseWriter, r *http.Request) {
	p, ok := api.server.Middlewares["static-response"].(*middleware.StaticResponse)
	if !ok {
		writeJSON(Response{
			Kind:          StaticResponseKind,
			Message:       MESSAGE_OK,
			CurrentStatus: formatBool(false),
			Static:        staticResponseStatusFromConfig(),
		}, w)
		return
	}
	status, err := p.Status()
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		writeJSON(Response{
			Kind:          StaticResponseKind,
			Message:       err.Error(),
			CurrentStatus: formatBool(p.IsEnabled()),
		}, w)
		return
	}

	writeJSON(Response{
		Kind:          StaticResponseKind,
		Message:       MESSAGE_OK,
		CurrentStatus: formatBool(status.Enabled),
		Static:        staticResponseStatusDTO(status),
	}, w)
}

func (api *v1) StaticResponseReplace(w http.ResponseWriter, r *http.Request) {
	p, ok := api.server.Middlewares["static-response"].(*middleware.StaticResponse)
	if !ok {
		w.WriteHeader(http.StatusBadRequest)
		writeJSON(Response{
			Kind:          StaticResponseKind,
			Message:       "static response middleware is not configured",
			CurrentStatus: formatBool(false),
			Static:        staticResponseStatusFromConfig(),
		}, w)
		return
	}
	req := &StaticReplaceRequest{}
	err := json.NewDecoder(r.Body).Decode(req)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		writeJSON(Response{
			Kind:          StaticResponseKind,
			Message:       err.Error(),
			CurrentStatus: formatBool(p.IsEnabled()),
		}, w)
		return
	}

	hosts, err := middleware.ReadHostsLines(req.Hosts)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		writeJSON(Response{
			Kind:          StaticResponseKind,
			Message:       err.Error(),
			CurrentStatus: formatBool(p.IsEnabled()),
		}, w)
		return
	}

	if err := p.ReplaceRuntimeHosts(hosts); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		writeJSON(Response{
			Kind:          StaticResponseKind,
			Message:       err.Error(),
			CurrentStatus: formatBool(p.IsEnabled()),
		}, w)
		return
	}

	status, err := p.Status()
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		writeJSON(Response{
			Kind:          StaticResponseKind,
			Message:       err.Error(),
			CurrentStatus: formatBool(p.IsEnabled()),
		}, w)
		return
	}

	writeJSON(Response{
		Kind:          StaticResponseKind,
		Message:       MESSAGE_OK,
		CurrentStatus: formatBool(status.Enabled),
		Static:        staticResponseStatusDTO(status),
	}, w)
}

func (api *v1) StaticResponseReplacePersisted(w http.ResponseWriter, r *http.Request) {
	p, ok := api.server.Middlewares["static-response"].(*middleware.StaticResponse)
	if !ok {
		w.WriteHeader(http.StatusBadRequest)
		writeJSON(Response{
			Kind:          StaticResponseKind,
			Message:       "static response middleware is not configured",
			CurrentStatus: formatBool(false),
			Static:        staticResponseStatusFromConfig(),
		}, w)
		return
	}

	req := &StaticReplaceRequest{}
	if err := json.NewDecoder(r.Body).Decode(req); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		writeJSON(Response{
			Kind:          StaticResponseKind,
			Message:       err.Error(),
			CurrentStatus: formatBool(p.IsEnabled()),
		}, w)
		return
	}

	hosts, err := middleware.ReadHostsLines(req.Hosts)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		writeJSON(Response{
			Kind:          StaticResponseKind,
			Message:       err.Error(),
			CurrentStatus: formatBool(p.IsEnabled()),
		}, w)
		return
	}

	store, err := api.overrideStore(r.Context())
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		writeJSON(Response{
			Kind:          StaticResponseKind,
			Message:       err.Error(),
			CurrentStatus: formatBool(p.IsEnabled()),
		}, w)
		return
	}
	defer func() { _ = store.Close() }()

	if err := replaceOverrideHosts(r.Context(), store, overrides.OverrideStaticHost, hosts); err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		writeJSON(Response{
			Kind:          StaticResponseKind,
			Message:       err.Error(),
			CurrentStatus: formatBool(p.IsEnabled()),
		}, w)
		return
	}

	conf := config.GetRunningConfig()
	conf.StaticResponse.ExtraHosts = middlewareCloneHosts(hosts)
	config.SetRunningConfig(conf)
	p.ReplacePersistedHosts(hosts)

	status, err := p.Status()
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		writeJSON(Response{
			Kind:          StaticResponseKind,
			Message:       err.Error(),
			CurrentStatus: formatBool(p.IsEnabled()),
		}, w)
		return
	}

	writeJSON(Response{
		Kind:          StaticResponseKind,
		Message:       MESSAGE_OK,
		CurrentStatus: formatBool(status.Enabled),
		Static:        staticResponseStatusDTO(status),
	}, w)
}

func staticResponseStatusFromConfig() *StaticResponseStatus {
	conf := config.GetRunningConfig()
	status := &StaticResponseStatus{
		Enabled:         conf.StaticResponse.Enabled,
		File:            conf.StaticResponse.File,
		Labels:          append([]string(nil), conf.StaticResponse.Labels...),
		ConfiguredHosts: []HostEntry{},
		PersistedHosts:  hostEntryDTOs(middleware.HostsToEntries(conf.StaticResponse.ExtraHosts)),
		RuntimeHosts:    []HostEntry{},
	}
	if conf.StaticResponse.File == "" {
		return status
	}

	hosts, err := middleware.ReadHosts(conf.StaticResponse.File)
	if err == nil {
		status.ConfiguredHosts = hostEntryDTOs(middleware.HostsToEntries(hosts))
	}
	return status
}
