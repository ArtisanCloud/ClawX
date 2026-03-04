package main

import (
	"log"
	"os"

	"synapsex/internal/application/service"
	"synapsex/internal/infrastructure/backend"
	"synapsex/internal/infrastructure/config"
	"synapsex/internal/infrastructure/persistence"
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

	// The channel adapters will be wired in later phases. Phase 3 only requires
	// the direct execution chain to be constructed and ready for use.
	_ = router
	_, _ = os.Stdout.WriteString("synapsex phase 1 chain initialized\n")
}
