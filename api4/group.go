// Copyright (c) 2018-present Mattermost, Inc. All Rights Reserved.
// See License.txt for license information.

package api4

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"

	"github.com/mattermost/mattermost-server/model"
)

const (
	groupMemberActionCreate = iota
	groupMemberActionDelete
)

func (api *API) InitGroup() {
	// GET /api/v4/groups
	api.BaseRoutes.Groups.Handle("", api.ApiSessionRequired(getGroups)).Methods("GET")

	// GET /api/v4/groups/:group_id
	api.BaseRoutes.Groups.Handle("/{group_id:[A-Za-z0-9]+}",
		api.ApiSessionRequired(getGroup)).Methods("GET")

	// PUT /api/v4/groups/:group_id/patch
	api.BaseRoutes.Groups.Handle("/{group_id:[A-Za-z0-9]+}/patch",
		api.ApiSessionRequired(patchGroup)).Methods("PUT")

	// POST /api/v4/groups/:group_id/teams/:team_id/link
	// POST /api/v4/groups/:group_id/channels/:channel_id/link
	api.BaseRoutes.Groups.Handle("/{group_id:[A-Za-z0-9]+}/{syncable_type:teams|channels}/{syncable_id:[A-Za-z0-9]+}/link",
		api.ApiSessionRequired(linkGroupSyncable)).Methods("POST")

	// DELETE /api/v4/groups/:group_id/teams/:team_id/link
	// DELETE /api/v4/groups/:group_id/channels/:channel_id/link
	api.BaseRoutes.Groups.Handle("/{group_id:[A-Za-z0-9]+}/{syncable_type:teams|channels}/{syncable_id:[A-Za-z0-9]+}/link",
		api.ApiSessionRequired(unlinkGroupSyncable)).Methods("DELETE")

	// GET /api/v4/groups/:group_id/teams/:team_id
	// GET /api/v4/groups/:group_id/channels/:channel_id
	api.BaseRoutes.Groups.Handle("/{group_id:[A-Za-z0-9]+}/{syncable_type:teams|channels}/{syncable_id:[A-Za-z0-9]+}",
		api.ApiSessionRequired(getGroupSyncable)).Methods("GET")

	// GET /api/v4/groups/:group_id/teams
	// GET /api/v4/groups/:group_id/channels
	api.BaseRoutes.Groups.Handle("/{group_id:[A-Za-z0-9]+}/{syncable_type:teams|channels}",
		api.ApiSessionRequired(getGroupSyncables)).Methods("GET")

	// PUT /api/v4/groups/:group_id/teams/:team_id/patch
	// PUT /api/v4/groups/:group_id/channels/:channel_id/patch
	api.BaseRoutes.Groups.Handle("/{group_id:[A-Za-z0-9]+}/{syncable_type:teams|channels}/{syncable_id:[A-Za-z0-9]+}/patch",
		api.ApiSessionRequired(patchGroupSyncable)).Methods("PUT")

	// GET /api/v4/groups/:group_id/members?page=0&per_page=100
	api.BaseRoutes.Groups.Handle("/{group_id:[A-Za-z0-9]+}/members",
		api.ApiSessionRequired(getGroupMembers)).Methods("GET")

	// GET /api/v4/channels/:channel_id/groups?page=0&per_page=100
	api.BaseRoutes.Channels.Handle("/{channel_id:[A-Za-z0-9]+}/groups",
		api.ApiSessionRequired(getGroupsByChannel)).Methods("GET")

	// GET /api/v4/teams/:team_id/groups?page=0&per_page=100
	api.BaseRoutes.Teams.Handle("/{team_id:[A-Za-z0-9]+}/groups",
		api.ApiSessionRequired(getGroupsByTeam)).Methods("GET")
}

func getGroups(c *Context, w http.ResponseWriter, r *http.Request) {
	if c.App.License() == nil || !*c.App.License().Features.LDAPGroups {
		c.Err = model.NewAppError("Api4.getGroups", "api.ldap_groups.license_error", nil, "", http.StatusNotImplemented)
		return
	}

	// TODO: Should the ability to list and search groups be behind a permission check?

	opts := model.GroupSearchOpts{}
	if len(c.Params.Q) > 1 {
		opts.Q = &c.Params.Q
	}
	if len(c.Params.NotAssociatedToTeam) == 26 {
		teamID := &c.Params.NotAssociatedToTeam
		if !c.App.SessionHasPermissionToTeam(c.App.Session, *teamID, model.PERMISSION_MANAGE_TEAM) {
			c.SetPermissionError(model.PERMISSION_MANAGE_TEAM)
			return
		}
		opts.NotAssociatedToTeam = teamID
	}
	if c.Params.IncludeMemberCount {
		opts.IncludeMemberCount = true
	}

	groups, err := c.App.GetGroupsPage(c.Params.Page, c.Params.PerPage, opts)
	if err != nil {
		c.Err = err
		return
	}

	b, marshalErr := json.Marshal(groups)
	if marshalErr != nil {
		c.Err = model.NewAppError("Api4.getGroups", "api.marshal_error", nil, marshalErr.Error(), http.StatusInternalServerError)
		return
	}

	w.Write(b)
}

func getGroup(c *Context, w http.ResponseWriter, r *http.Request) {
	c.RequireGroupId()
	if c.Err != nil {
		return
	}

	if c.App.License() == nil || !*c.App.License().Features.LDAPGroups {
		c.Err = model.NewAppError("Api4.getGroup", "api.ldap_groups.license_error", nil, "", http.StatusNotImplemented)
		return
	}

	if !c.App.SessionHasPermissionTo(c.App.Session, model.PERMISSION_MANAGE_SYSTEM) {
		c.SetPermissionError(model.PERMISSION_MANAGE_SYSTEM)
		return
	}

	group, err := c.App.GetGroup(c.Params.GroupId)
	if err != nil {
		c.Err = err
		return
	}

	b, marshalErr := json.Marshal(group)
	if marshalErr != nil {
		c.Err = model.NewAppError("Api4.getGroup", "api.marshal_error", nil, marshalErr.Error(), http.StatusInternalServerError)
		return
	}

	w.Write(b)
}

func patchGroup(c *Context, w http.ResponseWriter, r *http.Request) {
	c.RequireGroupId()
	if c.Err != nil {
		return
	}

	groupPatch := model.GroupPatchFromJson(r.Body)
	if groupPatch == nil {
		c.SetInvalidParam("group")
		return
	}

	if c.App.License() == nil || !*c.App.License().Features.LDAPGroups {
		c.Err = model.NewAppError("Api4.patchGroup", "api.ldap_groups.license_error", nil, "", http.StatusNotImplemented)
		return
	}

	if !c.App.SessionHasPermissionTo(c.App.Session, model.PERMISSION_MANAGE_SYSTEM) {
		c.SetPermissionError(model.PERMISSION_MANAGE_SYSTEM)
		return
	}

	group, err := c.App.GetGroup(c.Params.GroupId)
	if err != nil {
		c.Err = err
		return
	}

	group.Patch(groupPatch)

	group, err = c.App.UpdateGroup(group)
	if err != nil {
		c.Err = err
		return
	}

	b, marshalErr := json.Marshal(group)
	if marshalErr != nil {
		c.Err = model.NewAppError("Api4.patchGroup", "api.marshal_error", nil, marshalErr.Error(), http.StatusInternalServerError)
		return
	}

	w.Write(b)
}

func linkGroupSyncable(c *Context, w http.ResponseWriter, r *http.Request) {
	c.RequireGroupId()
	if c.Err != nil {
		return
	}

	c.RequireSyncableId()
	if c.Err != nil {
		return
	}
	syncableID := c.Params.SyncableId

	c.RequireSyncableType()
	if c.Err != nil {
		return
	}
	syncableType := c.Params.SyncableType

	body, err := ioutil.ReadAll(r.Body)
	if err != nil {
		c.Err = model.NewAppError("Api4.createGroupSyncable", "api.io_error", nil, err.Error(), http.StatusBadRequest)
		return
	}

	var patch *model.GroupSyncablePatch
	err = json.Unmarshal(body, &patch)
	if err != nil || patch == nil {
		c.SetInvalidParam(fmt.Sprintf("Group%s", syncableType.String()))
		return
	}

	if c.App.License() == nil || !*c.App.License().Features.LDAPGroups {
		c.Err = model.NewAppError("Api4.createGroupSyncable", "api.ldap_groups.license_error", nil, "", http.StatusNotImplemented)
		return
	}

	switch syncableType {
	case model.GroupSyncableTypeTeam:
		if !c.App.SessionHasPermissionToTeam(c.App.Session, syncableID, model.PERMISSION_MANAGE_TEAM) {
			c.SetPermissionError(model.PERMISSION_MANAGE_TEAM)
			return
		}
	case model.GroupSyncableTypeChannel:
		if !c.App.SessionHasPermissionToChannel(c.App.Session, syncableID, model.PERMISSION_MANAGE_PUBLIC_CHANNEL_MEMBERS) {
			c.SetPermissionError(model.PERMISSION_MANAGE_PRIVATE_CHANNEL_MEMBERS)
			return
		}
	}

	groupSyncable, appErr := c.App.GetGroupSyncable(c.Params.GroupId, syncableID, syncableType)
	if appErr != nil && appErr.DetailedError != sql.ErrNoRows.Error() {
		c.Err = appErr
		return
	}

	if groupSyncable == nil {
		groupSyncable = &model.GroupSyncable{
			GroupId:    c.Params.GroupId,
			SyncableId: syncableID,
			Type:       syncableType,
		}
		groupSyncable.Patch(patch)
		groupSyncable, appErr = c.App.CreateGroupSyncable(groupSyncable)
		if appErr != nil {
			c.Err = appErr
			return
		}
	} else {
		groupSyncable.DeleteAt = 0
		groupSyncable.Patch(patch)
		groupSyncable, appErr = c.App.UpdateGroupSyncable(groupSyncable)
		if appErr != nil {
			c.Err = appErr
			return
		}
	}

	w.WriteHeader(http.StatusCreated)

	b, marshalErr := json.Marshal(groupSyncable)
	if marshalErr != nil {
		c.Err = model.NewAppError("Api4.createGroupSyncable", "api.marshal_error", nil, marshalErr.Error(), http.StatusInternalServerError)
		return
	}

	w.Write(b)
}

func getGroupSyncable(c *Context, w http.ResponseWriter, r *http.Request) {
	c.RequireGroupId()
	if c.Err != nil {
		return
	}

	c.RequireSyncableId()
	if c.Err != nil {
		return
	}
	syncableID := c.Params.SyncableId

	c.RequireSyncableType()
	if c.Err != nil {
		return
	}
	syncableType := c.Params.SyncableType

	if c.App.License() == nil || !*c.App.License().Features.LDAPGroups {
		c.Err = model.NewAppError("Api4.getGroupSyncable", "api.ldap_groups.license_error", nil, "", http.StatusNotImplemented)
		return
	}

	if !c.App.SessionHasPermissionTo(c.App.Session, model.PERMISSION_MANAGE_SYSTEM) {
		c.SetPermissionError(model.PERMISSION_MANAGE_SYSTEM)
		return
	}

	groupSyncable, err := c.App.GetGroupSyncable(c.Params.GroupId, syncableID, syncableType)
	if err != nil {
		c.Err = err
		return
	}

	b, marshalErr := json.Marshal(groupSyncable)
	if marshalErr != nil {
		c.Err = model.NewAppError("Api4.getGroupSyncable", "api.marshal_error", nil, marshalErr.Error(), http.StatusInternalServerError)
		return
	}

	w.Write(b)
}

func getGroupSyncables(c *Context, w http.ResponseWriter, r *http.Request) {
	c.RequireGroupId()
	if c.Err != nil {
		return
	}

	c.RequireSyncableType()
	if c.Err != nil {
		return
	}
	syncableType := c.Params.SyncableType

	if c.App.License() == nil || !*c.App.License().Features.LDAPGroups {
		c.Err = model.NewAppError("Api4.getGroupSyncables", "api.ldap_groups.license_error", nil, "", http.StatusNotImplemented)
		return
	}

	if !c.App.SessionHasPermissionTo(c.App.Session, model.PERMISSION_MANAGE_SYSTEM) {
		c.SetPermissionError(model.PERMISSION_MANAGE_SYSTEM)
		return
	}

	groupSyncables, err := c.App.GetGroupSyncables(c.Params.GroupId, syncableType)
	if err != nil {
		c.Err = err
		return
	}

	b, marshalErr := json.Marshal(groupSyncables)
	if marshalErr != nil {
		c.Err = model.NewAppError("Api4.getGroupSyncables", "api.marshal_error", nil, marshalErr.Error(), http.StatusInternalServerError)
		return
	}

	w.Write(b)
}

func patchGroupSyncable(c *Context, w http.ResponseWriter, r *http.Request) {
	c.RequireGroupId()
	if c.Err != nil {
		return
	}

	c.RequireSyncableId()
	if c.Err != nil {
		return
	}
	syncableID := c.Params.SyncableId

	c.RequireSyncableType()
	if c.Err != nil {
		return
	}
	syncableType := c.Params.SyncableType

	body, err := ioutil.ReadAll(r.Body)
	if err != nil {
		c.Err = model.NewAppError("Api4.patchGroupSyncable", "api.io_error", nil, err.Error(), http.StatusBadRequest)
		return
	}

	var patch *model.GroupSyncablePatch
	err = json.Unmarshal(body, &patch)
	if err != nil || patch == nil {
		c.SetInvalidParam(fmt.Sprintf("Group[%s]Patch", syncableType.String()))
		return
	}

	if c.App.License() == nil || !*c.App.License().Features.LDAPGroups {
		c.Err = model.NewAppError("Api4.patchGroupSyncable", "api.ldap_groups.license_error", nil, "",
			http.StatusNotImplemented)
		return
	}

	if !c.App.SessionHasPermissionTo(c.App.Session, model.PERMISSION_MANAGE_SYSTEM) {
		c.SetPermissionError(model.PERMISSION_MANAGE_SYSTEM)
		return
	}

	groupSyncable, appErr := c.App.GetGroupSyncable(c.Params.GroupId, syncableID, syncableType)
	if appErr != nil {
		c.Err = appErr
		return
	}

	groupSyncable.Patch(patch)

	groupSyncable, appErr = c.App.UpdateGroupSyncable(groupSyncable)
	if appErr != nil {
		c.Err = appErr
		return
	}

	b, marshalErr := json.Marshal(groupSyncable)
	if marshalErr != nil {
		c.Err = model.NewAppError("Api4.patchGroupSyncable", "api.marshal_error", nil, marshalErr.Error(), http.StatusInternalServerError)
		return
	}

	w.Write(b)
}

func unlinkGroupSyncable(c *Context, w http.ResponseWriter, r *http.Request) {
	c.RequireGroupId()
	if c.Err != nil {
		return
	}

	c.RequireSyncableId()
	if c.Err != nil {
		return
	}
	syncableID := c.Params.SyncableId

	c.RequireSyncableType()
	if c.Err != nil {
		return
	}
	syncableType := c.Params.SyncableType

	if c.App.License() == nil || !*c.App.License().Features.LDAPGroups {
		c.Err = model.NewAppError("Api4.unlinkGroupSyncable", "api.ldap_groups.license_error", nil, "", http.StatusNotImplemented)
		return
	}

	if !c.App.SessionHasPermissionTo(c.App.Session, model.PERMISSION_MANAGE_SYSTEM) {
		c.SetPermissionError(model.PERMISSION_MANAGE_SYSTEM)
		return
	}

	_, err := c.App.DeleteGroupSyncable(c.Params.GroupId, syncableID, syncableType)
	if err != nil {
		c.Err = err
		return
	}

	ReturnStatusOK(w)
}

func getGroupMembers(c *Context, w http.ResponseWriter, r *http.Request) {
	c.RequireGroupId()
	if c.Err != nil {
		return
	}

	if c.App.License() == nil || !*c.App.License().Features.LDAPGroups {
		c.Err = model.NewAppError("Api4.getGroupMembers", "api.ldap_groups.license_error", nil, "", http.StatusNotImplemented)
		return
	}

	if !c.App.SessionHasPermissionTo(c.App.Session, model.PERMISSION_MANAGE_SYSTEM) {
		c.SetPermissionError(model.PERMISSION_MANAGE_SYSTEM)
		return
	}

	members, count, err := c.App.GetGroupMemberUsersPage(c.Params.GroupId, c.Params.Page, c.Params.PerPage)
	if err != nil {
		c.Err = err
		return
	}

	b, marshalErr := json.Marshal(struct {
		Members []*model.User `json:"members"`
		Count   int           `json:"total_member_count"`
	}{
		Members: members,
		Count:   count,
	})
	if marshalErr != nil {
		c.Err = model.NewAppError("Api4.getGroupMembers", "api.marshal_error", nil, marshalErr.Error(), http.StatusInternalServerError)
		return
	}

	w.Write(b)
}

func getGroupsByChannel(c *Context, w http.ResponseWriter, r *http.Request) {
	c.RequireChannelId()
	if c.Err != nil {
		return
	}

	if c.App.License() == nil || !*c.App.License().Features.LDAPGroups {
		c.Err = model.NewAppError("Api4.getGroupsByChannel", "api.ldap_groups.license_error", nil, "", http.StatusNotImplemented)
		return
	}

	if !c.App.SessionHasPermissionTo(c.App.Session, model.PERMISSION_MANAGE_SYSTEM) {
		c.SetPermissionError(model.PERMISSION_MANAGE_SYSTEM)
		return
	}

	groups, err := c.App.GetGroupsByChannel(c.Params.ChannelId, c.Params.Page, c.Params.PerPage)
	if err != nil {
		c.Err = err
		return
	}

	b, marshalErr := json.Marshal(groups)
	if marshalErr != nil {
		c.Err = model.NewAppError("Api4.getGroupsByChannel", "api.marshal_error", nil, marshalErr.Error(), http.StatusInternalServerError)
		return
	}

	w.Write(b)
}

func getGroupsByTeam(c *Context, w http.ResponseWriter, r *http.Request) {
	c.RequireTeamId()
	if c.Err != nil {
		return
	}

	if c.App.License() == nil || !*c.App.License().Features.LDAPGroups {
		c.Err = model.NewAppError("Api4.getGroupsByTeam", "api.ldap_groups.license_error", nil, "", http.StatusNotImplemented)
		return
	}

	if !c.App.SessionHasPermissionToTeam(c.App.Session, c.Params.TeamId, model.PERMISSION_MANAGE_TEAM) {
		c.SetPermissionError(model.PERMISSION_MANAGE_TEAM)
		return
	}

	var groups []*model.Group
	var err *model.AppError

	opts := model.GroupSearchOpts{}
	if len(c.Params.Q) > 1 {
		opts.Q = &c.Params.Q
	}
	if c.Params.IncludeMemberCount {
		opts.IncludeMemberCount = true
	}

	if c.Params.Paginate != nil && !*c.Params.Paginate {
		groups, err = c.App.GetGroupsByTeam(c.Params.TeamId, nil, nil, opts)
	} else {
		groups, err = c.App.GetGroupsByTeam(c.Params.TeamId, &c.Params.Page, &c.Params.PerPage, opts)
	}

	if err != nil {
		c.Err = err
		return
	}

	b, marshalErr := json.Marshal(groups)
	if marshalErr != nil {
		c.Err = model.NewAppError("Api4.getGroupsByTeam", "api.marshal_error", nil, marshalErr.Error(), http.StatusInternalServerError)
		return
	}

	w.Write(b)
}
