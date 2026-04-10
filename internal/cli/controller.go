package cli

import (
	"fmt"

	"github.com/sensimul/sensimul/internal/domain"
	"github.com/spf13/cobra"
)

func newControllerCommand() *cobra.Command {
	cmd := &cobra.Command{Use: "controller", Short: "Manage controllers"}

	add := &cobra.Command{
		Use:   "add",
		Short: "Add a controller",
		RunE: func(cmd *cobra.Command, args []string) error {
			siteID, _ := cmd.Flags().GetString("site")
			controllerID, _ := cmd.Flags().GetString("id")
			ctrlTypeRaw, _ := cmd.Flags().GetString("type")

			repo, err := openRepository()
			if err != nil {
				return err
			}
			defer repo.Close()

			site, err := repo.GetSite(siteID)
			if err != nil {
				return err
			}
			if site == nil {
				return domain.NewValidationError("site does not exist")
			}

			controller, err := domain.NewController(controllerID, siteID, domain.ControllerType(ctrlTypeRaw), site.Type)
			if err != nil {
				return err
			}

			if err := repo.CreateController(controller); err != nil {
				return err
			}

			fmt.Fprintf(cmd.OutOrStdout(), "controller created: %s\n", controller.ID)
			return nil
		},
	}
	add.Flags().String("site", "", "site id")
	add.Flags().String("id", "", "controller id")
	add.Flags().String("type", "cooling", "controller type")
	_ = add.MarkFlagRequired("site")
	_ = add.MarkFlagRequired("id")

	list := &cobra.Command{
		Use:   "list",
		Short: "List controllers",
		RunE: func(cmd *cobra.Command, args []string) error {
			siteID, _ := cmd.Flags().GetString("site")

			repo, err := openRepository()
			if err != nil {
				return err
			}
			defer repo.Close()

			controllers, err := repo.ListControllers(siteID)
			if err != nil {
				return err
			}

			for _, ctrl := range controllers {
				fmt.Fprintf(cmd.OutOrStdout(), "%s\t%s\t%s\t%s\n", ctrl.ID, ctrl.SiteID, ctrl.Type, ctrl.Status)
			}
			return nil
		},
	}
	list.Flags().String("site", "", "site id")
	_ = list.MarkFlagRequired("site")

	cmd.AddCommand(add, list)
	return cmd
}
