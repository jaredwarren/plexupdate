package config

// Configuration ...
type Configuration struct {
	Plex PlexConfiguration
}

// PlexConfiguration ...
type PlexConfiguration struct {
	Locations map[string]string
}
