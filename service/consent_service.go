package service

import (
	"context"
	"fmt"
	"strings"

	"github.com/sirupsen/logrus"

	"github.com/malekradhouane/magic/api"
	"github.com/malekradhouane/magic/pkg/interfaces"
	"github.com/malekradhouane/magic/pkg/mailer"
	"github.com/malekradhouane/magic/store/types"
)

// Exact consent texts shown to users. Stored alongside each record so the proof
// reflects what the user actually agreed to at submission time.
const (
	contactConsentText = "J'accepte que les informations saisies dans ce formulaire soient " +
		"utilisées pour traiter ma demande, conformément à la politique de confidentialité."
	newsletterConsentText = "En vous inscrivant, vous acceptez de recevoir nos communications " +
		"marketing par email. Vous pouvez vous désinscrire à tout moment via le lien de " +
		"désabonnement présent dans chacun de nos emails."

	contactSource    = "contact_form"
	newsletterSource = "newsletter_footer"
)

// ConsentMeta carries request-scoped metadata captured at the handler boundary.
type ConsentMeta struct {
	IPAddress string
	UserAgent string
}

// ConsentService handles contact and newsletter consent business logic.
type ConsentService struct {
	store        types.ConsentStore
	settings     *SettingsService
	mailer       mailer.Mailer
	mailFromName string
	mailFromMail string
	frontendURL  string
	logger       *logrus.Logger
}

// NewConsentService constructs a ConsentService. mailer and settings may be nil;
// email notifications are then skipped gracefully.
func NewConsentService(
	store types.ConsentStore,
	settings *SettingsService,
	mail mailer.Mailer,
	mailFromName, mailFromEmail, frontendURL string,
	logger *logrus.Logger,
) *ConsentService {
	if logger == nil {
		logger = logrus.New()
	}
	return &ConsentService{
		store:        store,
		settings:     settings,
		mailer:       mail,
		mailFromName: mailFromName,
		mailFromMail: mailFromEmail,
		frontendURL:  strings.TrimRight(frontendURL, "/"),
		logger:       logger,
	}
}

// SubmitContact validates and stores a contact-form submission with its consent proof.
func (s *ConsentService) SubmitContact(ctx context.Context, req *api.ContactRequest, meta ConsentMeta) (*interfaces.Consent, error) {
	if req == nil {
		return nil, fmt.Errorf("contact request is nil")
	}
	if !req.Consent {
		return nil, fmt.Errorf("consent is required")
	}

	consent := &interfaces.Consent{
		Type:        interfaces.ConsentTypeContact,
		Source:      contactSource,
		Email:       strings.TrimSpace(strings.ToLower(req.Email)),
		Name:        strings.TrimSpace(req.Name),
		Subject:     strings.TrimSpace(req.Subject),
		Message:     strings.TrimSpace(req.Message),
		Consent:     true,
		ConsentText: contactConsentText,
		IPAddress:   meta.IPAddress,
		UserAgent:   meta.UserAgent,
	}

	stored, err := s.store.Create(ctx, consent)
	if err != nil {
		return nil, fmt.Errorf("store contact consent: %w", err)
	}
	s.logger.WithField("email", consent.Email).Info("contact consent stored")
	s.sendContactNotificationAsync(stored)
	return stored, nil
}

// SubscribeNewsletter validates and stores a newsletter subscription with its consent proof.
func (s *ConsentService) SubscribeNewsletter(ctx context.Context, req *api.NewsletterSubscribeRequest, meta ConsentMeta) (*interfaces.Consent, error) {
	if req == nil {
		return nil, fmt.Errorf("newsletter request is nil")
	}
	if !req.Consent {
		return nil, fmt.Errorf("consent is required")
	}

	consent := &interfaces.Consent{
		Type:        interfaces.ConsentTypeNewsletter,
		Source:      newsletterSource,
		Email:       strings.TrimSpace(strings.ToLower(req.Email)),
		Consent:     true,
		ConsentText: newsletterConsentText,
		IPAddress:   meta.IPAddress,
		UserAgent:   meta.UserAgent,
	}

	stored, err := s.store.Create(ctx, consent)
	if err != nil {
		return nil, fmt.Errorf("store newsletter consent: %w", err)
	}
	s.logger.WithField("email", consent.Email).Info("newsletter consent stored")
	s.sendNewsletterWelcomeAsync(stored)
	return stored, nil
}

// List returns stored consent proofs for the admin dashboard.
func (s *ConsentService) List(ctx context.Context, consentType string, limit, offset int) ([]*interfaces.Consent, int64, error) {
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	if offset < 0 {
		offset = 0
	}
	switch consentType {
	case "", interfaces.ConsentTypeContact, interfaces.ConsentTypeNewsletter:
	default:
		return nil, 0, fmt.Errorf("invalid consent type: %s", consentType)
	}
	return s.store.List(ctx, consentType, limit, offset)
}
