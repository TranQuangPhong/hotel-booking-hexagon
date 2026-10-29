package main

import (
	roomv1 "booking/contracts/room/v1"
	"booking/room-service/config"
	roomgrpc "booking/room-service/internal/adapter/grpc"
	"booking/room-service/internal/adapter/handler"
	"booking/room-service/internal/adapter/postgres"
	"booking/room-service/internal/room"
	"context"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"runtime/debug"
	"syscall"
	"time"

	logger "github.com/TranQuangPhong/hotel-booking-logger"
	grpclogger "github.com/TranQuangPhong/hotel-booking-logger/grpclogger"
	"github.com/gin-gonic/gin"
	"github.com/grpc-ecosystem/go-grpc-middleware/v2/interceptors/recovery"
	"github.com/jackc/pgx/v5/pgxpool"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/reflection"
	"google.golang.org/grpc/status"
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
		slog.Error("failed to load config", "error", err)
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
		slog.Error("failed to init postgresql", "error", err)
		os.Exit(1)
	}
	if err := pool.Ping(ctx); err != nil {
		slog.Error("failed to ping postgresql server", "error", err)
		pool.Close()
		os.Exit(1)
	}
	defer pool.Close()

	// Init repository, service, handler, router
	roomRepository := postgres.NewRoomRepository(pool)
	// inventoryRepository := postgres.NewInventoryRepository(ctx, pool)
	roomService := room.NewService(roomRepository)
	roomHander := handler.New(roomService)
	// process-wide: a JSON field the request type doesn't have (e.g. currency on PATCH .../inventories) is a 400,
	// not silently dropped
	gin.EnableJsonDecoderDisallowUnknownFields()
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
			slog.Error("failed to start http server", "error", err)
			os.Exit(1)
		}
	}()

	// Start grpc server
	grpcPort := fmt.Sprintf(":%d", cfg.GRPC.Port)
	lis, err := net.Listen("tcp", grpcPort)
	if err != nil {
		slog.Error("grpc: failed to listen", "addr", grpcPort, "error", err)
		os.Exit(1)
	}
	opts := []grpc.ServerOption{
		grpc.ChainUnaryInterceptor(
			grpclogger.UnaryServerInterceptor(),
			// Without a handler, v2 sends a panic to the client as UNKNOWN + stack trace and logs nothing
			recovery.UnaryServerInterceptor(recovery.WithRecoveryHandlerContext(
				func(ctx context.Context, p any) error {
					slog.ErrorContext(ctx, "grpc handler panic", "panic", p, "stack", string(debug.Stack()))
					return status.Error(codes.Internal, "internal server error")
				},
			)),
		),
	}
	grpcServer := grpc.NewServer(opts...)
	roomv1.RegisterRoomServiceServer(grpcServer, roomgrpc.New(roomService))
	reflection.Register(grpcServer) // dev: grpcurl list/describe without -proto; gate it before deploying
	go func() {
		slog.Info("grpc server is starting...", "addr", grpcPort)
		if err := grpcServer.Serve(lis); err != nil {
			slog.Error("failed to start grpc server", "error", err)
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
		slog.Error("HTTP server forced to shutdown", "error", err)
	} else {
		slog.Info("HTTP server exiting gracefully")
	}
	// GracefulStop has no time limit: Stop force-closes the running RPCs once shutdownCtx expires
	cancelForceStop := context.AfterFunc(shutdownCtx, grpcServer.Stop)
	grpcServer.GracefulStop()
	if cancelForceStop() {
		slog.Info("gRPC server exiting gracefully")
	} else {
		slog.Error("gRPC server forced to stop")
	}
	slog.Info("Service exiting gracefully")
}
