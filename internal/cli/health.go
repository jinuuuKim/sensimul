package cli

import (
	"context"
	"fmt"
	"time"

	"github.com/sensimul/sensimul/internal/app"
	"github.com/spf13/cobra"
)

func newHealthCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "health",
		Short: "Check runtime health dependencies",
		RunE: func(cmd *cobra.Command, args []string) error {
			a, err := app.New(configPath)
			if err != nil {
				return err
			}
			defer a.Close()

			ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
			defer cancel()

			if err := a.Health(ctx); err != nil {
				return err
			}

			fmt.Fprintln(cmd.OutOrStdout(), "ok")
			return nil
		},
	}
}
