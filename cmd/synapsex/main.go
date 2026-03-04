package main

import (
	"log"
	"os"

	"synapsex/internal/application/service"
	"synapsex/internal/infrastructure/backend"
	"synapsex/internal/infrastructure/config"
	"synapsex/internal/infrastructure/persistence"
	discordchat "synapsex/internal/interfaces/chat/discord"
	telegramchat "synapsex/internal/interfaces/chat/telegram"
)

func main() {
	cfg, err := config.LoadFromEnv()
	if err != nil {
		log.Fatalf("load config: %v", err)
	}

	repository := persistence.NewSessionMemoryRepository()
	sessionManager := service.NewSessionManager(repository, repository, nil)
	runner := backend.NewDirectRunner("primary", cfg.Timeout, nil)
	router := service.NewRouter(cfg, sessionManager, runner)
	formatter := service.NewOutputFormatter()
	streamer := service.NewOutputStreamer()
	delivery := service.NewOutputDelivery(formatter, streamer)
	discordAdapter := discordchat.NewAdapter()
	telegramAdapter := telegramchat.NewAdapter()

	// Phase 5 wires the output and channel adapter skeletons into the startup
	// graph. Real inbound event loops are added as concrete integrations later.
	_ = router
	_ = delivery
	_ = discordAdapter
	_ = telegramAdapter
	_, _ = os.Stdout.WriteString("synapsex phase 1 chain and channel adapters initialized\n")
}
