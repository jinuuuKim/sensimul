package cli

import (
	"errors"

	"github.com/sensimul/sensimul/internal/domain"
	"github.com/spf13/cobra"
)

var configPath string

func Execute() error {
	return newRootCommand().Execute()
}

func ExitCodeForError(err error) int {
	if err == nil {
		return 0
	}

	var sensErr *domain.SensimulError
	if errors.As(err, &sensErr) {
		switch sensErr.Code {
		case domain.ErrCodeConfig, domain.ErrCodeValidation:
			return 2
		case domain.ErrCodeExternal:
			return 3
		default:
			return 1
		}
	}

	return 1
}

func newRootCommand() *cobra.Command {
	root := &cobra.Command{
		Use:   "sensimul",
		Short: "Environment sensor simulator",
	}

	root.PersistentFlags().StringVar(&configPath, "config", "config/sensimul.yaml", "config file path")
	root.AddCommand(newRunCommand())
	root.AddCommand(newHealthCommand())
	root.AddCommand(newSiteCommand())
	root.AddCommand(newSensorCommand())
	root.AddCommand(newControllerCommand())

	return root
}
