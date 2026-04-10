package cli

import (
	"fmt"

	"github.com/sensimul/sensimul/internal/domain"
	"github.com/spf13/cobra"
)

func newSensorCommand() *cobra.Command {
	cmd := &cobra.Command{Use: "sensor", Short: "Manage sensors"}

	add := &cobra.Command{
		Use:   "add",
		Short: "Add a sensor",
		RunE: func(cmd *cobra.Command, args []string) error {
			siteID, _ := cmd.Flags().GetString("site")
			sensorID, _ := cmd.Flags().GetString("id")
			sensorType, _ := cmd.Flags().GetString("type")

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

			sensor, err := domain.NewSensor(sensorID, siteID, sensorType)
			if err != nil {
				return err
			}

			if err := repo.CreateSensor(sensor); err != nil {
				return err
			}

			fmt.Fprintf(cmd.OutOrStdout(), "sensor created: %s\n", sensor.ID)
			return nil
		},
	}
	add.Flags().String("site", "", "site id")
	add.Flags().String("id", "", "sensor id")
	add.Flags().String("type", "temperature", "sensor type")
	_ = add.MarkFlagRequired("site")
	_ = add.MarkFlagRequired("id")

	list := &cobra.Command{
		Use:   "list",
		Short: "List sensors",
		RunE: func(cmd *cobra.Command, args []string) error {
			siteID, _ := cmd.Flags().GetString("site")

			repo, err := openRepository()
			if err != nil {
				return err
			}
			defer repo.Close()

			sensors, err := repo.ListSensors(siteID)
			if err != nil {
				return err
			}

			for _, sensor := range sensors {
				fmt.Fprintf(cmd.OutOrStdout(), "%s\t%s\t%s\n", sensor.ID, sensor.SiteID, sensor.SensorType)
			}
			return nil
		},
	}
	list.Flags().String("site", "", "site id")
	_ = list.MarkFlagRequired("site")

	cmd.AddCommand(add, list)
	return cmd
}
