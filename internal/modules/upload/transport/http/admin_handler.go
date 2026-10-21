package uploadhttp

import (
	"context"
	"errors"
	"mime/multipart"
	"net/http"

	"github.com/zxgad3wdasaq1e-ai/faka-58/internal/logger"
	contentapp "github.com/zxgad3wdasaq1e-ai/faka-58/internal/modules/content/application"
	contentdomain "github.com/zxgad3wdasaq1e-ai/faka-58/internal/modules/content/domain"
	uploadcontract "github.com/zxgad3wdasaq1e-ai/faka-58/internal/modules/upload/contract"
	"github.com/zxgad3wdasaq1e-ai/faka-58/internal/platform/http/ginutil"
	"github.com/zxgad3wdasaq1e-ai/faka-58/internal/platform/http/response"

	"github.com/gin-gonic/gin"
)

// FileUploader 是文件落盘端口。
type FileUploader interface {
	SaveFileWithMeta(file *multipart.FileHeader, scene string) (*uploadcontract.Result, error)
}

// MediaRecorder 是上传 HTTP 消费方所需的最小 Content 写入接口。
type MediaRecorder interface {
	RecordMedia(ctx context.Context, result contentapp.UploadResult, scene string) (*contentdomain.Media, error)
}

// AdminHandler 处理后台文件上传请求。
type AdminHandler struct {
	uploader FileUploader
	media    MediaRecorder
}

func NewAdminHandler(uploader FileUploader, media MediaRecorder) *AdminHandler {
	if uploader == nil || media == nil {
		panic("upload admin handler: required dependency is nil")
	}
	return &AdminHandler{uploader: uploader, media: media}
}

// UploadFile 文件上传
func (h *AdminHandler) UploadFile(c *gin.Context) {
	if limits, ok := h.uploader.(interface{ MaxUploadSize() int64 }); ok {
		limit := limits.MaxUploadSize()
		// 除文件内容外，为 multipart 字段和边界保留 1 MiB。
		bodyLimit := limit + (1 << 20)
		if limit > 0 && bodyLimit > limit {
			c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, bodyLimit)
		}
	}
	file, err := c.FormFile("file")
	if err != nil {
		var tooLarge *http.MaxBytesError
		if errors.As(err, &tooLarge) {
			ginutil.RespondError(c, response.CodeBadRequest, "error.upload_size_limit", nil)
			return
		}
		ginutil.RespondError(c, response.CodeBadRequest, "error.file_missing", nil)
		return
	}
	scene := c.DefaultPostForm("scene", "common")

	result, err := h.uploader.SaveFileWithMeta(file, scene)
	if err != nil {
		var validation *uploadcontract.ValidationError
		if errors.As(err, &validation) && validation.Key != "" {
			ginutil.RespondError(c, response.CodeBadRequest, validation.Key, nil)
			return
		}
		if isUploadValidationError(err) {
			ginutil.RespondErrorWithMsg(c, response.CodeBadRequest, err.Error(), nil)
			return
		}
		ginutil.RespondError(c, response.CodeInternal, "error.upload_failed", err)
		return
	}

	var mediaID uint
	media, err := h.media.RecordMedia(c.Request.Context(), contentapp.UploadResult{
		URL:      result.URL,
		Filename: result.Filename,
		MimeType: result.MimeType,
		Size:     result.Size,
		Width:    result.Width,
		Height:   result.Height,
	}, scene)
	if err != nil {
		logger.Warnw("upload_record_media_failed", "error", err, "url", result.URL)
	} else if media != nil {
		mediaID = media.ID
	}

	response.Success(c, gin.H{
		"url":      result.URL,
		"filename": result.Filename,
		"size":     result.Size,
		"media_id": mediaID,
	})
}

func isUploadValidationError(err error) bool {
	var marker interface{ UploadValidationError() }
	return errors.As(err, &marker)
}
