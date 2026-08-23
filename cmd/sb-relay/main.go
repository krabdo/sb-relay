package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/krabdo/sb-relay/internal/relay"
)

var version = "dev"

func main() {
	showVersion := flag.Bool("version", false, "print version and exit")
	flag.Parse()
	if *showVersion {
		fmt.Println(version)
		return
	}

	cfg, err := relay.ConfigFromEnv()
	if err != nil {
		log.Fatalf("configuration error: %v", err)
	}

	forum, err := relay.NewForumClient("https://sb.sb", cfg.UserID, cfg.Cookie, cfg.HTTPTimeout, version)
	if err != nil {
		log.Fatalf("forum client error: %v", err)
	}
	telegram, err := relay.NewTelegramClient("https://api.telegram.org", cfg.TelegramBotToken, cfg.TelegramChatID, cfg.HTTPTimeout)
	if err != nil {
		log.Fatalf("telegram client error: %v", err)
	}

	app := relay.NewApp(forum, telegram, relay.NewFileStateStore(cfg.StateFile), relay.AppOptions{
		PollInterval: cfg.PollInterval,
		MaxPages:     10,
		MaxSeen:      2048,
		Logger:       log.Default(),
	})

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	if err := app.Run(ctx); err != nil {
		log.Fatalf("sb-relay stopped: %v", err)
	}
}
