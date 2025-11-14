package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"github.com/immxrtalbeast/api-gateway/internal/clients/scripts"
	"github.com/immxrtalbeast/api-gateway/internal/clients/videos"
	"github.com/immxrtalbeast/api-gateway/internal/config"
	"github.com/immxrtalbeast/api-gateway/internal/events"
	"github.com/immxrtalbeast/api-gateway/internal/http/handlers"
	"github.com/immxrtalbeast/api-gateway/internal/http/middleware"
	"github.com/immxrtalbeast/api-gateway/lib/logger/slogpretty"
	authv1 "github.com/immxrtalbeast/protos/gen/go/auth/v1"
	"github.com/joho/godotenv"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

func main() {
	dotenvErr := godotenv.Load(".env")
	cfg := config.MustLoad()
	log := setupLogger(cfg.Env)
	log.Info("starting api gateway")
	if dotenvErr != nil {
		log.Warn(".env not loaded", slog.String("err", dotenvErr.Error()))
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	authConn, err := grpc.DialContext(ctx, cfg.AuthGRPC.Address, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		log.Error("failed to connect auth grpc", slog.String("err", err.Error()))
		os.Exit(1)
	}
	defer authConn.Close()

	authClient := authv1.NewAuthServiceClient(authConn)

	scriptClient, err := scripts.New(cfg.ScriptService.BaseURL, cfg.ScriptService.Timeout)
	if err != nil {
		log.Error("failed to init script client", slog.String("err", err.Error()))
		os.Exit(1)
	}

	videoClient, err := videos.New(cfg.VideoService.BaseURL, cfg.VideoService.Timeout)
	if err != nil {
		log.Error("failed to init video client", slog.String("err", err.Error()))
		os.Exit(1)
	}

	if cfg.AppSecret == "" {
		log.Error("APP_SECRET is not configured (set app_secret in config or APP_SECRET env)")
		os.Exit(1)
	}
	if cfg.TokenTTL <= 0 {
		log.Error("token_ttl must be greater than zero")
		os.Exit(1)
	}

	authHandler := handlers.NewAuthHandler(log, authClient, cfg.AuthGRPC.Timeout, cfg.TokenTTL)
	scriptHandler := handlers.NewScriptHandler(log, scriptClient, cfg.ScriptService.Timeout)
	var (
		streamHub     *events.Hub
		kafkaConsumer *events.KafkaConsumer
	)
	if cfg.Kafka.Enabled {
		if len(cfg.Kafka.Brokers) == 0 {
			log.Error("kafka brokers are not configured")
			os.Exit(1)
		}
		streamHub = events.NewHub()
		consumer, err := events.NewKafkaConsumer(
			events.KafkaConsumerConfig{
				Brokers: cfg.Kafka.Brokers,
				Topic:   cfg.Kafka.UpdatesTopic,
				GroupID: cfg.Kafka.GroupID,
				MaxWait: cfg.Kafka.MaxWait,
			},
			streamHub,
			log,
		)
		if err != nil {
			log.Error("failed to init kafka consumer", slog.String("err", err.Error()))
			os.Exit(1)
		}
		kafkaConsumer = consumer
		kafkaConsumer.Run(ctx)
		defer kafkaConsumer.Close()
	}

	videoHandler := handlers.NewVideoHandler(log, videoClient, cfg.VideoService.Timeout, streamHub)
	authMiddleware := middleware.AuthMiddleware(cfg.AppSecret)

	router := setupRouter(cfg.Env, authHandler, scriptHandler, videoHandler, authMiddleware)

	srv := &http.Server{
		Addr:         fmt.Sprintf("%s:%d", cfg.HTTP.Host, cfg.HTTP.Port),
		Handler:      router,
		ReadTimeout:  cfg.HTTP.ReadTimeout,
		WriteTimeout: cfg.HTTP.WriteTimeout,
		IdleTimeout:  cfg.HTTP.IdleTimeout,
	}

	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := srv.Shutdown(shutdownCtx); err != nil {
			log.Error("server shutdown error", slog.String("err", err.Error()))
		}
	}()

	log.Info("http server listening", slog.String("addr", srv.Addr))
	if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		log.Error("server stopped", slog.String("err", err.Error()))
	}
}

func requestLogger(log *slog.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		c.Next()
		duration := time.Since(start)
		status := c.Writer.Status()
		msg := "request completed"
		if status >= http.StatusBadRequest {
			log.Warn(msg,
				slog.String("method", c.Request.Method),
				slog.String("path", c.Request.URL.Path),
				slog.Int("status", status),
				slog.Duration("duration", duration),
				slog.String("client", c.ClientIP()),
				slog.String("error", c.Errors.String()),
			)
			return
		}
		log.Info(msg,
			slog.String("method", c.Request.Method),
			slog.String("path", c.Request.URL.Path),
			slog.Int("status", status),
			slog.Duration("duration", duration),
			slog.String("client", c.ClientIP()),
		)
	}
}

const (
	envLocal = "local"
	envDev   = "dev"
	envProd  = "prod"
)

func setupLogger(env string) *slog.Logger {
	var log *slog.Logger

	switch env {
	case envLocal:
		log = setupPrettySlog()
	case envDev:
		log = slog.New(
			slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelDebug}),
		)
	case envProd:
		log = slog.New(
			slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}),
		)
	default:
		log = slog.New(
			slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}),
		)
	}

	return log
}

func setupPrettySlog() *slog.Logger {
	opts := slogpretty.PrettyHandlerOptions{
		SlogOpts: &slog.HandlerOptions{
			Level: slog.LevelDebug,
		},
	}

	handler := opts.NewPrettyHandler(os.Stdout)

	return slog.New(handler)
}

func setupRouter(
	env string,
	authHandler *handlers.AuthHandler,
	scriptHandler *handlers.ScriptHandler,
	videoHandler *handlers.VideoHandler,
	authMiddleware gin.HandlerFunc,
) *gin.Engine {
	mode := gin.ReleaseMode
	if env == envLocal {
		mode = gin.DebugMode
	}
	gin.SetMode(mode)

	router := gin.New()
	config := cors.DefaultConfig()
	config.AllowOrigins = []string{
		"http://localhost:3000",
	}
	config.AllowCredentials = true
	config.AllowHeaders = []string{
		"Authorization",
		"Content-Type",
		"Origin",
		"Accept",
	}
	config.AllowMethods = []string{"GET", "POST", "PUT", "PATCH", "DELETE", "HEAD", "OPTIONS"}
	config.ExposeHeaders = []string{"Set-Cookie"}
	router.Use(cors.New(config))
	if env == envLocal {
		router.Use(gin.Logger())
	}
	router.Use(gin.Recovery())
	router.Use(requestLogger(setupLogger(env)))

	router.GET("/healthz", func(c *gin.Context) {
		c.String(http.StatusOK, "ok")
	})

	auth := router.Group("/api/auth")
	{
		auth.POST("/register", authHandler.Register)
		auth.POST("/login", authHandler.Login)
		auth.POST("/refresh", authHandler.RefreshToken)
		auth.POST("/logout", authHandler.Logout)
		auth.GET("/users/:id", authMiddleware, authHandler.GetUser)
		auth.GET("/users/:id/is_admin", authMiddleware, authHandler.IsAdmin)
	}

	scripts := router.Group("/api/scripts")
	scripts.Use(authMiddleware)
	{
		scripts.POST("", scriptHandler.CreateScript)
		scripts.GET("", scriptHandler.ListScripts)
	}

	videos := router.Group("/api/videos")
	videos.Use(authMiddleware)
	{
		videos.POST("", videoHandler.CreateVideo)
		videos.GET("", videoHandler.ListVideos)
		videos.GET("/:id", videoHandler.GetVideo)
		videos.POST("/:id/draft:approve", videoHandler.ApproveDraft)
		videos.POST("/:id/subtitles:approve", videoHandler.ApproveSubtitles)
		videos.POST("/media", videoHandler.UploadMedia)
		videos.GET("/media", videoHandler.ListMedia)
		videos.GET("/media/shared", videoHandler.ListSharedMedia)
		videos.POST("/media/videos", videoHandler.UploadVideoMedia)
		videos.POST("/media/videos:upload", videoHandler.UploadVideoBinary)
		videos.GET("/media/videos", videoHandler.ListVideoMedia)
		videos.GET("/media/shared/videos", videoHandler.ListSharedVideoMedia)
		videos.GET("/voices", videoHandler.ListVoices)
		videos.GET("/music", videoHandler.ListMusic)
		videos.GET("/:id/stream", videoHandler.StreamVideo)
	}

	ideas := router.Group("/api/ideas")
	ideas.Use(authMiddleware)
	{
		ideas.POST("/expand", videoHandler.ExpandIdea)
	}

	return router
}
