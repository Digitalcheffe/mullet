package shapes

// MediaStatus represents the now-playing state of a media player. Written
// by media data plugins (e.g. Plex, Spotify), consumed by the
// media-now-playing UI plugin.
// Custom contract -- no related metadata table.
type MediaStatus struct {
	ID               string  `db:"id"`
	PluginInstanceID int     `db:"plugin_instance_id"`
	PlayerName       string  `db:"player_name"`
	IsPlaying        bool    `db:"is_playing"`
	Title            *string `db:"title"`
	Artist           *string `db:"artist"`
	AlbumArtURL      *string `db:"album_art_url"`
}
