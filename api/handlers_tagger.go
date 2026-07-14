package api

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/jfardello/tdns/middleware"
	"github.com/jfardello/tdns/storage"
)

func (api *v1) taggerMiddleware() (*middleware.Tagger, error) {
	p, ok := api.server.Middlewares["tagger"]
	if !ok {
		return nil, errors.New("tagger middleware is disabled")
	}
	t, ok := p.(*middleware.Tagger)
	if !ok {
		return nil, errors.New("tagger middleware has an unexpected type")
	}
	return t, nil
}

func writeTaggerResponse(w http.ResponseWriter, status int, message string, items []string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	writeJSON(Response{
		Kind:          TaggerResponseKind,
		Message:       message,
		CurrentStatus: "Enabled",
		Items:         items,
	}, w)
}

func writeTaggerDataResponse(w http.ResponseWriter, status int, message string, members []storage.TagMember, hosts []storage.KnownHost) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	writeJSON(Response{
		Kind:          TaggerResponseKind,
		Message:       message,
		CurrentStatus: "Enabled",
		TagMembers:    tagMemberDTOs(members),
		KnownHosts:    knownHostDTOs(hosts),
	}, w)
}

func (api *v1) TaggerAddTag(w http.ResponseWriter, r *http.Request) {
	p, err := api.taggerMiddleware()
	if err != nil {
		writeTaggerResponse(w, http.StatusServiceUnavailable, err.Error(), nil)
		return
	}

	jr := new(AddTagRequest)
	if err := json.NewDecoder(r.Body).Decode(jr); err != nil {
		writeTaggerResponse(w, http.StatusBadRequest, err.Error(), nil)
		return
	}
	if err := p.CreateTag(strings.TrimSpace(jr.Name)); err != nil {
		writeTaggerResponse(w, http.StatusBadRequest, err.Error(), nil)
		return
	}
	writeTaggerResponse(w, http.StatusOK, MESSAGE_OK, nil)
}

func (api *v1) TaggerGetTags(w http.ResponseWriter, r *http.Request) {
	p, err := api.taggerMiddleware()
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
	p, err := api.taggerMiddleware()
	if err != nil {
		writeTaggerResponse(w, http.StatusServiceUnavailable, err.Error(), nil)
		return
	}

	tag := strings.TrimSpace(r.PathValue("tagName"))
	req := new(AddMemberRequest)
	if err := json.NewDecoder(r.Body).Decode(req); err != nil {
		writeTaggerResponse(w, http.StatusBadRequest, err.Error(), nil)
		return
	}
	if err := p.AddMembers(tag, req.Members); err != nil {
		writeTaggerResponse(w, http.StatusBadRequest, err.Error(), nil)
		return
	}
	members, err := p.GetMemberDetails(tag)
	if err != nil {
		writeTaggerResponse(w, http.StatusInternalServerError, err.Error(), nil)
		return
	}
	writeTaggerDataResponse(w, http.StatusOK, MESSAGE_OK, members, nil)
}

func (api *v1) TaggerTagGetMembers(w http.ResponseWriter, r *http.Request) {
	p, err := api.taggerMiddleware()
	if err != nil {
		writeTaggerResponse(w, http.StatusServiceUnavailable, err.Error(), nil)
		return
	}

	tag := strings.TrimSpace(r.PathValue("tagName"))
	members, err := p.GetMemberDetails(tag)
	if err != nil {
		writeTaggerResponse(w, http.StatusInternalServerError, err.Error(), nil)
		return
	}
	writeTaggerDataResponse(w, http.StatusOK, MESSAGE_OK, members, nil)
}

func (api *v1) TaggerKnownHosts(w http.ResponseWriter, r *http.Request) {
	p, err := api.taggerMiddleware()
	if err != nil {
		writeTaggerResponse(w, http.StatusServiceUnavailable, err.Error(), nil)
		return
	}

	query := strings.TrimSpace(r.URL.Query().Get("search"))
	limit := 20
	if raw := strings.TrimSpace(r.URL.Query().Get("limit")); raw != "" {
		parsed, err := strconv.Atoi(raw)
		if err != nil {
			writeTaggerResponse(w, http.StatusBadRequest, err.Error(), nil)
			return
		}
		limit = parsed
	}

	hosts, err := p.SearchKnownHosts(query, limit)
	if err != nil {
		writeTaggerResponse(w, http.StatusInternalServerError, err.Error(), nil)
		return
	}

	writeTaggerDataResponse(w, http.StatusOK, MESSAGE_OK, nil, hosts)
}

func (api *v1) TaggerDeleteTagMember(w http.ResponseWriter, r *http.Request) {
	p, err := api.taggerMiddleware()
	if err != nil {
		writeTaggerResponse(w, http.StatusServiceUnavailable, err.Error(), nil)
		return
	}

	tag := strings.TrimSpace(r.PathValue("tagName"))
	address := strings.TrimSpace(r.PathValue("address"))
	if err := p.RemoveMember(tag, address); err != nil {
		writeTaggerResponse(w, http.StatusBadRequest, err.Error(), nil)
		return
	}
	writeTaggerResponse(w, http.StatusOK, MESSAGE_OK, nil)
}

func (api *v1) TaggerAddressCreate(w http.ResponseWriter, r *http.Request) {
	p, err := api.taggerMiddleware()
	if err != nil {
		writeTaggerResponse(w, http.StatusServiceUnavailable, err.Error(), nil)
		return
	}

	req := new(MemberLabelsRequest)
	if err := json.NewDecoder(r.Body).Decode(req); err != nil {
		writeTaggerResponse(w, http.StatusBadRequest, err.Error(), nil)
		return
	}
	if err := p.UpsertMember(strings.TrimSpace(req.Address), req.Tags); err != nil {
		writeTaggerResponse(w, http.StatusBadRequest, err.Error(), nil)
		return
	}
	labels, err := p.GetMemberLabels(strings.TrimSpace(req.Address))
	if err != nil {
		writeTaggerResponse(w, http.StatusInternalServerError, err.Error(), nil)
		return
	}
	writeTaggerResponse(w, http.StatusOK, MESSAGE_OK, labels)
}

func (api *v1) TaggerAddressReplace(w http.ResponseWriter, r *http.Request) {
	p, err := api.taggerMiddleware()
	if err != nil {
		writeTaggerResponse(w, http.StatusServiceUnavailable, err.Error(), nil)
		return
	}

	address := strings.TrimSpace(r.PathValue("address"))
	if address == "" {
		address = strings.TrimSpace(r.PathValue("tagName"))
	}
	req := new(ReplaceMemberLabelsRequest)
	if err := json.NewDecoder(r.Body).Decode(req); err != nil {
		writeTaggerResponse(w, http.StatusBadRequest, err.Error(), nil)
		return
	}
	if err := p.UpsertMember(address, req.Tags); err != nil {
		writeTaggerResponse(w, http.StatusBadRequest, err.Error(), nil)
		return
	}
	labels, err := p.GetMemberLabels(address)
	if err != nil {
		writeTaggerResponse(w, http.StatusInternalServerError, err.Error(), nil)
		return
	}
	writeTaggerResponse(w, http.StatusOK, MESSAGE_OK, labels)
}

func (api *v1) TaggerDeleteTag(w http.ResponseWriter, r *http.Request) {
	p, err := api.taggerMiddleware()
	if err != nil {
		writeTaggerResponse(w, http.StatusServiceUnavailable, err.Error(), nil)
		return
	}

	tag := strings.TrimSpace(r.PathValue("tagName"))
	if err := p.DeleteTag(tag); err != nil {
		writeTaggerResponse(w, http.StatusBadRequest, err.Error(), nil)
		return
	}
	writeTaggerResponse(w, http.StatusOK, MESSAGE_OK, nil)
}
