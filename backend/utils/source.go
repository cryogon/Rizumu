package utils

import (
	"fmt"
)

func GetSourceURL(provider string, providerID string) (string, error) {
	switch provider {
	case "spotify":
		return "https://open.spotify.com/track/" + providerID, nil
	case "osu!":
		return "https://osu.ppy.sh/beatmapsets/" + providerID, nil
	case "youtube":
		return "https://www.youtube.com/watch?v=" + providerID, nil
	case "youtube-music":
		return "https://music.youtube.com/watch?v=" + providerID, nil
	default:
		return "", fmt.Errorf("source not supported")
	}
}
