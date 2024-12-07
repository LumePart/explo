package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
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

func (cfg *Credentials) PlexHeader() {
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

func (cfg *Config) getPlexLibrary() error {
	libraries, err := getPlexLibraries(*cfg)
	if err != nil {
		return fmt.Errorf("failed to fetch libraries: %s", err.Error())
	}

	for _, library := range libraries.MediaContainer.Library {
		if cfg.Plex.LibraryName == library.Title {
			cfg.Plex.LibraryID = library.Key
			return nil
		}
	}

	return fmt.Errorf("no library named %s found, please check LIBRARY_NAME variable", cfg.Plex.LibraryName)
}

func refreshPlexLibrary(cfg Config) error {
	params := fmt.Sprintf("/library/sections/%s/refresh?X-Plex-Token=%s", cfg.Plex.LibraryID, cfg.Creds.APIKey)

	_, err := makeRequest("GET", cfg.URL+params, nil, cfg.Creds.Headers)
	if err != nil {
		return fmt.Errorf("refreshPlexLibrary(): %s", err.Error())
	}
	return nil
}