package template

import (
	"bytes"
	_ "embed"
	"fmt"
	"html/template"
)

//go:embed activateAccount.html
var activateAccountTpl []byte

//go:embed resetPassword.html
var resetPasswordTpl []byte

//go:embed orderConfirmation.html
var orderConfirmationTpl []byte

//go:embed orderShipped.html
var orderShippedTpl []byte

//go:embed contactNotification.html
var contactNotificationTpl []byte

//go:embed newsletterWelcome.html
var newsletterWelcomeTpl []byte

// OrderConfirmationData is passed to the order confirmation email template.
type OrderConfirmationData struct {
	CustomerName    string
	OrderNumber     string
	OrderDate       string
	OrderURL        string
	Items           []OrderEmailItem
	Subtotal        string
	ShippingFee     string
	DiscountAmount  string
	HasDiscount     bool
	TotalPrice      string
	PaymentMethod   string
	PromoCode       string
	HasPromo        bool
	ShippingAddress string
	ShippingPhone   string
}

// OrderEmailItem is a line item in order emails.
type OrderEmailItem struct {
	Name      string
	Size      string
	Color     string
	Quantity  int
	LineTotal string
	HasSize   bool
	HasColor  bool
}

// OrderShippedData is passed to the order shipped email template.
type OrderShippedData struct {
	CustomerName   string
	OrderNumber    string
	OrderURL       string
	TrackingNumber string
	HasTracking    bool
}

// ContactNotificationData is passed to the contact notification email template.
type ContactNotificationData struct {
	Name       string
	Email      string
	Subject    string
	Message    string
	ReceivedAt string
	IPAddress  string
}

// NewsletterWelcomeData is passed to the newsletter welcome email template.
type NewsletterWelcomeData struct {
	PrivacyURL string
}

// RenderInviteUser returns the activateAccount HTML with the link injected
func RenderActivateAccount(link string) (string, error) {
	if len(activateAccountTpl) == 0 {
		return "", fmt.Errorf("" +
			" template not embedded")
	}
	tpl, err := template.New("activateAccount").Parse(string(activateAccountTpl))
	if err != nil {
		return "", fmt.Errorf("parse invite template: %w", err)
	}

	var buf bytes.Buffer
	data := struct {
		Link string
	}{Link: link}

	if err := tpl.Execute(&buf, data); err != nil {
		return "", fmt.Errorf("execute activateAccount template: %w", err)
	}
	return buf.String(), nil
}

// RenderResetPassword returns the reset password HTML with the link injected
func RenderResetPassword(link string) (string, error) {
	if len(resetPasswordTpl) == 0 {
		return "", fmt.Errorf("reset password template not embedded")
	}
	tpl, err := template.New("resetPassword").Parse(string(resetPasswordTpl))
	if err != nil {
		return "", fmt.Errorf("parse reset password template: %w", err)
	}

	var buf bytes.Buffer
	data := struct {
		Link string
	}{Link: link}

	if err := tpl.Execute(&buf, data); err != nil {
		return "", fmt.Errorf("execute reset password template: %w", err)
	}
	return buf.String(), nil
}

// RenderOrderConfirmation returns the order confirmation HTML.
func RenderOrderConfirmation(data OrderConfirmationData) (string, error) {
	return renderTemplate("orderConfirmation", orderConfirmationTpl, data)
}

// RenderOrderShipped returns the order shipped HTML.
func RenderOrderShipped(data OrderShippedData) (string, error) {
	return renderTemplate("orderShipped", orderShippedTpl, data)
}

// RenderContactNotification returns the contact notification HTML.
func RenderContactNotification(data ContactNotificationData) (string, error) {
	return renderTemplate("contactNotification", contactNotificationTpl, data)
}

// RenderNewsletterWelcome returns the newsletter welcome HTML.
func RenderNewsletterWelcome(data NewsletterWelcomeData) (string, error) {
	return renderTemplate("newsletterWelcome", newsletterWelcomeTpl, data)
}

func renderTemplate(name string, tplBytes []byte, data interface{}) (string, error) {
	if len(tplBytes) == 0 {
		return "", fmt.Errorf("%s template not embedded", name)
	}
	tpl, err := template.New(name).Parse(string(tplBytes))
	if err != nil {
		return "", fmt.Errorf("parse %s template: %w", name, err)
	}
	var buf bytes.Buffer
	if err := tpl.Execute(&buf, data); err != nil {
		return "", fmt.Errorf("execute %s template: %w", name, err)
	}
	return buf.String(), nil
}
