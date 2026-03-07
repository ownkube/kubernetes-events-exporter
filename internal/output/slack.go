package output

import (
	"context"
	"log/slog"
	"sort"

	"github.com/ownkube/kubernetes-events-exporter/internal/event"
	"github.com/slack-go/slack"
)

type SlackConfig struct {
	Token      string            `yaml:"token"`
	Channel    string            `yaml:"channel"`
	Message    string            `yaml:"message"`
	Color      string            `yaml:"color"`
	Footer     string            `yaml:"footer"`
	Title      string            `yaml:"title"`
	AuthorName string            `yaml:"author_name"`
	Fields     map[string]string `yaml:"fields"`
}

type SlackHandler struct {
	cfg    *SlackConfig
	client *slack.Client
}

func NewSlackHandler(cfg *SlackConfig) (Handler, error) {
	return &SlackHandler{
		cfg:    cfg,
		client: slack.New(cfg.Token),
	}, nil
}

func (s *SlackHandler) Send(ctx context.Context, ev *event.EnrichedEvent) error {
	channel, err := RenderTemplate(ev, s.cfg.Channel)
	if err != nil {
		return err
	}

	message, err := RenderTemplate(ev, s.cfg.Message)
	if err != nil {
		return err
	}

	options := []slack.MsgOption{slack.MsgOptionText(message, true)}
	if s.cfg.Fields != nil {
		fields := make([]slack.AttachmentField, 0)
		for k, v := range s.cfg.Fields {
			fieldText, err := RenderTemplate(ev, v)
			if err != nil {
				return err
			}

			fields = append(fields, slack.AttachmentField{
				Title: k,
				Value: fieldText,
				Short: false,
			})
		}

		sort.SliceStable(fields, func(i, j int) bool {
			return fields[i].Title < fields[j].Title
		})

		// make slack attachment
		slackAttachment := slack.Attachment{}
		slackAttachment.Fields = fields
		if s.cfg.AuthorName != "" {
			slackAttachment.AuthorName, err = RenderTemplate(ev, s.cfg.AuthorName)
			if err != nil {
				return err
			}
		}
		if s.cfg.Color != "" {
			slackAttachment.Color, err = RenderTemplate(ev, s.cfg.Color)
			if err != nil {
				return err
			}
		}
		if s.cfg.Title != "" {
			slackAttachment.Title, err = RenderTemplate(ev, s.cfg.Title)
			if err != nil {
				return err
			}
		}
		if s.cfg.Footer != "" {
			slackAttachment.Footer, err = RenderTemplate(ev, s.cfg.Footer)
			if err != nil {
				return err
			}
		}

		options = append(options, slack.MsgOptionAttachments(slackAttachment))
	}

	ch, ts, text, err := s.client.SendMessageContext(ctx, channel, options...)
	slog.Debug("Slack Response", "ch", ch, "ts", ts, "text", text, "error", err)
	return err
}

func (s *SlackHandler) Close() {
	// No-op
}
