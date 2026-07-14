package api

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/jfardello/tdns/internal/overrides"
	"github.com/jfardello/tdns/log"
	"github.com/jfardello/tdns/middleware"
	"github.com/sirupsen/logrus"
)

func (api *v1) DeleteCache(w http.ResponseWriter, r *http.Request) {
	l := log.GetLogger("serve", "api-server")
	logger := l.WithFields(logrus.Fields{"Method": "ClearCache"})
	w.Header().Set("Content-Type", "application/json")
	logger.Info("Clearing cache")
	err := api.server.ClearCache()
	if err != nil {
		logger.Error("Error clearing cache: ", err)
		writeJSON(Response{
			Kind:          CacheResponseKind,
			Message:       "Status Fail",
			CurrentStatus: formatBool(middleware.GetCache().IsEnabled()),
		}, w)
		return
	}
	status := middleware.GetCache().StatusView()
	writeJSON(Response{
		Kind:          CacheResponseKind,
		Message:       MESSAGE_OK,
		CurrentStatus: formatBool(status.Enabled),
		Cache:         cacheStatusDTO(status),
	}, w)
}

func (api *v1) CacheStatus(w http.ResponseWriter, r *http.Request) {
	status := middleware.GetCache().StatusView()
	writeJSON(Response{
		Kind:          CacheResponseKind,
		Message:       MESSAGE_OK,
		CurrentStatus: formatBool(status.Enabled),
		Cache:         cacheStatusDTO(status),
	}, w)
}

func (api *v1) CacheToggle(w http.ResponseWriter, r *http.Request) {
	state, err := actionToBool(r.PathValue("action"))
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		writeJSON(Response{
			Kind:          CacheResponseKind,
			Message:       err.Error(),
			CurrentStatus: formatBool(middleware.GetCache().IsEnabled()),
		}, w)
		return
	}

	store, err := api.overrideStore(r.Context())
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		writeJSON(Response{
			Kind:          CacheResponseKind,
			Message:       err.Error(),
			CurrentStatus: formatBool(middleware.GetCache().IsEnabled()),
		}, w)
		return
	}
	defer func() { _ = store.Close() }()

	if err := store.Upsert(r.Context(), overrides.OverrideCacheEnabled, overrides.OverrideSet, "enabled", strconv.FormatBool(state)); err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		writeJSON(Response{
			Kind:          CacheResponseKind,
			Message:       err.Error(),
			CurrentStatus: formatBool(middleware.GetCache().IsEnabled()),
		}, w)
		return
	}

	api.server.CacheToggle(state)
	status := middleware.GetCache().StatusView()
	writeJSON(Response{
		Kind:          CacheResponseKind,
		Message:       MESSAGE_OK,
		CurrentStatus: formatBool(status.Enabled),
		Cache:         cacheStatusDTO(status),
	}, w)
}

func (api *v1) CacheReplaceExcludes(w http.ResponseWriter, r *http.Request) {
	req := &CacheExcludeRequest{}
	if err := json.NewDecoder(r.Body).Decode(req); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		writeJSON(Response{
			Kind:          CacheResponseKind,
			Message:       err.Error(),
			CurrentStatus: formatBool(middleware.GetCache().IsEnabled()),
		}, w)
		return
	}

	normalized := overrides.NormalizeCacheSelectors(req.Excludes)

	store, err := api.overrideStore(r.Context())
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		writeJSON(Response{
			Kind:          CacheResponseKind,
			Message:       err.Error(),
			CurrentStatus: formatBool(middleware.GetCache().IsEnabled()),
		}, w)
		return
	}
	defer func() { _ = store.Close() }()

	if err := store.DeleteByKind(r.Context(), overrides.OverrideCacheExclude); err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		writeJSON(Response{
			Kind:          CacheResponseKind,
			Message:       err.Error(),
			CurrentStatus: formatBool(middleware.GetCache().IsEnabled()),
		}, w)
		return
	}
	for _, each := range normalized {
		if err := store.Upsert(r.Context(), overrides.OverrideCacheExclude, overrides.OverrideUpsert, each, ""); err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			writeJSON(Response{
				Kind:          CacheResponseKind,
				Message:       err.Error(),
				CurrentStatus: formatBool(middleware.GetCache().IsEnabled()),
			}, w)
			return
		}
	}

	api.server.CacheReplaceExcludes(normalized)
	status := middleware.GetCache().StatusView()
	writeJSON(Response{
		Kind:          CacheResponseKind,
		Message:       MESSAGE_OK,
		CurrentStatus: formatBool(status.Enabled),
		Cache:         cacheStatusDTO(status),
	}, w)
}
