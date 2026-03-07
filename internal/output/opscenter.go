package output

import (
	"context"
	"fmt"
	"strconv"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/ssm"
	"github.com/aws/aws-sdk-go/service/ssm/ssmiface"
	"github.com/ownkube/kubernetes-events-exporter/internal/event"
)

// OpsCenterConfig is the configuration of the Handler.
type OpsCenterConfig struct {
	Category        string            `yaml:"category"`
	Description     string            `yaml:"description"`
	Notifications   []string          `yaml:"notifications"`
	OperationalData map[string]string `yaml:"operationalData"`
	Priority        string            `yaml:"priority"`
	Region          string            `yaml:"region"`
	RelatedOpsItems []string          `yaml:"relatedOpsItems"`
	Severity        string            `yaml:"severity"`
	Source          string            `yaml:"source"`
	Tags            map[string]string `yaml:"tags"`
	Title           string            `yaml:"title"`
}

// OpsCenterHandler is an AWS OpsCenter notification path.
type OpsCenterHandler struct {
	cfg *OpsCenterConfig
	svc ssmiface.SSMAPI
}

// NewOpsCenterHandler returns a new OpsCenterHandler.
func NewOpsCenterHandler(cfg *OpsCenterConfig) (Handler, error) {
	sess, err := session.NewSession(&aws.Config{
		Region: aws.String(cfg.Region)},
	)
	if err != nil {
		return nil, err
	}

	svc := ssm.New(sess)
	return &OpsCenterHandler{
		cfg: cfg,
		svc: svc,
	}, nil
}

// Send creates an OpsItem in AWS OpsCenter.
func (s *OpsCenterHandler) Send(ctx context.Context, ev *event.EnrichedEvent) error {
	oi := ssm.CreateOpsItemInput{}
	t, err := RenderTemplate(ev, s.cfg.Title)
	if err != nil {
		return err
	}
	oi.Title = aws.String(t)
	d, err := RenderTemplate(ev, s.cfg.Description)
	if err != nil {
		return err
	}
	oi.Description = aws.String(d)
	su, err := RenderTemplate(ev, s.cfg.Source)
	if err != nil {
		return err
	}
	oi.Source = aws.String(su)

	// Category is optional although highly recommended
	if len(s.cfg.Category) != 0 {
		c, err := RenderTemplate(ev, s.cfg.Category)
		if err != nil {
			return err
		}
		oi.Category = aws.String(c)
	}

	// Severity is optional although highly recommended
	if len(s.cfg.Severity) != 0 {
		se, err := RenderTemplate(ev, s.cfg.Severity)
		if err != nil {
			return err
		}
		oi.Severity = aws.String(se)
	}

	// Priority is optional although highly recommended
	if len(s.cfg.Priority) != 0 {
		p, err := RenderTemplate(ev, s.cfg.Priority)
		if err != nil {
			return err
		}
		n, err := strconv.ParseInt(p, 10, 64)
		if err != nil {
			return fmt.Errorf("Priority is a non int")
		}
		oi.Priority = aws.Int64(n)
	}
	if s.cfg.OperationalData != nil {
		oids := make(map[string]*ssm.OpsItemDataValue)
		for k, v := range s.cfg.OperationalData {
			dv, err := RenderTemplate(ev, v)
			if err != nil {
				return err
			}
			oids[k] = &ssm.OpsItemDataValue{Type: aws.String("SearchableString"), Value: aws.String(dv)}
		}
		oi.OperationalData = oids
	}
	if s.cfg.Tags != nil {
		tvs := make([]*ssm.Tag, 0)
		for k, v := range s.cfg.Tags {
			tv, err := RenderTemplate(ev, v)
			if err != nil {
				return err
			}
			tvs = append(tvs, &ssm.Tag{Key: aws.String(k), Value: aws.String(tv)})
		}
		oi.Tags = tvs
	}
	if s.cfg.RelatedOpsItems != nil {
		ris := make([]*ssm.RelatedOpsItem, 0)
		for _, v := range s.cfg.OperationalData {
			ri, err := RenderTemplate(ev, v)
			if err != nil {
				return err
			}
			ris = append(ris, &ssm.RelatedOpsItem{OpsItemId: aws.String(ri)})
		}
		oi.RelatedOpsItems = ris
	}
	if s.cfg.Notifications != nil {
		ns := make([]*ssm.OpsItemNotification, 0)
		for _, v := range s.cfg.Notifications {
			n, err := RenderTemplate(ev, v)
			if err != nil {
				return err
			}
			ns = append(ns, &ssm.OpsItemNotification{Arn: aws.String(n)})
		}
		oi.Notifications = ns
	}

	_, createErr := s.svc.CreateOpsItemWithContext(ctx, &oi)

	return createErr
}

// Close is a no-op for OpsCenter.
func (s *OpsCenterHandler) Close() {
	// No-op
}
