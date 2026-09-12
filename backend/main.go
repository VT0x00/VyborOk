package main

import (
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/VT0x00/vyborok/http/handler"
	pb "github.com/VT0x00/vyborok/http/proto"
	"github.com/VT0x00/vyborok/internal/auth"
	"github.com/VT0x00/vyborok/internal/config"
	"github.com/VT0x00/vyborok/internal/repository"
	"github.com/VT0x00/vyborok/pkg/database"

	jsoncodec "go.unistack.org/micro-codec-json/v3"
	httpsrv "go.unistack.org/micro-server-http/v3"
	"go.unistack.org/micro/v3/server"

	"github.com/joho/godotenv"
)

func main() {
	_ = godotenv.Load()

	logger := setupLogger()
	slog.SetDefault(logger)

	cfg := config.Load()
	logger.Info("config loaded", "env", cfg.App.Env, "port", cfg.App.Port)

	// --- БД ---
	db, err := database.Connect(database.Config{
		Host:            cfg.DB.Host,
		Port:            cfg.DB.Port,
		User:            cfg.DB.User,
		Password:        cfg.DB.Password,
		DBName:          cfg.DB.Name,
		SSLMode:         cfg.DB.SSLMode,
		MaxOpenConns:    cfg.DB.MaxOpenConns,
		MaxIdleConns:    cfg.DB.MaxIdleConns,
		ConnMaxLifetime: cfg.DB.ConnMaxLifetime,
	})
	if err != nil {
		logger.Error("cannot connect to postgres", "err", err)
		os.Exit(1)
	}

	defer db.Close()
	logger.Info("connected to postgres", "host", cfg.DB.Host, "db", cfg.DB.Name)

	userRepo := repository.NewUserRepository(db)
	jwtMgr := auth.NewJWTManager(cfg.JWT.Secret, cfg.JWT.Issuer, cfg.JWT.AccessTTL, cfg.JWT.RefreshTTL)
	authSvc := auth.NewService(userRepo, jwtMgr)
	h := handler.New(authSvc, logger)

	// --- micro HTTP-сервер ---
	srv := httpsrv.NewServer(
		server.Name("vyborok"),
		server.Address(":"+cfg.App.Port),
		server.Codec("application/json", jsoncodec.NewCodec()),
	)

	if err := pb.RegisterVyborokServer(srv, h); err != nil {
		logger.Error("cannot register handler", "err", err)
		os.Exit(1)
	}

	if err := srv.Start(); err != nil {
		logger.Error("cannot start server", "err", err)
		os.Exit(1)
	}
	logger.Info("micro http server started", "addr", srv.Options().Address)

	// --- graceful shutdown ---
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	<-stop

	logger.Info("shutting down by signal")
	if err := srv.Stop(); err != nil {
		logger.Error("stop error", "err", err)
	}

	logger.Info("bye")
}

func setupLogger() *slog.Logger {
	env := os.Getenv("APP_ENV")
	if env == "prod" {
		return slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
	}
	return slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelDebug}))
}
