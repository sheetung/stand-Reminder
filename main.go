package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"stand-reminder/internal/app"
	"stand-reminder/internal/tray"
	webui "stand-reminder/internal/web"
)

const (
	webAddress = "127.0.0.1:47831"
	// CurrentVersion is the application version
	// This is compared with the latest GitHub release to check for updates
	CurrentVersion = "v0.6.2"
)

func main() {
	log.SetFlags(log.LstdFlags | log.Lmicroseconds)

	configDir, err := os.UserConfigDir()
	if err != nil {
		log.Fatalf("failed to get config directory: %v", err)
	}
	appDir := filepath.Join(configDir, "Stand Reminder")
	dbPath := filepath.Join(appDir, "stand-reminder.db")

	application, err := app.New(dbPath, CurrentVersion)
	if err != nil {
		log.Fatalf("failed to start app: %v", err)
	}

	go application.Run()

	server := &http.Server{Addr: webAddress, Handler: webui.NewServer(application).Handler()}
	go func() {
		log.Printf("web console ready: http://%s", webAddress)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
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
		log.Printf("tray exited with error: %v", err)
	}

	// Graceful shutdown
	log.Println("shutting down...")
	application.Shutdown()
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	if err := server.Shutdown(shutdownCtx); err != nil {
		log.Printf("web server shutdown: %v", err)
	}
	log.Println("shutdown complete")
}

