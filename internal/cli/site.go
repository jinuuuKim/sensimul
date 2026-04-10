package cli

import (
	"fmt"

	"github.com/sensimul/sensimul/internal/config"
	"github.com/sensimul/sensimul/internal/domain"
	"github.com/sensimul/sensimul/internal/persistence/sqlite"
	"github.com/spf13/cobra"
)

func newSiteCommand() *cobra.Command {
	cmd := &cobra.Command{Use: "site", Short: "Manage sites"}

	add := &cobra.Command{
		Use:   "add",
		Short: "Add a site",
		RunE: func(cmd *cobra.Command, args []string) error {
			id, _ := cmd.Flags().GetString("id")
			name, _ := cmd.Flags().GetString("name")
			siteTypeRaw, _ := cmd.Flags().GetString("type")
			lat, _ := cmd.Flags().GetFloat64("lat")
			lon, _ := cmd.Flags().GetFloat64("lon")

			site := domain.NewSite(id, name, domain.SiteType(siteTypeRaw), lat, lon)
			if err := site.Validate(); err != nil {
				return err
			}

			repo, err := openRepository()
			if err != nil {
				return err
			}
			defer repo.Close()

			if err := repo.CreateSite(site); err != nil {
				return err
			}

			fmt.Fprintf(cmd.OutOrStdout(), "site created: %s\n", site.ID)
			return nil
		},
	}
	add.Flags().String("id", "", "site id")
	add.Flags().String("name", "", "site name")
	add.Flags().String("type", "indoor", "site type: indoor|outdoor")
	add.Flags().Float64("lat", 37.5665, "latitude")
	add.Flags().Float64("lon", 126.9780, "longitude")
	_ = add.MarkFlagRequired("id")
	_ = add.MarkFlagRequired("name")

	list := &cobra.Command{
		Use:   "list",
		Short: "List sites",
		RunE: func(cmd *cobra.Command, args []string) error {
			repo, err := openRepository()
			if err != nil {
				return err
			}
			defer repo.Close()

			sites, err := repo.ListSites()
			if err != nil {
				return err
			}

			for _, site := range sites {
				fmt.Fprintf(cmd.OutOrStdout(), "%s\t%s\t%s\n", site.ID, site.Name, site.Type)
			}
			return nil
		},
	}

	cmd.AddCommand(add, list)
	return cmd
}

func openRepository() (*sqlite.Repository, error) {
	cfg := config.MustLoad(configPath)
	repo, err := sqlite.New(cfg.SQLite.Path)
	if err != nil {
		return nil, domain.NewRuntimeError("failed to open sqlite repository", err)
	}
	return repo, nil
}
