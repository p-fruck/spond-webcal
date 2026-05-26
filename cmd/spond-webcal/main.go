package main

import (
	"log"

	"code.p-fruck.eu/spond-webcal/internal/config"
	"code.p-fruck.eu/spond-webcal/internal/web"
)

func main() {
	cfg := config.Load()
	server := web.NewServer(cfg)

	if err := server.Start(cfg.Addr); err != nil {
		log.Fatal(err)
	}
}