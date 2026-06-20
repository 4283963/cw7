package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"cw7/config"
	"cw7/internal/handlers"
	"cw7/internal/repositories"
	"cw7/internal/router"
	"cw7/internal/services"
	"cw7/pkg/snowflake"
)

var configPath = flag.String("config", "config/config.yaml", "config file path")

func main() {
	flag.Parse()

	cfg, err := config.LoadConfig(*configPath)
	if err != nil {
		log.Fatalf("load config failed: %v", err)
	}

	if err := config.InitMySQL(&cfg.MySQL); err != nil {
		log.Fatalf("init mysql failed: %v", err)
	}
	log.Println("mysql connected")

	if err := config.InitRedis(&cfg.Redis); err != nil {
		log.Printf("warn: init redis failed: %v", err)
	} else {
		log.Println("redis connected")
	}

	if err := config.AutoMigrate(); err != nil {
		log.Fatalf("auto migrate failed: %v", err)
	}
	log.Println("schema migration done")

	idGen, err := snowflake.NewSnowflake(1, 1)
	if err != nil {
		log.Fatalf("create snowflake failed: %v", err)
	}

	// 构造 repository
	driverRepo := repositories.NewDriverRepository(config.DB)
	withdrawRepo := repositories.NewWithdrawalRepository(config.DB, config.RDB)
	bankRepo := repositories.NewBankStatementRepository(config.DB)
	reconcileRepo := repositories.NewReconcileRepository(config.DB)

	// 构造 service
	withdrawSvc := services.NewWithdrawService(driverRepo, withdrawRepo, idGen, &cfg.Withdraw)
	reconcileSvc := services.NewReconcileService(withdrawRepo, bankRepo, reconcileRepo, idGen)

	// 构造 handler
	withdrawH := handlers.NewWithdrawHandler(withdrawSvc)
	reconcileH := handlers.NewReconcileHandler(reconcileSvc)

	// 组装路由
	engine := router.Setup(cfg.Server.Mode, withdrawH, reconcileH)

	addr := fmt.Sprintf(":%d", cfg.Server.Port)
	srv := &http.Server{
		Addr:         addr,
		Handler:      engine,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 60 * time.Second,
	}

	go func() {
		log.Printf("server starting on %s", addr)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("listen and serve error: %v", err)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	log.Println("shutdown server...")

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		log.Fatalf("server shutdown error: %v", err)
	}
	log.Println("server exited")
}
