package main

import (
	"os"

	"github.com/sensimul/sensimul/internal/cli"
	"github.com/sensimul/sensimul/internal/logging"
)

func main() {
	logger := logging.Init()
	defer logger.Close()

	if err := cli.Execute(); err != nil {
		logger.Error().Err(err).Msg("application error")
		os.Exit(cli.ExitCodeForError(err))
	}
}
