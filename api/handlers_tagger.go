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

// Create a tag.
//
//	@Summary		Create a tag
//	@Description	Create a client classification tag.
//	@Tags			tagger
//	@ID				taggerTagCreate
//	@Param			request	body	AddTagRequest	true	"Tag name"
//	@Security		BearerAuth
//	@Success		200	{object}	Response
//	@Failure		401	{string}	string	"Unauthorized"
//	@Failure		400	{object}	Response
//	@Failure		503	{object}	Response
//	@Router			/api/tagger/tags [post]
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

// List tags.
//
//	@Summary		List tags
//	@Description	Return all client classification tags.
//	@Tags			tagger
//	@ID				taggerTagsList
//	@Security		BearerAuth
//	@Success		200	{object}	Response
//	@Failure		401	{string}	string	"Unauthorized"
//	@Failure		500	{object}	Response
//	@Failure		503	{object}	Response
//	@Router			/api/tagger/tags [get]
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

// Add tag members.
//
//	@Summary		Add tag members
//	@Description	Assign one or more addresses to a tag.
//	@Tags			tagger
//	@ID				taggerTagMembersAdd
//	@Param			tagName	path	string				true	"Tag name"
//	@Param			request	body	AddMemberRequest	true	"Member addresses"
//	@Security		BearerAuth
//	@Success		200	{object}	Response
//	@Failure		401	{string}	string	"Unauthorized"
//	@Failure		400	{object}	Response
//	@Failure		500	{object}	Response
//	@Failure		503	{object}	Response
//	@Router			/api/tagger/tags/{tagName} [post]
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

// List tag members.
//
//	@Summary		List tag members
//	@Description	Return addresses assigned to a tag.
//	@Tags			tagger
//	@ID				taggerTagMembersList
//	@Param			tagName	path	string	true	"Tag name"
//	@Security		BearerAuth
//	@Success		200	{object}	Response
//	@Failure		401	{string}	string	"Unauthorized"
//	@Failure		500	{object}	Response
//	@Failure		503	{object}	Response
//	@Router			/api/tagger/tags/{tagName} [get]
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

// Search known hosts.
//
//	@Summary		Search known hosts
//	@Description	Search hosts observed in the DNS log.
//	@Tags			tagger
//	@ID				taggerKnownHostsSearch
//	@Param			search	query	string	false	"Address or host substring"
//	@Param			limit	query	int		false	"Maximum result count"	default(20)	minimum(1)
//	@Security		BearerAuth
//	@Success		200	{object}	Response
//	@Failure		401	{string}	string	"Unauthorized"
//	@Failure		400	{object}	Response
//	@Failure		500	{object}	Response
//	@Failure		503	{object}	Response
//	@Router			/api/tagger/hosts [get]
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

// Remove a tag member.
//
//	@Summary		Remove a tag member
//	@Description	Remove an address from a tag.
//	@Tags			tagger
//	@ID				taggerTagMemberDelete
//	@Param			tagName	path	string	true	"Tag name"
//	@Param			address	path	string	true	"IP address or CIDR"
//	@Security		BearerAuth
//	@Success		200	{object}	Response
//	@Failure		401	{string}	string	"Unauthorized"
//	@Failure		400	{object}	Response
//	@Failure		503	{object}	Response
//	@Router			/api/tagger/tags/{tagName}/{address} [delete]
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

// Create or update an address.
//
//	@Summary		Create or update an address
//	@Description	Set all tags for an address supplied in the request body.
//	@Tags			tagger
//	@ID				taggerAddressCreate
//	@Param			request	body	MemberLabelsRequest	true	"Address and tags"
//	@Security		BearerAuth
//	@Success		200	{object}	Response
//	@Failure		401	{string}	string	"Unauthorized"
//	@Failure		400	{object}	Response
//	@Failure		500	{object}	Response
//	@Failure		503	{object}	Response
//	@Router			/api/tagger/address [post]
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

// Replace address tags.
//
//	@Summary		Replace address tags
//	@Description	Replace all tags assigned to an address.
//	@Tags			tagger
//	@ID				taggerAddressReplace
//	@Param			address	path	string						true	"IP address or CIDR"
//	@Param			request	body	ReplaceMemberLabelsRequest	true	"Replacement tags"
//	@Security		BearerAuth
//	@Success		200	{object}	Response
//	@Failure		401	{string}	string	"Unauthorized"
//	@Failure		400	{object}	Response
//	@Failure		500	{object}	Response
//	@Failure		503	{object}	Response
//	@Router			/api/tagger/address/{address} [put]
func (api *v1) TaggerAddressReplace(w http.ResponseWriter, r *http.Request) {
	api.taggerAddressReplace(w, r, strings.TrimSpace(r.PathValue("address")))
}

// Replace address tags using the legacy path.
//
//	@Summary		Replace address tags using the legacy path
//	@Description	Replace all tags assigned to an address through the legacy endpoint.
//	@Tags			tagger
//	@ID				taggerLegacyAddressReplace
//	@Param			tagName	path	string						true	"Address encoded in the legacy path parameter"
//	@Param			request	body	ReplaceMemberLabelsRequest	true	"Replacement tags"
//	@Security		BearerAuth
//	@Success		200	{object}	Response
//	@Failure		401	{string}	string	"Unauthorized"
//	@Failure		400	{object}	Response
//	@Failure		500	{object}	Response
//	@Failure		503	{object}	Response
//	@Router			/api/tagger/addr/{tagName} [put]
func (api *v1) TaggerLegacyAddressReplace(w http.ResponseWriter, r *http.Request) {
	api.taggerAddressReplace(w, r, strings.TrimSpace(r.PathValue("tagName")))
}

func (api *v1) taggerAddressReplace(w http.ResponseWriter, r *http.Request, address string) {
	p, err := api.taggerMiddleware()
	if err != nil {
		writeTaggerResponse(w, http.StatusServiceUnavailable, err.Error(), nil)
		return
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

// Delete a tag.
//
//	@Summary		Delete a tag
//	@Description	Delete a client classification tag.
//	@Tags			tagger
//	@ID				taggerTagDelete
//	@Param			tagName	path	string	true	"Tag name"
//	@Security		BearerAuth
//	@Success		200	{object}	Response
//	@Failure		401	{string}	string	"Unauthorized"
//	@Failure		400	{object}	Response
//	@Failure		503	{object}	Response
//	@Router			/api/tagger/tags/{tagName} [delete]
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
