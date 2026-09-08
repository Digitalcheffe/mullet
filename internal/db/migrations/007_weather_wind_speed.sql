-- Wind speed for the weather-current UI plugin (issue #22). Units match
-- whatever the writing data plugin was configured with (km/h for
-- metric, mph for imperial) -- same convention as temp/high/low, which
-- carry no separate unit column either.
ALTER TABLE shape_weather_current ADD COLUMN wind_speed REAL;
