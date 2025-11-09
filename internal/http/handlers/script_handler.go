package handlers

import (
	"context"
	"io"
	"net/http"
	"strings"
	"time"

	"log/slog"

	"github.com/gin-gonic/gin"
	"github.com/immxrtalbeast/api-gateway/internal/clients/scripts"
)

type ScriptHandler struct {
	log     *slog.Logger
	client  *scripts.Client
	timeout time.Duration
}

func NewScriptHandler(log *slog.Logger, client *scripts.Client, timeout time.Duration) *ScriptHandler {
	return &ScriptHandler{log: log, client: client, timeout: timeout}
}

func (h *ScriptHandler) CreateScript(c *gin.Context) {
	body, err := io.ReadAll(io.LimitReader(c.Request.Body, 1<<20))
	if err != nil {
		writeError(c, http.StatusBadRequest, "failed to read request body")
		return
	}
	ctx, cancel := context.WithTimeout(c.Request.Context(), h.timeout)
	defer cancel()

	resp, err := h.client.CreateScript(ctx, body)
	if err != nil {
		h.log.Error("script create failed", slog.String("err", err.Error()))
		writeError(c, http.StatusBadGateway, "script service error")
		return
	}
	h.forwardResponse(c, resp)
}

func (h *ScriptHandler) ListScripts(c *gin.Context) {
	ctx, cancel := context.WithTimeout(c.Request.Context(), h.timeout)
	defer cancel()

	resp, err := h.client.ListScripts(ctx)
	if err != nil {
		h.log.Error("list scripts failed", slog.String("err", err.Error()))
		writeError(c, http.StatusBadGateway, "script service error")
		return
	}
	h.forwardResponse(c, resp)
}

func (h *ScriptHandler) forwardResponse(c *gin.Context, resp *scripts.Response) {
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
			h.log.Error("write response failed", slog.String("err", err.Error()))
		}
	}
}
