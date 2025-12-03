package handlers

import (
	"fmt"
	"io"
	"net/http"
	"strings"
	"vpnpanel/internal/repository"

	"github.com/gin-gonic/gin"
)

type MediaController struct {
	repo *repository.StorageRepo
}

func NewMediaController(repo *repository.StorageRepo) *MediaController {
	return &MediaController{repo: repo}
}

func (s MediaController) Register(r *gin.RouterGroup) {
	r.GET("/:path", s.GetFile)
}

type MediaPath struct {
	Path string
}

func (c *MediaController) GetFile(ctx *gin.Context) {
	path := strings.TrimPrefix(ctx.Param("path"), "/")
	if path == "" {
		ctx.JSON(http.StatusOK, Response{Success: false, Msg: "failed get path param"})
		return
	}

	// get path from body
	// var path string
	// if err := ctx.ShouldBindJSON(&path); err != nil {
	// 	ctx.JSON(http.StatusOK, Response{Success: false, Msg: "failed get path param:" + err.Error()})
	// 	return
	// }
	// path = strings.Split(path, "-")[0]+"/"+strings.Split(path, "-")[0]
	obj, contentType, size, err := c.repo.GetFile(path)
	if err != nil {
		ctx.JSON(http.StatusOK, Response{Success: false, Msg: "failed get file:" + err.Error()})
		return
	}
	defer obj.Close()

	if contentType == "" {
		contentType = "application/octet-stream"
	}

	ctx.Header("Content-Type", contentType)
	ctx.Header("Content-Length", fmt.Sprintf("%d", size))
	ctx.Header("Cache-Control", "public, max-age=86400")

	if _, err := io.Copy(ctx.Writer, obj); err != nil {
		// логируем ошибку стриминга
		fmt.Println("failed to stream file:", err)
	}
}
