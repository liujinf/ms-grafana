package api

import (
	"net/http"
	"strings"

	"k8s.io/apimachinery/pkg/api/errors"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"

	"github.com/grafana/grafana/pkg/api/dtos"
	"github.com/grafana/grafana/pkg/api/response"
	"github.com/grafana/grafana/pkg/api/routing"
	"github.com/grafana/grafana/pkg/apis/playlist/v0alpha1"
	"github.com/grafana/grafana/pkg/middleware"
	contextmodel "github.com/grafana/grafana/pkg/services/contexthandler/model"
	"github.com/grafana/grafana/pkg/services/featuremgmt"
	"github.com/grafana/grafana/pkg/services/grafana-apiserver/endpoints/request"
	"github.com/grafana/grafana/pkg/services/playlist"
	"github.com/grafana/grafana/pkg/util/errutil/errhttp"
	"github.com/grafana/grafana/pkg/web"
)

type playlistAPIHandler struct {
	SearchPlaylists  []web.Handler
	GetPlaylist      []web.Handler
	GetPlaylistItems []web.Handler
	DeletePlaylist   []web.Handler
	UpdatePlaylist   []web.Handler
	CreatePlaylist   []web.Handler
}

func chainHandlers(h ...web.Handler) []web.Handler {
	return h
}

func (hs *HTTPServer) registerPlaylistAPI(apiRoute routing.RouteRegister) {
	handler := playlistAPIHandler{
		SearchPlaylists:  chainHandlers(routing.Wrap(hs.SearchPlaylists)),
		GetPlaylist:      chainHandlers(hs.validateOrgPlaylist, routing.Wrap(hs.GetPlaylist)),
		GetPlaylistItems: chainHandlers(hs.validateOrgPlaylist, routing.Wrap(hs.GetPlaylistItems)),
		DeletePlaylist:   chainHandlers(middleware.ReqEditorRole, hs.validateOrgPlaylist, routing.Wrap(hs.DeletePlaylist)),
		UpdatePlaylist:   chainHandlers(middleware.ReqEditorRole, hs.validateOrgPlaylist, routing.Wrap(hs.UpdatePlaylist)),
		CreatePlaylist:   chainHandlers(middleware.ReqEditorRole, routing.Wrap(hs.CreatePlaylist)),
	}

	// Alternative implementations for k8s
	if hs.Features.IsEnabled(featuremgmt.FlagKubernetesPlaylistsAPI) {
		namespacer := request.GetNamespaceMapper(hs.Cfg)
		gvr := schema.GroupVersionResource{
			Group:    v0alpha1.GroupName,
			Version:  v0alpha1.VersionID,
			Resource: "playlists",
		}

		clientGetter := func(c *contextmodel.ReqContext) (dynamic.ResourceInterface, bool) {
			dyn, err := dynamic.NewForConfig(hs.clientConfigProvider.GetDirectRestConfig(c))
			if err != nil {
				c.JsonApiErr(500, "client", err)
				return nil, false
			}
			return dyn.Resource(gvr).Namespace(namespacer(c.OrgID)), true
		}

		errorWriter := func(c *contextmodel.ReqContext, err error) {
			//nolint:errorlint
			statusError, ok := err.(*errors.StatusError)
			if ok {
				c.JsonApiErr(int(statusError.Status().Code),
					statusError.Status().Message, err)
				return
			}
			errhttp.Write(c.Req.Context(), err, c.Resp)
		}

		handler.SearchPlaylists = []web.Handler{func(c *contextmodel.ReqContext) {
			client, ok := clientGetter(c)
			if !ok {
				return // error is already sent
			}
			out, err := client.List(c.Req.Context(), v1.ListOptions{})
			if err != nil {
				errorWriter(c, err)
				return
			}

			query := strings.ToUpper(c.Query("query"))
			playlists := []playlist.Playlist{}
			for _, item := range out.Items {
				p := v0alpha1.UnstructuredToLegacyPlaylist(item)
				if p == nil {
					continue
				}
				if query != "" && !strings.Contains(strings.ToUpper(p.Name), query) {
					continue // query filter
				}
				playlists = append(playlists, *p)
			}
			c.JSON(http.StatusOK, playlists)
		}}

		handler.GetPlaylist = []web.Handler{func(c *contextmodel.ReqContext) {
			client, ok := clientGetter(c)
			if !ok {
				return // error is already sent
			}
			uid := web.Params(c.Req)[":uid"]
			out, err := client.Get(c.Req.Context(), uid, v1.GetOptions{})
			if err != nil {
				errorWriter(c, err)
				return
			}
			c.JSON(http.StatusOK, v0alpha1.UnstructuredToLegacyPlaylistDTO(*out))
		}}

		handler.GetPlaylistItems = []web.Handler{func(c *contextmodel.ReqContext) {
			client, ok := clientGetter(c)
			if !ok {
				return // error is already sent
			}
			uid := web.Params(c.Req)[":uid"]
			out, err := client.Get(c.Req.Context(), uid, v1.GetOptions{})
			if err != nil {
				errorWriter(c, err)
				return
			}
			c.JSON(http.StatusOK, v0alpha1.UnstructuredToLegacyPlaylistDTO(*out).Items)
		}}
	}

	// Register the actual handlers
	apiRoute.Group("/playlists", func(playlistRoute routing.RouteRegister) {
		playlistRoute.Get("/", handler.SearchPlaylists...)
		playlistRoute.Get("/:uid", handler.GetPlaylist...)
		playlistRoute.Get("/:uid/items", handler.GetPlaylistItems...)
		playlistRoute.Delete("/:uid", handler.DeletePlaylist...)
		playlistRoute.Put("/:uid", handler.UpdatePlaylist...)
		playlistRoute.Post("/", handler.CreatePlaylist...)
	})
}

func (hs *HTTPServer) validateOrgPlaylist(c *contextmodel.ReqContext) {
	uid := web.Params(c.Req)[":uid"]
	query := playlist.GetPlaylistByUidQuery{UID: uid, OrgId: c.SignedInUser.GetOrgID()}
	p, err := hs.playlistService.GetWithoutItems(c.Req.Context(), &query)

	if err != nil {
		c.JsonApiErr(404, "Playlist not found", err)
		return
	}

	if p.OrgId == 0 {
		c.JsonApiErr(404, "Playlist not found", err)
		return
	}

	if p.OrgId != c.SignedInUser.GetOrgID() {
		c.JsonApiErr(403, "You are not allowed to edit/view playlist", nil)
		return
	}
}

// swagger:route GET /playlists playlists searchPlaylists
//
// Get playlists.
//
// Responses:
// 200: searchPlaylistsResponse
// 500: internalServerError
func (hs *HTTPServer) SearchPlaylists(c *contextmodel.ReqContext) response.Response {
	query := c.Query("query")
	limit := c.QueryInt("limit")

	if limit == 0 {
		limit = 1000
	}

	searchQuery := playlist.GetPlaylistsQuery{
		Name:  query,
		Limit: limit,
		OrgId: c.SignedInUser.GetOrgID(),
	}

	playlists, err := hs.playlistService.Search(c.Req.Context(), &searchQuery)
	if err != nil {
		return response.Error(500, "Search failed", err)
	}

	return response.JSON(http.StatusOK, playlists)
}

// swagger:route GET /playlists/{uid} playlists getPlaylist
//
// Get playlist.
//
// Responses:
// 200: getPlaylistResponse
// 401: unauthorisedError
// 403: forbiddenError
// 404: notFoundError
// 500: internalServerError
func (hs *HTTPServer) GetPlaylist(c *contextmodel.ReqContext) response.Response {
	uid := web.Params(c.Req)[":uid"]
	cmd := playlist.GetPlaylistByUidQuery{UID: uid, OrgId: c.SignedInUser.GetOrgID()}

	dto, err := hs.playlistService.Get(c.Req.Context(), &cmd)
	if err != nil {
		return response.Error(500, "Playlist not found", err)
	}

	return response.JSON(http.StatusOK, dto)
}

// swagger:route GET /playlists/{uid}/items playlists getPlaylistItems
//
// Get playlist items.
//
// Responses:
// 200: getPlaylistItemsResponse
// 401: unauthorisedError
// 403: forbiddenError
// 404: notFoundError
// 500: internalServerError
func (hs *HTTPServer) GetPlaylistItems(c *contextmodel.ReqContext) response.Response {
	uid := web.Params(c.Req)[":uid"]
	cmd := playlist.GetPlaylistByUidQuery{UID: uid, OrgId: c.SignedInUser.GetOrgID()}

	dto, err := hs.playlistService.Get(c.Req.Context(), &cmd)
	if err != nil {
		return response.Error(500, "Playlist not found", err)
	}

	return response.JSON(http.StatusOK, dto.Items)
}

// swagger:route DELETE /playlists/{uid} playlists deletePlaylist
//
// Delete playlist.
//
// Responses:
// 200: okResponse
// 401: unauthorisedError
// 403: forbiddenError
// 404: notFoundError
// 500: internalServerError
func (hs *HTTPServer) DeletePlaylist(c *contextmodel.ReqContext) response.Response {
	uid := web.Params(c.Req)[":uid"]

	cmd := playlist.DeletePlaylistCommand{UID: uid, OrgId: c.SignedInUser.GetOrgID()}
	if err := hs.playlistService.Delete(c.Req.Context(), &cmd); err != nil {
		return response.Error(500, "Failed to delete playlist", err)
	}

	return response.JSON(http.StatusOK, "")
}

// swagger:route POST /playlists playlists createPlaylist
//
// Create playlist.
//
// Responses:
// 200: createPlaylistResponse
// 401: unauthorisedError
// 403: forbiddenError
// 404: notFoundError
// 500: internalServerError
func (hs *HTTPServer) CreatePlaylist(c *contextmodel.ReqContext) response.Response {
	cmd := playlist.CreatePlaylistCommand{}
	if err := web.Bind(c.Req, &cmd); err != nil {
		return response.Error(http.StatusBadRequest, "bad request data", err)
	}
	cmd.OrgId = c.SignedInUser.GetOrgID()

	p, err := hs.playlistService.Create(c.Req.Context(), &cmd)
	if err != nil {
		return response.Error(500, "Failed to create playlist", err)
	}

	return response.JSON(http.StatusOK, p)
}

// swagger:route PUT /playlists/{uid} playlists updatePlaylist
//
// Update playlist.
//
// Responses:
// 200: updatePlaylistResponse
// 401: unauthorisedError
// 403: forbiddenError
// 404: notFoundError
// 500: internalServerError
func (hs *HTTPServer) UpdatePlaylist(c *contextmodel.ReqContext) response.Response {
	cmd := playlist.UpdatePlaylistCommand{}
	if err := web.Bind(c.Req, &cmd); err != nil {
		return response.Error(http.StatusBadRequest, "bad request data", err)
	}
	cmd.OrgId = c.SignedInUser.GetOrgID()
	cmd.UID = web.Params(c.Req)[":uid"]

	_, err := hs.playlistService.Update(c.Req.Context(), &cmd)
	if err != nil {
		return response.Error(500, "Failed to save playlist", err)
	}

	dto, err := hs.playlistService.Get(c.Req.Context(), &playlist.GetPlaylistByUidQuery{
		UID:   cmd.UID,
		OrgId: c.SignedInUser.GetOrgID(),
	})
	if err != nil {
		return response.Error(500, "Failed to load playlist", err)
	}
	return response.JSON(http.StatusOK, dto)
}

// swagger:parameters searchPlaylists
type SearchPlaylistsParams struct {
	// in:query
	// required:false
	Query string `json:"query"`
	// in:limit
	// required:false
	Limit int `json:"limit"`
}

// swagger:parameters getPlaylist
type GetPlaylistParams struct {
	// in:path
	// required:true
	UID string `json:"uid"`
}

// swagger:parameters getPlaylistItems
type GetPlaylistItemsParams struct {
	// in:path
	// required:true
	UID string `json:"uid"`
}

// swagger:parameters getPlaylistDashboards
type GetPlaylistDashboardsParams struct {
	// in:path
	// required:true
	UID string `json:"uid"`
}

// swagger:parameters deletePlaylist
type DeletePlaylistParams struct {
	// in:path
	// required:true
	UID string `json:"uid"`
}

// swagger:parameters updatePlaylist
type UpdatePlaylistParams struct {
	// in:body
	// required:true
	Body playlist.UpdatePlaylistCommand
	// in:path
	// required:true
	UID string `json:"uid"`
}

// swagger:parameters createPlaylist
type CreatePlaylistParams struct {
	// in:body
	// required:true
	Body playlist.CreatePlaylistCommand
}

// swagger:response searchPlaylistsResponse
type SearchPlaylistsResponse struct {
	// The response message
	// in: body
	Body playlist.Playlists `json:"body"`
}

// swagger:response getPlaylistResponse
type GetPlaylistResponse struct {
	// The response message
	// in: body
	Body *playlist.PlaylistDTO `json:"body"`
}

// swagger:response getPlaylistItemsResponse
type GetPlaylistItemsResponse struct {
	// The response message
	// in: body
	Body []playlist.PlaylistItemDTO `json:"body"`
}

// swagger:response getPlaylistDashboardsResponse
type GetPlaylistDashboardsResponse struct {
	// The response message
	// in: body
	Body dtos.PlaylistDashboardsSlice `json:"body"`
}

// swagger:response updatePlaylistResponse
type UpdatePlaylistResponse struct {
	// The response message
	// in: body
	Body *playlist.PlaylistDTO `json:"body"`
}

// swagger:response createPlaylistResponse
type CreatePlaylistResponse struct {
	// The response message
	// in: body
	Body *playlist.Playlist `json:"body"`
}
