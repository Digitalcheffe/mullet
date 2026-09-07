package shapes

// WeatherCurrent represents current weather conditions.
// No related metadata table -- standalone data.
type WeatherCurrent struct {
	ID               string   `db:"id"`
	PluginInstanceID int      `db:"plugin_instance_id"`
	Temp             float64  `db:"temp"`
	FeelsLike        *float64 `db:"feels_like"`
	Condition        string   `db:"condition"`
	Icon             string   `db:"icon"`
	Humidity         *int     `db:"humidity"`
	High             *float64 `db:"high"`
	Low              *float64 `db:"low"`
	Sunrise          *string  `db:"sunrise"`
	Sunset           *string  `db:"sunset"`
}

// WeatherForecast represents a single day's forecast.
// No related metadata table -- standalone data.
type WeatherForecast struct {
	ID               string  `db:"id"`
	PluginInstanceID int     `db:"plugin_instance_id"`
	Date             string  `db:"date"`
	High             float64 `db:"high"`
	Low              float64 `db:"low"`
	Condition        string  `db:"condition"`
	Icon             string  `db:"icon"`
	PrecipChance     *int    `db:"precip_chance"`
}
