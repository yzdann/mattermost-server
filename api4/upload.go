// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package api4

import (
	"net/http"

	"github.com/mattermost/mattermost-server/v5/audit"
	"github.com/mattermost/mattermost-server/v5/model"
)

func (api *API) InitUpload() {
	api.BaseRoutes.Uploads.Handle("", api.ApiSessionRequired(createUpload)).Methods("POST")
	api.BaseRoutes.Upload.Handle("", api.ApiSessionRequired(getUpload)).Methods("GET")
	api.BaseRoutes.Upload.Handle("", api.ApiSessionRequired(uploadData)).Methods("POST")
}

func createUpload(c *Context, w http.ResponseWriter, r *http.Request) {
	if !*c.App.Config().FileSettings.EnableFileAttachments {
		c.Err = model.NewAppError("createUpload",
			"api.file.attachments.disabled.app_error",
			nil, "", http.StatusNotImplemented)
		return
	}

	us := model.UploadSessionFromJson(r.Body)
	if us == nil {
		c.SetInvalidParam("upload")
		return
	}

	auditRec := c.MakeAuditRecord("createUpload", audit.Fail)
	defer c.LogAuditRec(auditRec)
	auditRec.AddMeta("upload", us)

	if !c.App.SessionHasPermissionToChannel(*c.App.Session(), us.ChannelId, model.PERMISSION_UPLOAD_FILE) {
		c.SetPermissionError(model.PERMISSION_UPLOAD_FILE)
		return
	}

	us.Id = model.NewId()
	us.Type = model.UploadTypeAttachment
	us.UserId = c.App.Session().UserId
	us, err := c.App.CreateUploadSession(us)
	if err != nil {
		c.Err = err
		return
	}

	auditRec.Success()
	w.WriteHeader(http.StatusCreated)
	w.Write([]byte(us.ToJson()))
}

func getUpload(c *Context, w http.ResponseWriter, r *http.Request) {
	c.RequireUploadId()
	if c.Err != nil {
		return
	}

	us, err := c.App.GetUploadSession(c.Params.UploadId)
	if err != nil {
		c.Err = err
		return
	}

	if us.UserId != c.App.Session().UserId {
		c.Err = model.NewAppError("getUpload", "api.upload.get_upload.forbidden.app_error", nil, "", http.StatusForbidden)
		return
	}

	w.Write([]byte(us.ToJson()))
}

func uploadData(c *Context, w http.ResponseWriter, r *http.Request) {
	if !*c.App.Config().FileSettings.EnableFileAttachments {
		c.Err = model.NewAppError("uploadData", "api.file.attachments.disabled.app_error",
			nil, "", http.StatusNotImplemented)
		return
	}

	c.RequireUploadId()
	if c.Err != nil {
		return
	}

	auditRec := c.MakeAuditRecord("uploadData", audit.Fail)
	defer c.LogAuditRec(auditRec)
	auditRec.AddMeta("upload_id", c.Params.UploadId)

	us, err := c.App.GetUploadSession(c.Params.UploadId)
	if err != nil {
		c.Err = err
		return
	}

	if r.ContentLength > (us.FileSize - us.FileOffset) {
		c.Err = model.NewAppError("uploadData", "api.upload.upload_data.invalid_content_length",
			nil, "", http.StatusBadRequest)
		return
	}

	if us.UserId != c.App.Session().UserId || !c.App.SessionHasPermissionToChannel(*c.App.Session(), us.ChannelId, model.PERMISSION_UPLOAD_FILE) {
		c.SetPermissionError(model.PERMISSION_UPLOAD_FILE)
		return
	}

	info, err := c.App.UploadData(us, r.Body)
	if err != nil {
		c.Err = err
		return
	}

	auditRec.Success()

	if info == nil {
		w.WriteHeader(http.StatusNoContent)
		return
	}

	w.Write([]byte(info.ToJson()))
}
