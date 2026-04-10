package cli

import (
	"context"
	"os/signal"
	"syscall"

	"github.com/sensimul/sensimul/internal/app"
	"github.com/spf13/cobra"
)

func newRunCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "run",
		Short: "Run simulation loop",
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
			defer stop()

			a, err := app.New(configPath)
			if err != nil {
				return err
			}
			defer a.Close()

			return a.Run(ctx)
		},
	}
}
