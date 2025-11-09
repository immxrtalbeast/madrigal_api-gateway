package handlers

import (
	"context"
	"io"
	"net/http"
	"strings"
	"time"

	"log/slog"

	"github.com/gin-gonic/gin"
	"github.com/immxrtalbeast/api-gateway/internal/clients/videos"
)

type VideoHandler struct {
	log     *slog.Logger
	client  *videos.Client
	timeout time.Duration
}

func NewVideoHandler(log *slog.Logger, client *videos.Client, timeout time.Duration) *VideoHandler {
	return &VideoHandler{log: log, client: client, timeout: timeout}
}

func (h *VideoHandler) CreateVideo(c *gin.Context) {
	body, err := readJSONBody(c.Request.Body)
	if err != nil {
		writeError(c, http.StatusBadRequest, "failed to read request body")
		return
	}
	ctx, cancel := context.WithTimeout(c.Request.Context(), h.timeout)
	defer cancel()

	resp, err := h.client.CreateVideo(ctx, body)
	if err != nil {
		h.log.Error("video create failed", slog.String("err", err.Error()))
		writeError(c, http.StatusBadGateway, "video service error")
		return
	}
	forwardResponse(c, resp)
}

func (h *VideoHandler) ListVideos(c *gin.Context) {
	ctx, cancel := context.WithTimeout(c.Request.Context(), h.timeout)
	defer cancel()

	resp, err := h.client.ListVideos(ctx)
	if err != nil {
		h.log.Error("list videos failed", slog.String("err", err.Error()))
		writeError(c, http.StatusBadGateway, "video service error")
		return
	}
	forwardResponse(c, resp)
}

func (h *VideoHandler) GetVideo(c *gin.Context) {
	videoID := c.Param("id")
	ctx, cancel := context.WithTimeout(c.Request.Context(), h.timeout)
	defer cancel()

	resp, err := h.client.GetVideo(ctx, videoID)
	if err != nil {
		h.log.Error("get video failed", slog.String("err", err.Error()))
		writeError(c, http.StatusBadGateway, "video service error")
		return
	}
	forwardResponse(c, resp)
}

func (h *VideoHandler) ExpandIdea(c *gin.Context) {
	body, err := readJSONBody(c.Request.Body)
	if err != nil {
		writeError(c, http.StatusBadRequest, "failed to read request body")
		return
	}
	ctx, cancel := context.WithTimeout(c.Request.Context(), h.timeout)
	defer cancel()

	resp, err := h.client.ExpandIdea(ctx, body)
	if err != nil {
		h.log.Error("idea expand failed", slog.String("err", err.Error()))
		writeError(c, http.StatusBadGateway, "idea service error")
		return
	}
	forwardResponse(c, resp)
}

func readJSONBody(body io.Reader) ([]byte, error) {
	if body == nil {
		return nil, nil
	}
	return io.ReadAll(io.LimitReader(body, 1<<20))
}

func forwardResponse(c *gin.Context, resp *videos.Response) {
	for k, v := range resp.Header {
		if strings.EqualFold(k, "Content-Length") {
			continue
		}
		for _, value := range v {
			c.Writer.Header().Add(k, value)
		}
	}
	if c.Writer.Header().Get("Content-Type") == "" {
		c.Writer.Header().Set("Content-Type", "application/json")
	}
	c.Status(resp.StatusCode)
	if len(resp.Body) > 0 {
		if _, err := c.Writer.Write(resp.Body); err != nil {
			c.Error(err)
		}
	}
}
