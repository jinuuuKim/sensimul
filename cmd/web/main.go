package main

import (
	"flag"
	"os"

	"github.com/sensimul/sensimul/internal/logging"
	"github.com/sensimul/sensimul/internal/web"
)

func main() {
	configPath := flag.String("config", "config/sensimul.yaml", "config file path")
	checkConfigOnly := flag.Bool("check-config-only", false, "validate config and exit")
	flag.Parse()

	logger := logging.Init()
	defer logger.Close()

	server, err := web.NewServer(*configPath)
	if err != nil {
		logger.Error().Err(err).Msg("failed to initialize web server")
		os.Exit(2)
	}
	defer server.Close()

	if *checkConfigOnly {
		logger.Info().Msg("web config check completed")
		return
	}

	if err := server.Run(); err != nil {
		logger.Error().Err(err).Msg("web server exited with error")
		os.Exit(1)
	}
}
