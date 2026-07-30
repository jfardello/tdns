package httpapi

import (
	"encoding/json"
	"net/http"

	"github.com/jfardello/tdns/config"
	"github.com/jfardello/tdns/log"
	"github.com/jfardello/tdns/middleware"
	"github.com/sirupsen/logrus"
)

// Toggle the stub resolver.
//
//	@Summary		Toggle the stub resolver
//	@Description	Enable or disable stub resolution.
//	@Tags			stub-resolver
//	@ID				stubResolverToggle
//	@Param			action	path	string	true	"Requested state"	Enums(start,stop)
//	@Security		BearerAuth
//	@Security		CookieAuth
//	@Success		200	{object}	api.Response
//	@Failure		401	{string}	string	"Unauthorized"
//	@Failure		403	{string}	string	"Forbidden"
//	@Failure		400	{object}	api.Response
//	@Router			/api/stub-resolver/{action} [post]
func (api *v1) StubToggle(w http.ResponseWriter, r *http.Request) {
	p := api.server.Middlewares["stub-resolver"].(*middleware.StubResolver)
	state, err := actionToBool(r.PathValue("action"))
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		writeJSON(Response{
			Message:       err.Error(),
			CurrentStatus: formatBool(p.IsEnabled()),
			Kind:          StubResolverResponseKind,
		}, w)
		return
	}

	api.server.StubsToggle(state)
	status := p.Status()
	res := Response{
		Message:       MESSAGE_OK,
		CurrentStatus: formatBool(status.Enabled),
		Kind:          StubResolverResponseKind,
		StubResolver:  stubResolverStatusDTO(status),
	}
	writeJSON(res, w)

}

// Get stub resolver status.
//
//	@Summary		Get stub resolver status
//	@Description	Return configured and runtime stub resolver state.
//	@Tags			stub-resolver
//	@ID				stubResolverStatus
//	@Security		BearerAuth
//	@Security		CookieAuth
//	@Success		200	{object}	api.Response
//	@Failure		401	{string}	string	"Unauthorized"
//	@Failure		403	{string}	string	"Forbidden"
//	@Router			/api/stub-resolver [get]
func (api *v1) StubStatus(w http.ResponseWriter, r *http.Request) {
	p := api.server.Middlewares["stub-resolver"].(*middleware.StubResolver)
	status := p.Status()
	writeJSON(Response{
		Message:       MESSAGE_OK,
		CurrentStatus: formatBool(status.Enabled),
		Kind:          StubResolverResponseKind,
		StubResolver:  stubResolverStatusDTO(status),
	}, w)
}

// Replace runtime stub resolvers.
//
//	@Summary		Replace runtime stub resolvers
//	@Description	Replace the in-memory stub resolver entries.
//	@Tags			stub-resolver
//	@ID				stubResolverReplace
//	@Param			request	body	api.StubReplaceRequest	true	"Stub resolver entries"
//	@Security		BearerAuth
//	@Security		CookieAuth
//	@Success		200	{object}	api.Response
//	@Failure		401	{string}	string	"Unauthorized"
//	@Failure		403	{string}	string	"Forbidden"
//	@Failure		400	{object}	api.Response
//	@Router			/api/stub-resolver [post]
func (api *v1) StubReplace(w http.ResponseWriter, r *http.Request) {
	l := log.GetLogger("serve", "api-server")
	logger := l.WithFields(logrus.Fields{"Method": "StubReplace"})
	stubRequest := &StubReplaceRequest{}
	p := api.server.Middlewares["stub-resolver"]
	err := json.NewDecoder(r.Body).Decode(&stubRequest)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	st := p.(*middleware.StubResolver)
	c := config.GetRunningConfig()
	err = st.ReplaceRuntimeEntries(stubRequest.Stubs, c.Timeout, c.UpstreamTimeout)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		writeJSON(Response{
			Kind:          StubResolverResponseKind,
			Message:       err.Error(),
			CurrentStatus: formatBool(st.IsEnabled()),
		}, w)
		return
	}

	status := st.Status()
	logger.Infof("Loaded: %d stubs", len(status.RuntimeStubs))
	resp := Response{
		Message:       MESSAGE_OK,
		Kind:          StubResolverResponseKind,
		CurrentStatus: formatBool(status.Enabled),
		Items:         stubRequest.Stubs,
		StubResolver:  stubResolverStatusDTO(status),
	}
	writeJSON(resp, w)

}
