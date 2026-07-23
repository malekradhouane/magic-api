package service

import (
	"context"
	"fmt"
	"strings"

	"github.com/sirupsen/logrus"

	"github.com/malekradhouane/magic/pkg/interfaces"
	"github.com/malekradhouane/magic/pkg/mailer/template"
)

// sendContactNotificationAsync emails the incoming contact message to the store
// inbox so the team is notified without polling the database.
func (s *ConsentService) sendContactNotificationAsync(c *interfaces.Consent) {
	if c == nil {
		return
	}
	go s.sendContactNotification(context.Background(), c)
}

func (s *ConsentService) sendContactNotification(ctx context.Context, c *interfaces.Consent) {
	if s.mailer == nil {
		s.logger.Warn("Mailer not configured, skipping contact notification email")
		return
	}

	recipient := s.contactRecipient(ctx)
	if recipient == "" {
		s.logger.Warn("No contact recipient configured, skipping contact notification email")
		return
	}

	subject := fmt.Sprintf("Nouveau message de contact — %s", c.Subject)
	text := fmt.Sprintf(
		"Nouveau message via le formulaire de contact.\n\nNom : %s\nEmail : %s\nSujet : %s\n\nMessage :\n%s\n\nReçu le %s (IP : %s)",
		c.Name, c.Email, c.Subject, c.Message, c.CreatedAt.Format("2006-01-02 15:04"), c.IPAddress,
	)
	htmlPart, err := template.RenderContactNotification(template.ContactNotificationData{
		Name:       c.Name,
		Email:      c.Email,
		Subject:    c.Subject,
		Message:    c.Message,
		ReceivedAt: c.CreatedAt.Format("2006-01-02 15:04"),
		IPAddress:  c.IPAddress,
	})
	if err != nil {
		s.logger.WithError(err).Error("Failed to render contact notification email")
		return
	}

	// Send from the store identity; the customer email is included in the body
	// for a direct reply, since the mailer interface is minimal.
	if err := s.mailer.Send(ctx, s.mailFromName, s.mailFromMail, "", recipient, subject, text, htmlPart); err != nil {
		s.logger.WithError(err).WithFields(logrus.Fields{
			"recipient": recipient,
			"email":     c.Email,
		}).Error("Failed to send contact notification email")
		return
	}
	s.logger.WithField("recipient", recipient).Info("Contact notification email sent")
}

// sendNewsletterWelcomeAsync emails a confirmation to a new newsletter subscriber.
func (s *ConsentService) sendNewsletterWelcomeAsync(c *interfaces.Consent) {
	if c == nil {
		return
	}
	go s.sendNewsletterWelcome(context.Background(), c)
}

func (s *ConsentService) sendNewsletterWelcome(ctx context.Context, c *interfaces.Consent) {
	if s.mailer == nil {
		s.logger.Warn("Mailer not configured, skipping newsletter welcome email")
		return
	}

	privacyURL := s.frontendURL + "/confidentialite"
	subject := "Bienvenue dans le club Magic"
	text := fmt.Sprintf(
		"Merci pour votre inscription à la newsletter Magic !\n\n"+
			"Vous recevrez nos nouveautés et offres exclusives. Vous pouvez vous "+
			"désinscrire à tout moment via le lien présent dans chacun de nos emails.\n\n"+
			"Politique de confidentialité : %s\n",
		privacyURL,
	)
	htmlPart, err := template.RenderNewsletterWelcome(template.NewsletterWelcomeData{
		PrivacyURL: privacyURL,
	})
	if err != nil {
		s.logger.WithError(err).Error("Failed to render newsletter welcome email")
		return
	}

	if err := s.mailer.Send(ctx, s.mailFromName, s.mailFromMail, "", c.Email, subject, text, htmlPart); err != nil {
		s.logger.WithError(err).WithField("email", c.Email).Error("Failed to send newsletter welcome email")
		return
	}
	s.logger.WithField("email", c.Email).Info("Newsletter welcome email sent")
}

// contactRecipient resolves the store inbox from settings, falling back to the
// configured sender address.
func (s *ConsentService) contactRecipient(ctx context.Context) string {
	if s.settings != nil {
		if setting, err := s.settings.GetByKey(ctx, "general"); err == nil && setting != nil {
			if v, ok := setting.Value["contact_email"].(string); ok {
				if email := strings.TrimSpace(v); email != "" {
					return email
				}
			}
		}
	}
	return strings.TrimSpace(s.mailFromMail)
}
