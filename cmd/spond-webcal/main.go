package main

import (
	"log"

	"code.p-fruck.eu/spond-webcal/internal/caldav"
	"code.p-fruck.eu/spond-webcal/internal/config"
	"code.p-fruck.eu/spond-webcal/internal/db"
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

	server, err := web.NewServer(cfg, caldavHandler)
	if err != nil {
		log.Fatal(err)
	}
	log.Printf("starting spond-webcal server on %s (spond api: %s)", cfg.Addr, cfg.SpondBaseURL)

	if err := server.Start(cfg.Addr); err != nil {
		log.Fatal(err)
	}
}
