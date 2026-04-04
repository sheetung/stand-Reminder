package main

import (
	"errors"
	"log"
	"os"
	"path/filepath"

	"stand-reminder/internal/app"
	"stand-reminder/internal/tray"
	webui "stand-reminder/internal/web"
)

const webAddress = "127.0.0.1:47831"

func main() {
	log.SetFlags(log.LstdFlags | log.Lmicroseconds)

	configPath := filepath.Join(".", "config.json")
	application, err := app.New(configPath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			log.Fatalf("config file not found: %s", configPath)
		}

		log.Fatalf("failed to start app: %v", err)
	}

	go application.Run()

	server := webui.NewServer(application)
	go func() {
		log.Printf("web console ready: http://%s", webAddress)
		if err := server.ListenAndServe(webAddress); err != nil {
			log.Fatalf("web server failed: %v", err)
		}
	}()

	if err := tray.Run("http://" + webAddress); err != nil {
		log.Fatalf("tray failed: %v", err)
	}
}
