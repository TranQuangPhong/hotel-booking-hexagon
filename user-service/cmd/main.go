package main

import (
	"booking/user-service/config"
	"booking/user-service/internal/adapter/handler"
	"booking/user-service/internal/adapter/postgres"
	"booking/user-service/internal/user"
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	logger "github.com/TranQuangPhong/hotel-booking-logger"
	"github.com/jackc/pgx/v5/pgxpool"
)

func main() {
	// Init logger
	log := logger.NewLogger()
	slog.SetDefault(log)

	// Create root context listening to OS shutdown signals
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	// Load env config
	cfg, err := config.Load()

	// Init postgresql
	connString := fmt.Sprintf( // Eg: "postgres://userservice:userservice@localhost:5440/users"
		"postgres://%s:%s@%s:%d/%s",
		cfg.DB.User,
		cfg.DB.Password,
		cfg.DB.Host,
		cfg.DB.Port,
		cfg.DB.Name,
	)
	pool, err := pgxpool.New(ctx, connString)
	if err != nil {
		slog.Error("failed to init postgresql", "error", err.Error())
		os.Exit(1)
	}
	if err := pool.Ping(ctx); err != nil {
		slog.Error("failed to ping postgresql server", "error", err.Error())
		pool.Close()
		os.Exit(1)
	}
	defer pool.Close()

	// Init repository, service, handler, router
	userRepository := postgres.NewUserRepository(ctx, pool)
	userService := user.NewUserService(userRepository)
	userHandler := handler.NewUserHandler(userService)
	router := userHandler.UserRouter()

	// Start http server
	httpServerPort := fmt.Sprintf(":%d", cfg.Server.Port) //Eg ":8181"
	server := &http.Server{
		Addr:    httpServerPort,
		Handler: router,
	}
	go func() {
		slog.Info("Server is starting...", "addr", httpServerPort)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Error("failed to start http server", "error", err.Error())
			os.Exit(1)
		}
	}()

	// Block main thread waiting for shutdown signal
	<-ctx.Done()
	slog.Info("Shutdown signal received, exiting")

	// Graceful shutdown
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := server.Shutdown(shutdownCtx); err != nil {
		slog.Error("HTTP server forced to shutdown", "error", err.Error())
	} else {
		slog.Info("HTTP server exiting gracefully")
	}

	slog.Info("Service exiting gracefully")
}
