package main

import (
	"log"

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

	server := web.NewServer(cfg)

	if err := server.Start(cfg.Addr); err != nil {
		log.Fatal(err)
	}
}
