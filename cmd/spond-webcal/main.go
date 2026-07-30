package main

import (
	"context"
	"log"

	"code.p-fruck.eu/spond-webcal/internal/caldav"
	"code.p-fruck.eu/spond-webcal/internal/config"
	"code.p-fruck.eu/spond-webcal/internal/db"
	"code.p-fruck.eu/spond-webcal/internal/syncworker"
	"code.p-fruck.eu/spond-webcal/internal/web"
)

func main() {
	cfg := config.Load()

	database, err := db.OpenAndMigrate(cfg.DBURL)
	if err != nil {
		log.Fatal(err)
	}

	sqlDB, err := database.DB()
	if err != nil {
		log.Fatal(err)
	}
	defer func() {
		_ = sqlDB.Close()
	}()

	caldavStore := db.NewCalDAVStore(database)
	caldavHandler, err := caldav.NewHandler(caldavStore, "default")
	if err != nil {
		log.Fatal(err)
	}

	userTokenStore := db.NewUserTokenStore(database)
	cacheStore := db.NewSpondCacheStore(database)

	worker, err := syncworker.NewEventsWorker(
		userTokenStore,
		cacheStore,
		cfg.SpondBaseURL,
		cfg.SyncInterval,
		cfg.SyncTimeout,
		log.Default(),
	)
	if err != nil {
		log.Fatal(err)
	}

	workerCtx, workerCancel := context.WithCancel(context.Background())
	defer workerCancel()
	worker.Start(workerCtx)
	log.Printf("events sync worker enabled (interval=%s timeout=%s)", cfg.SyncInterval, cfg.SyncTimeout)

	server, err := web.NewServer(cfg, caldavHandler, userTokenStore)
	if err != nil {
		log.Fatal(err)
	}
	log.Printf("starting spond-webcal server on http://%s", cfg.Addr)
	log.Printf("spond api: %s", cfg.SpondBaseURL)

	if err := server.Start(cfg.Addr); err != nil {
		log.Fatal(err)
	}
}
