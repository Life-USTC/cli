package weather

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/Life-USTC/CLI/internal/api"
	"github.com/Life-USTC/CLI/internal/cmd/cmdutil"
	openapi "github.com/Life-USTC/CLI/internal/openapi"
	"github.com/Life-USTC/CLI/internal/output"
)

const defaultLocationKey = "ustc-main"

// NewCmdWeather queries the server's canonical campus weather locations.
func NewCmdWeather() *cobra.Command {
	locationKey := defaultLocationKey
	cmd := &cobra.Command{
		Use:   "weather",
		Short: "Show campus weather",
		Long:  "Query current conditions and forecasts for a USTC weather location.",
		Example: `  # Main campus (the default)
  life-ustc catalog weather

  # Gaoxin campus
  life-ustc catalog weather --location-key ustc-gaoxin

  # Get the complete response for scripts
  life-ustc catalog weather --json`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := validateLocationKey(locationKey); err != nil {
				return err
			}
			client, err := api.NewTypedClient(cmdutil.ServerFromCmd(cmd), false)
			if err != nil {
				return err
			}
			key := openapi.CatalogWeatherGetParamsLocationKey(locationKey)
			data, err := fetchWeather(client, &openapi.CatalogWeatherGetParams{LocationKey: &key})
			if err != nil {
				return err
			}
			return render(data)
		},
	}
	cmd.Flags().StringVar(&locationKey, "location-key", defaultLocationKey, "Weather location (ustc-main or ustc-gaoxin)")
	return cmd
}

func fetchWeather(client *api.TypedClient, params *openapi.CatalogWeatherGetParams) (any, error) {
	return api.ParseResponseRaw(client.CatalogWeatherGet(api.Ctx(), params))
}

func validateLocationKey(value string) error {
	if value != "ustc-main" && value != "ustc-gaoxin" {
		return fmt.Errorf("--location-key must be ustc-main or ustc-gaoxin")
	}
	return nil
}

func render(data any) error {
	if output.IsJSON() {
		return output.JSON(data)
	}
	m := cmdutil.AsMap(data)
	if err := output.OutputDetail(data, []output.FieldDef{
		{Key: "location.name", Label: "Location"},
		{Key: "location.key", Label: "Location key"},
		{Key: "fetchedAt", Label: "Fetched"},
		{Key: "current.condition.text", Label: "Condition"},
		{Key: "current.temperature", Label: "Temperature"},
		{Key: "current.feelsLike", Label: "Feels like", SkipEmpty: true},
		{Key: "current.humidity", Label: "Humidity", SkipEmpty: true},
		{Key: "current.pressure", Label: "Pressure", SkipEmpty: true},
		{Key: "current.windDirection", Label: "Wind direction", SkipEmpty: true},
		{Key: "current.windSpeed", Label: "Wind speed", SkipEmpty: true},
	}, "Weather"); err != nil {
		return err
	}
	if daily, ok := m["daily"].([]any); ok && len(daily) > 0 {
		fmt.Println()
		output.Bold("  Forecast")
		output.Table(cmdutil.RowsFromAny(daily), []output.Column{
			{Header: "Date", Key: "date"},
			{Header: "Condition", Key: "condition.text"},
			{Header: "Low", Key: "temperatureLow"},
			{Header: "High", Key: "temperatureHigh"},
		})
	}
	return nil
}
