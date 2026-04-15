package main

import (
	"log"
	"os"
	"path/filepath"

	"stand-reminder/internal/app"
	"stand-reminder/internal/tray"
	webui "stand-reminder/internal/web"
)

const (
	webAddress = "127.0.0.1:47831"
	// CurrentVersion is the application version
	// This is compared with the latest GitHub release to check for updates
	CurrentVersion = "v0.5.2"
)

func main() {
	log.SetFlags(log.LstdFlags | log.Lmicroseconds)

	exePath, err := os.Executable()
	if err != nil {
		log.Fatalf("failed to resolve executable path: %v", err)
	}
	exeDir := filepath.Dir(exePath)
	dbPath := filepath.Join(exeDir, "stand-reminder.db")

	application, err := app.New(dbPath, CurrentVersion)
	if err != nil {
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

	controlCenterURL := "http://" + webAddress
	application.SetControlCenterURL(controlCenterURL)
	go func() {
		if err := application.NotifyStarted(controlCenterURL); err != nil {
			log.Printf("failed to send startup notification: %v", err)
		}
	}()

	if err := tray.Run(controlCenterURL, application.Locale); err != nil {
		log.Fatalf("tray failed: %v", err)
	}
}
