package gmail

import (
	"context"
	"encoding/base64"
	"fmt"
	"net/http"
	"strings"

	"github.com/rs/zerolog/log"
	"google.golang.org/api/gmail/v1"
	"google.golang.org/api/option"

	"github.com/innacy/assistant-agent/internal/models"
	"github.com/innacy/assistant-agent/pkg/config"
	"github.com/innacy/assistant-agent/pkg/sources"
)

type Syncer struct {
	client   *http.Client
	gmailCfg config.GmailConfig
}

func New(client *http.Client, gmailCfg config.GmailConfig) *Syncer {
	return &Syncer{client: client, gmailCfg: gmailCfg}
}

func (s *Syncer) Name() string { return models.SourceGmail }

func (s *Syncer) Sync(ctx context.Context, state *models.SyncState) ([]sources.RawItem, string, error) {
	srv, err := gmail.NewService(ctx, option.WithHTTPClient(s.client))
	if err != nil {
		return nil, "", fmt.Errorf("gmail service: %w", err)
	}

	query := buildQuery(s.gmailCfg)
	var messages []*gmail.Message

	if state != nil && state.LastPageToken != "" {
		messages, err = s.fetchByHistoryID(ctx, srv, state.LastPageToken, query)
		if err != nil {
			log.Warn().
				Err(err).
				Str("history_id", state.LastPageToken).
				Msg("gmail history ID expired or invalid, falling back to date query")
			messages, err = s.fetchByQuery(ctx, srv, query)
			if err != nil {
				return nil, "", err
			}
		}
	} else {
		messages, err = s.fetchByQuery(ctx, srv, query)
		if err != nil {
			return nil, "", err
		}
	}

	var items []sources.RawItem
	for _, msg := range messages {
		if msg == nil {
			continue
		}
		full, err := srv.Users.Messages.Get("me", msg.Id).Format("full").Context(ctx).Do()
		if err != nil {
			log.Warn().Err(err).Str("msg_id", msg.Id).Msg("failed to fetch message")
			continue
		}

		from, subject := extractHeaders(full)
		body := extractBody(full)

		item := ParseEmail(msg.Id, from, subject, body, s.gmailCfg.SenderWhitelist)
		if item != nil {
			items = append(items, *item)
		}
	}

	profile, _ := srv.Users.GetProfile("me").Context(ctx).Do()
	var historyID string
	if profile != nil {
		historyID = fmt.Sprintf("%d", profile.HistoryId)
	}

	log.Info().Int("emails_parsed", len(items)).Int("total_fetched", len(messages)).Msg("gmail sync complete")
	return items, historyID, nil
}

func (s *Syncer) fetchByQuery(ctx context.Context, srv *gmail.Service, query string) ([]*gmail.Message, error) {
	var messages []*gmail.Message
	call := srv.Users.Messages.List("me").Q(query).MaxResults(100)

	err := call.Pages(ctx, func(resp *gmail.ListMessagesResponse) error {
		messages = append(messages, resp.Messages...)
		return nil
	})

	return messages, err
}

func (s *Syncer) fetchByHistoryID(ctx context.Context, srv *gmail.Service, historyID, query string) ([]*gmail.Message, error) {
	var id uint64
	fmt.Sscanf(historyID, "%d", &id)

	resp, err := srv.Users.History.List("me").
		StartHistoryId(id).
		HistoryTypes("messageAdded").
		Context(ctx).
		Do()
	if err != nil {
		return nil, err
	}

	var messages []*gmail.Message
	for _, h := range resp.History {
		for _, added := range h.MessagesAdded {
			if added.Message != nil {
				messages = append(messages, added.Message)
			}
		}
	}
	return messages, nil
}

func buildQuery(cfg config.GmailConfig) string {
	parts := make([]string, 0, len(cfg.QueryFilters)+len(cfg.SenderWhitelist))
	for _, f := range cfg.QueryFilters {
		parts = append(parts, fmt.Sprintf("(%s)", f))
	}
	for _, sender := range cfg.SenderWhitelist {
		parts = append(parts, fmt.Sprintf("(from:%s)", sender))
	}
	return strings.Join(parts, " OR ")
}

func extractHeaders(msg *gmail.Message) (from, subject string) {
	if msg.Payload == nil {
		return
	}
	for _, h := range msg.Payload.Headers {
		switch strings.ToLower(h.Name) {
		case "from":
			from = h.Value
		case "subject":
			subject = h.Value
		}
	}
	return
}

func extractBody(msg *gmail.Message) string {
	if msg.Payload == nil {
		return ""
	}

	if msg.Payload.Body != nil && msg.Payload.Body.Data != "" {
		data, _ := base64.URLEncoding.DecodeString(msg.Payload.Body.Data)
		return string(data)
	}

	for _, part := range msg.Payload.Parts {
		if part.MimeType == "text/plain" && part.Body != nil && part.Body.Data != "" {
			data, _ := base64.URLEncoding.DecodeString(part.Body.Data)
			return string(data)
		}
	}

	for _, part := range msg.Payload.Parts {
		if part.MimeType == "text/html" && part.Body != nil && part.Body.Data != "" {
			data, _ := base64.URLEncoding.DecodeString(part.Body.Data)
			return string(data)
		}
	}

	return ""
}
