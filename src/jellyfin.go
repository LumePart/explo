package main

import (
	"bytes"
	"encoding/json"
	"explo/src/debug"
	"fmt"
	"log"
	"net/url"
	"strings"
)

type Paths []struct {
	Name           string         `json:"Name"`
	Locations      []string       `json:"Locations"`
	CollectionType string         `json:"CollectionType"`
	ItemID         string         `json:"ItemId"`
	RefreshStatus  string         `json:"RefreshStatus"`
}

type Search struct {
	SearchHints      []SearchHints `json:"SearchHints"`
	TotalRecordCount int           `json:"TotalRecordCount"`
}
type SearchHints struct {
	ItemID                  string    `json:"ItemId"`
	ID                      string    `json:"Id"`
	Name                    string    `json:"Name"`
	Album                   string    `json:"Album"`
	AlbumID                 string    `json:"AlbumId"`
	AlbumArtist             string    `json:"AlbumArtist"`
}

type Audios struct {
	Items            []Items `json:"Items"`
	TotalRecordCount int     `json:"TotalRecordCount"`
	StartIndex       int     `json:"StartIndex"`
}

type Items struct {
	Name              string          `json:"Name"`
	ServerID          string          `json:"ServerId"`
	ID                string          `json:"Id"`
	Path			  string		  `json:"Path"`
	Album             string          `json:"Album,omitempty"`
	AlbumArtist       string          `json:"AlbumArtist,omitempty"`
}

type JFPlaylist struct {
	ID string `json:"Id"`
}

func (cfg *Credentials) jfHeader() {
	cfg.Headers = make(map[string]string)

	cfg.Headers["Authorization"] = fmt.Sprintf("MediaBrowser Token=%s, Client=Explo", cfg.APIKey)
	
}

func jfAllPaths(cfg Config) (Paths, error) {
	params := "/Library/VirtualFolders"

	body, err := makeRequest("GET", cfg.URL+params, nil, cfg.Creds.Headers)
	if err != nil {
		return nil, fmt.Errorf("jfAllPaths(): %s", err.Error())
	}

	var paths Paths
	if err = parseResp(body, &paths); err != nil {
		return nil, fmt.Errorf("jfAllPaths(): %s", err.Error())
	}
	return paths, nil
}

func (cfg *Config) getJfPath()  { // Gets Librarys ID
	paths, err := jfAllPaths(*cfg)
	if err != nil {
		log.Fatalf("getJfPath(): %s", err.Error())
	}

	for _, path := range paths {
		if path.Name == cfg.Jellyfin.LibraryName {
			cfg.Jellyfin.LibraryID = path.ItemID
		}
	}
}

func jfAddPath(cfg Config)  { // adds Jellyfin library, if not set
	cleanPath := url.PathEscape(cfg.Youtube.DownloadDir)
	params := fmt.Sprintf("/Library/VirtualFolders?name=%s&paths=%s&collectionType=music&refreshLibrary=true", cfg.Jellyfin.LibraryName, cleanPath)
	payload := []byte(`{
		"LibraryOptions": {
		  "Enabled": true,
		  "EnableRealtimeMonitor": true,
		  "EnableLUFSScan": false
		}
	  }`)

	body, err := makeRequest("POST", cfg.URL+params, bytes.NewReader(payload), cfg.Creds.Headers)
	if err != nil {
		debug.Debug(fmt.Sprintf("response: %s", body))
		log.Fatalf("failed to add library to Jellyfin using the download path, please define a library name using LIBRARY_NAME in .env: %s", err.Error())
	}
}

func refreshJfLibrary(cfg Config) error {
	params := fmt.Sprintf("/Items/%s/Refresh", cfg.Jellyfin.LibraryID)

	if _, err := makeRequest("POST", cfg.URL+params, nil, cfg.Creds.Headers); err != nil {
		return fmt.Errorf("refreshJfLibrary(): %s", err.Error())
	}
	return nil
}

func getJfSong(cfg Config, track Track) (string, error) { // Gets all files in Explo library and filters out new ones
	params := fmt.Sprintf("/Items?parentId=%s&fields=Path", cfg.Jellyfin.LibraryID)

	body, err := makeRequest("GET", cfg.URL+params, nil, cfg.Creds.Headers)
	if err != nil {
		return "", fmt.Errorf("getJfSong(): %s", err.Error())
	}

	var results Audios
	if err = parseResp(body, &results); err != nil {
		return "", fmt.Errorf("getJfSong(): %s", err.Error())
	}

	for _, item := range results.Items {
		if strings.Contains(item.Path, track.File) {
			return item.ID, nil
		}
	}
	return "", nil
}

func findJfPlaylist(cfg Config) (string, error) {
	params := fmt.Sprintf("/Search/Hints?searchTerm=%s&mediaTypes=Playlist", cfg.PlaylistName)

	body, err := makeRequest("GET", cfg.URL+params, nil, cfg.Creds.Headers)
	if err != nil {
		return "", fmt.Errorf("findJfPlaylist(): %s", err.Error())
	}

	var results Search
	if err = parseResp(body, &results); err != nil {
		return "", fmt.Errorf("findJfPlaylist(): %s", err.Error())
	}
	
	if len(results.SearchHints) != 0 {
		return results.SearchHints[0].ID, nil
	} else {
		return "", fmt.Errorf("no results found for playlist: %s", cfg.PlaylistName)
	}
}

func createJfPlaylist(cfg Config, tracks []Track) (string, error) {
	var songIDs []string
	
	for _, track := range tracks {
		if track.ID == "" {
			songID, err := getJfSong(cfg, track)
			if songID == "" || err != nil {
				debug.Debug(fmt.Sprintf("could not get %s", track.File))
				continue
			}
			track.ID = songID
		}
		songIDs = append(songIDs, track.ID)
	}
	
	params := "/Playlists"

	IDs, err := json.Marshal(songIDs)
	if err != nil {
		debug.Debug(fmt.Sprintf("songIDs: %v", songIDs))
		return "", fmt.Errorf("createJfPlaylist(): %s", err.Error())
	}

	payload := []byte(fmt.Sprintf(`
		{
		"Name": "%s",
		"Ids": %s,
		"MediaType": "Audio",
		"UserId": "%s"
		}`, cfg.PlaylistName, IDs, cfg.Creds.APIKey))

	body, err := makeRequest("POST", cfg.URL+params, bytes.NewReader(payload), cfg.Creds.Headers)
	if err != nil {
		return "", fmt.Errorf("createJfPlaylist(): %s", err.Error())
	}
	var playlist JFPlaylist
	if err = parseResp(body, &playlist); err != nil {
		return "", fmt.Errorf("createJfPlaylist(): %s", err.Error())
	}
	return playlist.ID, nil
}

func updateJfPlaylist(cfg Config, ID, overview string) error {
	params := fmt.Sprintf("/Items/%s", ID)

	payload := []byte(fmt.Sprintf(`
		{
		"Id":"%s",
		"Name":"%s",
		"Overview":"%s",
		"Genres":[],
		"Tags":[],
		"ProviderIds":{}
		}`, ID, cfg.PlaylistName, overview)) // the additional fields have to be added, otherwise JF returns code 400

	if _, err := makeRequest("POST", cfg.URL+params, bytes.NewBuffer(payload), cfg.Creds.Headers); err != nil {
		return err
	}
	return nil
}

func deleteJfPlaylist(cfg Config, ID string) error {
	params := fmt.Sprintf("/Items/%s", ID)

	if _, err := makeRequest("DELETE", cfg.URL+params, nil, cfg.Creds.Headers); err != nil {
		return fmt.Errorf("deleyeJfPlaylist(): %s", err.Error())
	}
	return nil
}