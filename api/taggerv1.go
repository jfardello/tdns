package api

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/jfardello/tdns/plugin"
)

type AddTagRequest struct {
	Name string `json:"name"`
}

type AddMemberRequest struct {
	Members []string `json:"members"`
}

func (api *v1) taggerPlugin() (*plugin.Tagger, error) {
	p, ok := api.server.Plugins["tagger"]
	if !ok {
		return nil, errors.New("tagger middleware is disabled")
	}
	t, ok := p.(*plugin.Tagger)
	if !ok {
		return nil, errors.New("tagger middleware has an unexpected type")
	}
	return t, nil
}

func writeTaggerResponse(w http.ResponseWriter, status int, message string, items []string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	writeJSON(Response{
		Kind:          TAGGEER_RESPONSE_KIND,
		Message:       message,
		CurrentStatus: "Enabled",
		Items:         items,
	}, w)
}

func (api *v1) TaggerAddTag(w http.ResponseWriter, r *http.Request) {
	p, err := api.taggerPlugin()
	if err != nil {
		writeTaggerResponse(w, http.StatusServiceUnavailable, err.Error(), nil)
		return
	}

	jr := new(AddTagRequest)
	if err := json.NewDecoder(r.Body).Decode(jr); err != nil {
		writeTaggerResponse(w, http.StatusBadRequest, err.Error(), nil)
		return
	}
	if err := p.CreateTag(jr.Name); err != nil {
		writeTaggerResponse(w, http.StatusBadRequest, err.Error(), nil)
		return
	}
	writeTaggerResponse(w, http.StatusOK, MESSAGE_OK, nil)
}

func (api *v1) TaggerGetTags(w http.ResponseWriter, r *http.Request) {
	p, err := api.taggerPlugin()
	if err != nil {
		writeTaggerResponse(w, http.StatusServiceUnavailable, err.Error(), nil)
		return
	}

	tags, err := p.GetTags()
	if err != nil {
		writeTaggerResponse(w, http.StatusInternalServerError, err.Error(), nil)
		return
	}
	writeTaggerResponse(w, http.StatusOK, MESSAGE_OK, tags)
}

func (api *v1) TaggerAddMember(w http.ResponseWriter, r *http.Request) {
	writeTaggerResponse(w, http.StatusNotImplemented, "TaggerAddMember is pending the storage contract redesign in phase 1", nil)
}

func (api *v1) TaggerTagGetMembers(w http.ResponseWriter, r *http.Request) {
	writeTaggerResponse(w, http.StatusNotImplemented, "TaggerTagGetMembers is pending the storage contract redesign in phase 1", nil)
}

func (api *v1) TaggerDeleteTagMember(w http.ResponseWriter, r *http.Request) {
	writeTaggerResponse(w, http.StatusNotImplemented, "TaggerDeleteTagMember is pending the storage contract redesign in phase 1", nil)
}

func (api *v1) TaggerAddressCreate(w http.ResponseWriter, r *http.Request) {
	writeTaggerResponse(w, http.StatusNotImplemented, "TaggerAddressCreate is pending the storage contract redesign in phase 1", nil)
}

func (api *v1) TaggerAddressReplace(w http.ResponseWriter, r *http.Request) {
	writeTaggerResponse(w, http.StatusNotImplemented, "TaggerAddressReplace is pending the storage contract redesign in phase 1", nil)
}

func (api *v1) TaggerDeleteTag(w http.ResponseWriter, r *http.Request) {
	p, err := api.taggerPlugin()
	if err != nil {
		writeTaggerResponse(w, http.StatusServiceUnavailable, err.Error(), nil)
		return
	}

	tag := r.PathValue("tagName")
	if err := p.DeleteTag(tag); err != nil {
		writeTaggerResponse(w, http.StatusBadRequest, err.Error(), nil)
		return
	}
	writeTaggerResponse(w, http.StatusOK, MESSAGE_OK, nil)
}
