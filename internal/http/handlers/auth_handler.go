package handlers

import (
	"context"
	"net/http"
	"strings"
	"time"

	"log/slog"

	"github.com/gin-gonic/gin"
	authv1 "github.com/immxrtalbeast/protos/gen/go/auth/v1"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type AuthHandler struct {
	log      *slog.Logger
	client   authv1.AuthServiceClient
	timeout  time.Duration
	tokenTTL time.Duration
}

func NewAuthHandler(log *slog.Logger, client authv1.AuthServiceClient, timeout, tokenTTL time.Duration) *AuthHandler {
	return &AuthHandler{log: log, client: client, timeout: timeout, tokenTTL: tokenTTL}
}

type registerRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type loginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type refreshRequest struct {
	RefreshToken string `json:"refresh_token"`
}

type logoutRequest struct {
	RefreshToken string `json:"refresh_token"`
}

type userResponse struct {
	ID        string `json:"id"`
	Email     string `json:"email"`
	Role      string `json:"role"`
	CreatedAt string `json:"created_at,omitempty"`
	UpdatedAt string `json:"updated_at,omitempty"`
}

func (h *AuthHandler) Register(c *gin.Context) {
	var req registerRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		writeError(c, http.StatusBadRequest, "invalid json payload")
		return
	}
	req.Email = strings.TrimSpace(req.Email)
	if req.Email == "" || req.Password == "" {
		writeError(c, http.StatusBadRequest, "email and password are required")
		return
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), h.timeout)
	defer cancel()

	resp, err := h.client.Register(ctx, &authv1.RegisterRequest{Email: req.Email, Password: req.Password})
	if err != nil {
		h.handleAuthError(c, err)
		return
	}

	writeJSON(c, http.StatusCreated, map[string]any{"user": convertUser(resp.GetUser())})
}

func (h *AuthHandler) Login(c *gin.Context) {
	var req loginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		writeError(c, http.StatusBadRequest, "invalid json payload")
		return
	}
	req.Email = strings.TrimSpace(req.Email)
	if req.Email == "" || req.Password == "" {
		writeError(c, http.StatusBadRequest, "email and password are required")
		return
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), h.timeout)
	defer cancel()

	resp, err := h.client.Login(ctx, &authv1.LoginRequest{Email: req.Email, Password: req.Password})
	if err != nil {
		h.handleAuthError(c, err)
		return
	}
	c.SetSameSite(http.SameSiteLaxMode)
	c.SetCookie(
		"jwt",
		resp.GetAccessToken(),
		maxAgeSeconds(h.tokenTTL),
		"/",
		"",
		false,
		true,
	)

	writeJSON(c, http.StatusOK, map[string]any{
		"refresh_token": resp.GetRefreshToken(),
		"user":          convertUser(resp.GetUser()),
	})
}

func (h *AuthHandler) RefreshToken(c *gin.Context) {
	var req refreshRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		writeError(c, http.StatusBadRequest, "invalid json payload")
		return
	}
	if strings.TrimSpace(req.RefreshToken) == "" {
		writeError(c, http.StatusBadRequest, "refresh_token is required")
		return
	}
	accessToken, _ := c.Cookie("jwt")
	accessToken = strings.TrimSpace(accessToken)
	ctx, cancel := context.WithTimeout(c.Request.Context(), h.timeout)
	defer cancel()

	resp, err := h.client.RefreshToken(ctx, &authv1.RefreshTokenRequest{
		AccessToken:  accessToken,
		RefreshToken: req.RefreshToken,
	})
	if err != nil {
		h.handleAuthError(c, err)
		return
	}
	c.SetCookie(
		"jwt",
		resp.GetAccessToken(),
		maxAgeSeconds(h.tokenTTL),
		"/",
		"",
		false,
		true,
	)
	writeJSON(c, http.StatusOK, map[string]any{
		"refresh_token": resp.GetRefreshToken(),
	})
}

func (h *AuthHandler) Logout(c *gin.Context) {
	var req logoutRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		writeError(c, http.StatusBadRequest, "invalid json payload")
		return
	}
	if strings.TrimSpace(req.RefreshToken) == "" {
		writeError(c, http.StatusBadRequest, "refresh_token is required")
		return
	}
	accessToken, _ := c.Cookie("jwt")
	accessToken = strings.TrimSpace(accessToken)
	ctx, cancel := context.WithTimeout(c.Request.Context(), h.timeout)
	defer cancel()

	_, err := h.client.Logout(ctx, &authv1.LogoutRequest{
		AccessToken:  accessToken,
		RefreshToken: req.RefreshToken,
	})
	if err != nil {
		h.handleAuthError(c, err)
		return
	}
	c.SetCookie("jwt", "", -1, "/", "", false, true)
	c.Status(http.StatusNoContent)
}

func (h *AuthHandler) GetUser(c *gin.Context) {
	userID := strings.TrimSpace(c.Param("id"))
	if userID == "" {
		writeError(c, http.StatusBadRequest, "user id is required")
		return
	}
	ctx, cancel := context.WithTimeout(c.Request.Context(), h.timeout)
	defer cancel()

	resp, err := h.client.GetUser(ctx, &authv1.GetUserRequest{UserId: userID})
	if err != nil {
		h.handleAuthError(c, err)
		return
	}
	writeJSON(c, http.StatusOK, map[string]any{"user": convertUser(resp.GetUser())})
}

func (h *AuthHandler) IsAdmin(c *gin.Context) {
	userID := strings.TrimSpace(c.Param("id"))
	if userID == "" {
		writeError(c, http.StatusBadRequest, "user id is required")
		return
	}
	ctx, cancel := context.WithTimeout(c.Request.Context(), h.timeout)
	defer cancel()

	resp, err := h.client.IsAdmin(ctx, &authv1.IsAdminRequest{UserId: userID})
	if err != nil {
		h.handleAuthError(c, err)
		return
	}
	writeJSON(c, http.StatusOK, map[string]any{"is_admin": resp.GetIsAdmin()})
}

func maxAgeSeconds(d time.Duration) int {
	if d <= 0 {
		return 0
	}
	return int(d.Seconds())
}

func (h *AuthHandler) handleAuthError(c *gin.Context, err error) {
	sts, ok := status.FromError(err)
	if ok {
		switch sts.Code() {
		case codes.InvalidArgument, codes.Unauthenticated:
			writeError(c, http.StatusBadRequest, sts.Message())
			return
		case codes.NotFound:
			writeError(c, http.StatusNotFound, sts.Message())
			return
		case codes.Unavailable:
			writeError(c, http.StatusServiceUnavailable, "auth service unavailable")
			return
		}
	}
	writeError(c, http.StatusInternalServerError, "auth service error")
}

func convertUser(u *authv1.User) userResponse {
	if u == nil {
		return userResponse{}
	}
	res := userResponse{
		ID:    u.GetId(),
		Email: u.GetEmail(),
		Role:  roleToString(u.GetRole()),
	}
	if ts := u.GetCreatedAt(); ts != nil {
		res.CreatedAt = ts.AsTime().Format(time.RFC3339)
	}
	if ts := u.GetUpdatedAt(); ts != nil {
		res.UpdatedAt = ts.AsTime().Format(time.RFC3339)
	}
	return res
}

func roleToString(role authv1.UserRole) string {
	switch role {
	case authv1.UserRole_USER_ROLE_ADMIN:
		return "admin"
	case authv1.UserRole_USER_ROLE_USER:
		return "user"
	default:
		return "unspecified"
	}
}
