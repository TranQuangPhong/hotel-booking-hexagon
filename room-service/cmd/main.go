package main

import (
	"booking/room-service/config"
	"booking/room-service/internal/adapter/handler"
	"booking/room-service/internal/adapter/postgres"
	"booking/room-service/internal/inventory"
	"booking/room-service/internal/room"
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
	// 1. Initialize structured logger FIRST
	log := logger.NewLogger()
	slog.SetDefault(log)

	// 2. Create a root context that listens for OS shutdown signals
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	// Load env config
	cfg, err := config.Load()
	if err != nil {
		slog.Error("failed to load config", "error", err.Error())
		os.Exit(1)
	}

	// Init postgresql
	connString := fmt.Sprintf( // Eg: "postgres://roomservice:roomservice@localhost:5441/rooms"
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
	roomRepository := postgres.NewRoomRepository(ctx, pool)
	roomService := room.NewRoomService(roomRepository)
	inventoryService := inventory.NewInventoryService()
	roomHander := handler.NewRoomHandler(roomService, inventoryService)
	router := roomHander.RoomRouter()

	// Start http server
	httpServerPort := fmt.Sprintf(":%d", cfg.Server.Port) //Eg ":8182"
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
