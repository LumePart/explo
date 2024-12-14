package main

import (
	"bytes"
	"encoding/json"
	"explo/debug"
	"fmt"
	"log"
	"net/url"
)

type LoginPayload struct {
	User LoginUser `json:"user"`
}

type LoginUser struct {
	Login    string `json:"login"`
	Password string `json:"password"`
}

type LoginResponse struct {
	User struct {
		AuthToken string `json:"authToken"`
	} `json:"user"`
}

type Libraries struct {
	MediaContainer struct {
		Size      int    `json:"size"`
		AllowSync bool   `json:"allowSync"`
		Title1    string `json:"title1"`
		Library []struct {
			Title 			 string `json:"title"`
			Key              string `json:"key"`
			Location         []struct {
				ID   int    `json:"id"`
				Path string `json:"path"`
			} `json:"Location"`
		} `json:"Directory"`
	} `json:"MediaContainer"`
}

type PlexSearch struct {
	MediaContainer struct {
		Size         int `json:"size"`
		SearchResult []struct {
			Score    float64 `json:"score"`
			Metadata struct {
				LibrarySectionTitle  string `json:"librarySectionTitle"`
				Key                  string `json:"key"`
				Type                 string `json:"type"`
				Title                string `json:"title"` // Track
				GrandparentTitle     string `json:"grandparentTitle"` // Artist
				ParentTitle          string `json:"parentTitle"` // Album
				OriginalTitle        string `json:"originalTitle"`
				Summary              string `json:"summary"`
				Duration             int    `json:"duration"`
				AddedAt              int    `json:"addedAt"`
				UpdatedAt            int    `json:"updatedAt"`
				Media                []struct {
					ID            int    `json:"id"`
					Duration      int    `json:"duration"`
					AudioChannels int    `json:"audioChannels"`
					AudioCodec    string `json:"audioCodec"`
					Container     string `json:"container"`
				} `json:"Media"`
			} `json:"Metadata"`
		} `json:"SearchResult"`
	} `json:"MediaContainer"`
}

	
type PlexServer struct {
	MediaContainer struct {
		Size              int    `json:"size"`
		APIVersion        string `json:"apiVersion"`
		Claimed           bool   `json:"claimed"`
		MachineIdentifier string `json:"machineIdentifier"`
		Version           string `json:"version"`
	} `json:"MediaContainer"`
}

type PlexPlaylist struct {
	MediaContainer struct {
		Size     int `json:"size"`
		Metadata []struct {
			RatingKey    string `json:"ratingKey"`
			Key          string `json:"key"`
			GUID         string `json:"guid"`
			Type         string `json:"type"`
			Title        string `json:"title"`
			Summary      string `json:"summary"`
			Smart        bool   `json:"smart"`
			PlaylistType string `json:"playlistType"`
			AddedAt      int    `json:"addedAt"`
			UpdatedAt    int    `json:"updatedAt"`
			Duration     int    `json:"duration,omitempty"`
		} `json:"Metadata"`
	} `json:"MediaContainer"`
}

func (cfg *Credentials) plexHeader() {
	cfg.Headers = make(map[string]string)

	cfg.Headers["X-Plex-Client-Identifier"] = "explo"
}


func (cfg *Credentials) getPlexAuth() { // Get user token from plex
	payload := LoginPayload{
		User: LoginUser{
			Login:    cfg.User,
			Password: cfg.Password,
		},
	}

	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		log.Fatalf("failed to marshal payload: %s", err.Error())
	}


	body, err := makeRequest("POST", "https://plex.tv/users/sign_in.json", bytes.NewBuffer(payloadBytes), cfg.Headers)
	
	if err != nil {
		log.Fatalf("failed to make request to plex: %s", err.Error())
	}

	var auth LoginResponse
	err = parseResp(body, &auth)
	if err != nil {
		log.Fatalf("getPlexAuth(): %s", err.Error())
	}

	cfg.APIKey = auth.User.AuthToken
}

func getPlexLibraries(cfg Config) (Libraries, error) {
	params := fmt.Sprintf("/library/sections/?X-Plex-Token=%s", cfg.Creds.APIKey)

	body, err := makeRequest("GET", cfg.URL+params, nil, cfg.Creds.Headers)
	if err != nil {
		return Libraries{}, fmt.Errorf("failed to make request to plex: %s", err.Error())
	}

	var libraries Libraries
	err = parseResp(body, &libraries)
	if err != nil {
		log.Fatalf("getPlexLibraries(): %s", err.Error())
	}
	return libraries, nil
}

func (cfg *Config) getPlexLibrary() {
	libraries, err := getPlexLibraries(*cfg)
	if err != nil {
		log.Fatalf("failed to fetch libraries: %s", err.Error())
	}

	for _, library := range libraries.MediaContainer.Library {
		if cfg.Plex.LibraryName == library.Title {
			cfg.Plex.LibraryID = library.Key
		}
	}

	log.Fatalf("no library named %s found, please check LIBRARY_NAME variable", cfg.Plex.LibraryName)
}

func refreshPlexLibrary(cfg Config) error {
	params := fmt.Sprintf("/library/sections/%s/refresh?X-Plex-Token=%s", cfg.Plex.LibraryID, cfg.Creds.APIKey)

	_, err := makeRequest("GET", cfg.URL+params, nil, cfg.Creds.Headers)
	if err != nil {
		return fmt.Errorf("refreshPlexLibrary(): %s", err.Error())
	}
	return nil
}

func searchPlexSong(cfg Config, track Track) (string, error) {
	params := fmt.Sprintf("/library/search?query=%s", url.QueryEscape(track.Title))


	body, err := makeRequest("GET", cfg.URL+params, nil, cfg.Creds.Headers)
	if err != nil {
		return "", fmt.Errorf("searchPlexSong(): failed request for '%s': %s", track.Title, err.Error())
	}
	var searchResults PlexSearch

	err = parseResp(body, &searchResults)
	if err != nil {
		return "", fmt.Errorf("searchPlexSong(): failed to parse response for '%s': %s", track.Title, err.Error())
	}
	key, err := getPlexSong(track, searchResults)
	if err != nil {
		return "", fmt.Errorf("searchPlexSong(): %s", err.Error())
	}
	return key, nil
}

func getPlexSong(track Track, searchResults PlexSearch) (string, error) {

	for _, result := range searchResults.MediaContainer.SearchResult {
		if result.Metadata.Type == "track" && result.Metadata.Title == track.Title && result.Metadata.ParentTitle == track.Album {
			return result.Metadata.Key, nil
		}
	}
	return "", fmt.Errorf("failed to find '%s' by '%s' in %s album", track.Title, track.Artist, track.Album)
}

func searchPlexPlaylist(cfg Config) (string, error) {
	params := fmt.Sprintf("/playlists?X-Plex-Token=%s", cfg.Creds.APIKey)

	body, err := makeRequest("GET", cfg.URL+params, nil, cfg.Creds.Headers)
	if err != nil {
		return "", fmt.Errorf("searchPlexPlaylist(): failed to request playlists: %s", err.Error())
	}

	var playlists PlexPlaylist
	err = parseResp(body, &playlists)
	if err != nil {
		return "", fmt.Errorf("searchPlexPlaylist(): failed to parse response: %s", err.Error())
	}

	key := getPlexPlaylist(playlists, cfg.PlaylistName)
	if key == "" {
		debug.Debug("no playlist found")
	}
	return key, nil
}

func getPlexPlaylist(playlists PlexPlaylist, playlistName string) string {

	for _, playlist := range playlists.MediaContainer.Metadata {
		if playlist.Title == playlistName {
			return playlist.Key
		}
	}
	return ""
}

func getPlexServer(cfg Config) (string, error) {
	params := fmt.Sprintf("/identity?X-Plex-Token=%s", cfg.Creds.APIKey)

	body, err := makeRequest("GET", cfg.URL+params, nil, cfg.Creds.Headers)
	if err != nil {
		return "", fmt.Errorf("getPlexServer(): failed to create playlists: %s", err.Error())
	}

	var server PlexServer

	err = parseResp(body, &server)
	if err != nil {
		return "", fmt.Errorf("getPlexServer(): failed to parse response: %s", err.Error())
	}
	return server.MediaContainer.MachineIdentifier, nil
}

func createPlexPlaylist(cfg Config, machineID string) (string, error) {
	params := fmt.Sprintf("/playlists?title=%s&type=audio&smart=0&uri=server://%s/com.plexapp.plugins.library/%s&X-Plex-Token=%s", cfg.PlaylistName, machineID, cfg.Plex.LibraryID, cfg.Creds.APIKey)

	body, err := makeRequest("POST", cfg.URL+params, nil, cfg.Creds.Headers)
	if err != nil {
		return "", fmt.Errorf("createPlexPlaylist(): failed to create playlists: %s", err.Error())
	}

	var playlist PlexPlaylist

	err = parseResp(body, &playlist)
	if err != nil {
		return "", fmt.Errorf("createPlexPlaylist(): failed to parse response: %s", err.Error())
	}

	return playlist.MediaContainer.Metadata[0].Key, nil
}

func addToPlexPlaylist(cfg Config, playlistKey, machineID string, tracks []Track) {
	for _, track := range tracks {
		params := fmt.Sprintf("/playlists/%s?uri=server://%s/com.plexapp.plugins.library/%s&X-Plex-Token=%s", playlistKey, machineID, track.ID, cfg.Creds.APIKey)

		_, err := makeRequest("PUT", cfg.URL+params, nil, cfg.Creds.Headers)
		if err != nil {
			log.Printf("addToPlexPlaylist(): failed to add %s to playlist: %s", track.Title, err.Error())
		}
	}
}

func deletePlexPlaylist(cfg Config, playlistKey string) error {
	params := fmt.Sprintf("/playlists/%s?X-Plex-Token=%s", playlistKey, cfg.Creds.APIKey)

	_, err := makeRequest("DELETE", cfg.URL+params, nil, cfg.Creds.Headers)
	if err != nil {
		return fmt.Errorf("deletePlexPlaylist(): failed to delete plex playlist: %s", err.Error())
	}
	return nil
}