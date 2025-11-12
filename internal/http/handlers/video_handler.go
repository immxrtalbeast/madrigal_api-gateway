package handlers

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"log/slog"

	"github.com/gin-gonic/gin"
	"github.com/immxrtalbeast/api-gateway/internal/clients/videos"
	"github.com/immxrtalbeast/api-gateway/internal/events"
	"golang.org/x/net/websocket"
)

type VideoHandler struct {
	log       *slog.Logger
	client    *videos.Client
	timeout   time.Duration
	streamHub *events.Hub
}

func NewVideoHandler(log *slog.Logger, client *videos.Client, timeout time.Duration, hub *events.Hub) *VideoHandler {
	return &VideoHandler{log: log, client: client, timeout: timeout, streamHub: hub}
}

func (h *VideoHandler) CreateVideo(c *gin.Context) {
	body, err := readJSONBody(c.Request.Body)
	if err != nil {
		writeError(c, http.StatusBadRequest, "failed to read request body")
		return
	}
	ctx, cancel := context.WithTimeout(c.Request.Context(), h.timeout)
	defer cancel()

	resp, err := h.client.CreateVideo(ctx, body, userHeaders(c))
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

	resp, err := h.client.ListVideos(ctx, userHeaders(c))
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

	resp, err := h.client.GetVideo(ctx, videoID, userHeaders(c))
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

	resp, err := h.client.ExpandIdea(ctx, body, userHeaders(c))
	if err != nil {
		h.log.Error("idea expand failed", slog.String("err", err.Error()))
		writeError(c, http.StatusBadGateway, "idea service error")
		return
	}
	forwardResponse(c, resp)
}

func (h *VideoHandler) ApproveDraft(c *gin.Context) {
	jobID := c.Param("id")
	body, err := readJSONBody(c.Request.Body)
	if err != nil {
		writeError(c, http.StatusBadRequest, "failed to read request body")
		return
	}
	ctx, cancel := context.WithTimeout(c.Request.Context(), h.timeout)
	defer cancel()

	resp, err := h.client.ApproveDraft(ctx, jobID, body, userHeaders(c))
	if err != nil {
		h.log.Error("draft approve failed", slog.String("err", err.Error()))
		writeError(c, http.StatusBadGateway, "video service error")
		return
	}
	forwardResponse(c, resp)
}

func (h *VideoHandler) ApproveSubtitles(c *gin.Context) {
	jobID := c.Param("id")
	body, err := readJSONBody(c.Request.Body)
	if err != nil {
		writeError(c, http.StatusBadRequest, "failed to read request body")
		return
	}
	ctx, cancel := context.WithTimeout(c.Request.Context(), h.timeout)
	defer cancel()

	resp, err := h.client.ApproveSubtitles(ctx, jobID, body, userHeaders(c))
	if err != nil {
		h.log.Error("subtitles approve failed", slog.String("err", err.Error()))
		writeError(c, http.StatusBadGateway, "video service error")
		return
	}
	forwardResponse(c, resp)
}

func (h *VideoHandler) UploadMedia(c *gin.Context) {
	body, err := readJSONBody(c.Request.Body)
	if err != nil {
		writeError(c, http.StatusBadRequest, "failed to read request body")
		return
	}
	ctx, cancel := context.WithTimeout(c.Request.Context(), h.timeout)
	defer cancel()

	resp, err := h.client.UploadMedia(ctx, body, userHeaders(c))
	if err != nil {
		h.log.Error("media upload failed", slog.String("err", err.Error()))
		writeError(c, http.StatusBadGateway, "video service error")
		return
	}
	forwardResponse(c, resp)
}

func (h *VideoHandler) ListMedia(c *gin.Context) {
	folder := c.Query("folder")
	ctx, cancel := context.WithTimeout(c.Request.Context(), h.timeout)
	defer cancel()

	resp, err := h.client.ListMedia(ctx, folder, userHeaders(c))
	if err != nil {
		h.log.Error("media list failed", slog.String("err", err.Error()))
		writeError(c, http.StatusBadGateway, "video service error")
		return
	}
	forwardResponse(c, resp)
}

func (h *VideoHandler) StreamVideo(c *gin.Context) {
	jobID := c.Param("id")
	ws := websocket.Server{
		Handshake: func(config *websocket.Config, req *http.Request) error {
			return nil
		},
		Handler: func(conn *websocket.Conn) {
			defer conn.Close()
			ctx := c.Request.Context()
			if h.streamHub != nil {
				h.handleKafkaStream(ctx, conn, jobID)
				return
			}
			h.handleVideoStream(ctx, conn, jobID)
		},
	}
	ws.ServeHTTP(c.Writer, c.Request)
}

func (h *VideoHandler) handleKafkaStream(ctx context.Context, conn *websocket.Conn, jobID string) {
	body, stage, err := h.fetchJobSnapshot(ctx, jobID)
	if err != nil {
		websocket.Message.Send(conn, fmt.Sprintf(`{"error":"%s"}`, err.Error()))
		return
	}
	if err := websocket.Message.Send(conn, string(body)); err != nil {
		return
	}
	if stage == "ready" || stage == "failed" {
		return
	}
	updates, cancel := h.streamHub.Subscribe(jobID)
	defer cancel()
	for {
		select {
		case <-ctx.Done():
			return
		case payload, ok := <-updates:
			if !ok {
				return
			}
			if err := websocket.Message.Send(conn, string(payload)); err != nil {
				return
			}
			nextStage, err := extractStage(payload)
			if err != nil {
				continue
			}
			if nextStage == "ready" || nextStage == "failed" {
				return
			}
		}
	}
}

func (h *VideoHandler) handleVideoStream(ctx context.Context, conn *websocket.Conn, jobID string) {
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	var lastHash [32]byte
	sendUpdate := func() (bool, bool) {
		body, stage, err := h.fetchJobSnapshot(ctx, jobID)
		if err != nil {
			websocket.Message.Send(conn, fmt.Sprintf(`{"error":"%s"}`, err.Error()))
			return false, true
		}
		hash := sha256.Sum256(body)
		if hash == lastHash {
			return true, stage == "ready" || stage == "failed"
		}
		lastHash = hash
		if err := websocket.Message.Send(conn, string(body)); err != nil {
			return false, true
		}
		return true, stage == "ready" || stage == "failed"
	}

	if ok, done := sendUpdate(); !ok || done {
		return
	}

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			ok, done := sendUpdate()
			if !ok || done {
				return
			}
		}
	}
}

func (h *VideoHandler) fetchJobSnapshot(ctx context.Context, jobID string) ([]byte, string, error) {
	reqCtx, cancel := context.WithTimeout(ctx, h.timeout)
	defer cancel()
	resp, err := h.client.GetVideo(reqCtx, jobID, nil)
	if err != nil {
		return nil, "", err
	}
	body := append([]byte(nil), resp.Body...)
	stage, err := extractStage(body)
	if err != nil {
		return nil, "", err
	}
	return body, stage, nil
}

type jobStagePayload struct {
	Job struct {
		Stage string `json:"stage"`
	} `json:"job"`
}

func extractStage(body []byte) (string, error) {
	var payload jobStagePayload
	if err := json.Unmarshal(body, &payload); err != nil {
		return "", err
	}
	return payload.Job.Stage, nil
}

func readJSONBody(body io.Reader) ([]byte, error) {
	if body == nil {
		return nil, nil
	}
	return io.ReadAll(io.LimitReader(body, 1<<20))
}

func userHeaders(c *gin.Context) map[string]string {
	userIDVal, exists := c.Get("userID")
	if !exists {
		return nil
	}
	userID := fmt.Sprint(userIDVal)
	if userID == "" {
		return nil
	}
	return map[string]string{"X-User-ID": userID}
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
