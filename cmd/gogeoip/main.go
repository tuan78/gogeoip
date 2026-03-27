package main

import (
	"context"
	"log"
	"os/signal"
	"syscall"

	"github.com/tuan78/gogeoip/internal/cache"
	"github.com/tuan78/gogeoip/internal/config"
	"github.com/tuan78/gogeoip/internal/geo"
	"github.com/tuan78/gogeoip/internal/server"
	"github.com/tuan78/gogeoip/internal/utils"
)

func main() {
	cfg := config.Load()

	// Set up optional Redis cache; fall back to no-op if REDIS_ADDR is empty.
	var c cache.Cache = cache.NoopCache{}
	if cfg.RedisAddr != "" {
		rc, err := cache.NewRedisCache(cfg.RedisAddr, cfg.RedisPassword)
		if err != nil {
			log.Fatalf("gogeoip: failed to connect to Redis at %s: %v", cfg.RedisAddr, err)
		}
		defer func() { _ = rc.Close() }()
		c = rc
		log.Printf("gogeoip: Redis cache enabled (%s)", cfg.RedisAddr)
	} else {
		log.Println("gogeoip: REDIS_ADDR not set, running without cache")
	}

	interval, err := utils.ResolveInterval(cfg.GeoDBRefreshInterval)
	if err != nil {
		log.Printf("gogeoip: invalid GEO_DB_REFRESH_INTERVAL, using default 24h: %v", err)
		interval = geo.DefaultRefreshInterval
	}

	db := &geo.DB{}
	db.Start(cfg.GeoDBPath, cfg.MaxmindAccountID, cfg.MaxmindLicenseKey, interval)

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	server.Serve(ctx, cfg, db, c)
}
