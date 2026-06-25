package auth

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"

	"github.com/rs/zerolog/log"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"

	"github.com/innacy/assistant-agent/pkg/config"
)

var Scopes = []string{
	"https://www.googleapis.com/auth/gmail.readonly",
	"https://www.googleapis.com/auth/calendar.readonly",
	"https://www.googleapis.com/auth/tasks.readonly",
	"https://www.googleapis.com/auth/contacts.readonly",
}

func RunAuthFlow(cfg config.GoogleConfig) error {
	b, err := os.ReadFile(cfg.CredentialsFile)
	if err != nil {
		return fmt.Errorf("unable to read credentials file: %w", err)
	}

	oauthCfg, err := google.ConfigFromJSON(b, Scopes...)
	if err != nil {
		return fmt.Errorf("unable to parse credentials: %w", err)
	}

	token, err := getTokenFromWeb(oauthCfg)
	if err != nil {
		return err
	}

	return saveToken(cfg.TokenFile, token)
}

func GetClient(cfg config.GoogleConfig) (*http.Client, error) {
	b, err := os.ReadFile(cfg.CredentialsFile)
	if err != nil {
		return nil, fmt.Errorf("unable to read credentials file: %w", err)
	}

	oauthCfg, err := google.ConfigFromJSON(b, Scopes...)
	if err != nil {
		return nil, fmt.Errorf("unable to parse credentials: %w", err)
	}

	token, err := loadToken(cfg.TokenFile)
	if err != nil {
		return nil, fmt.Errorf("token not found (run --auth first): %w", err)
	}

	tokenSource := oauthCfg.TokenSource(context.Background(), token)
	newToken, err := tokenSource.Token()
	if err != nil {
		return nil, fmt.Errorf("token refresh failed (run --auth again): %w", err)
	}

	if newToken.AccessToken != token.AccessToken {
		if err := saveToken(cfg.TokenFile, newToken); err != nil {
			log.Warn().Err(err).Msg("failed to save refreshed token")
		}
	}

	return oauth2.NewClient(context.Background(), tokenSource), nil
}

func getTokenFromWeb(cfg *oauth2.Config) (*oauth2.Token, error) {
	authURL := cfg.AuthCodeURL("state-token", oauth2.AccessTypeOffline, oauth2.ApprovalForce)
	fmt.Printf("\nOpen this URL in your browser:\n\n%s\n\n", authURL)
	fmt.Print("Enter the authorization code: ")

	var code string
	if _, err := fmt.Scan(&code); err != nil {
		return nil, fmt.Errorf("unable to read authorization code: %w", err)
	}

	token, err := cfg.Exchange(context.Background(), code)
	if err != nil {
		return nil, fmt.Errorf("unable to exchange code for token: %w", err)
	}
	return token, nil
}

func loadToken(path string) (*oauth2.Token, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var token oauth2.Token
	return &token, json.NewDecoder(f).Decode(&token)
}

func saveToken(path string, token *oauth2.Token) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()

	log.Info().Str("path", path).Msg("token saved")
	return json.NewEncoder(f).Encode(token)
}
