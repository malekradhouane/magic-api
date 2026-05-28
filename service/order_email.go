package service

import (
	"context"
	"fmt"
	"net/url"
	"strings"

	"github.com/sirupsen/logrus"

	"github.com/malekradhouane/magic/pkg/interfaces"
	"github.com/malekradhouane/magic/pkg/mailer/template"
)

func (os *OrderService) sendOrderConfirmationEmailAsync(order *interfaces.Order) {
	if order == nil {
		return
	}
	go os.sendOrderConfirmationEmail(context.Background(), order)
}

func (os *OrderService) sendOrderConfirmationEmail(ctx context.Context, order *interfaces.Order) {
	if order == nil {
		return
	}
	toEmail, toName, err := os.resolveOrderRecipient(ctx, order)
	if err != nil {
		os.logger.WithError(err).WithField("order_id", order.ID).Warn("Cannot send order confirmation: no recipient email")
		return
	}
	if os.mailer == nil {
		os.logger.WithField("order_id", order.ID).Warn(
			"Mailer not configured (set MAILJET_API_KEY_PUBLIC and MAILJET_API_KEY_PRIVATE), skipping order confirmation email",
		)
		return
	}

	html, err := template.RenderOrderConfirmation(os.buildOrderConfirmationData(order))
	if err != nil {
		os.logger.WithError(err).WithField("order_id", order.ID).Error("Failed to render order confirmation email")
		return
	}

	subject := fmt.Sprintf("Confirmation de commande %s - Magic", order.OrderNumber)
	text := fmt.Sprintf(
		"Bonjour %s,\n\nVotre commande %s a bien été enregistrée. Total : %s.\n\nSuivre la commande : %s\n",
		toName, order.OrderNumber, formatMoney(order.TotalPrice, order.Currency), os.orderTrackingURL(order),
	)

	if err := os.mailer.Send(ctx, os.mailFromName, os.mailFromEmail, toName, toEmail, subject, text, html); err != nil {
		os.logger.WithError(err).WithFields(logrus.Fields{
			"order_id":   order.ID,
			"email":      toEmail,
			"from_email": os.mailFromEmail,
		}).Error("Failed to send order confirmation email")
		return
	}

	os.logger.WithFields(logrus.Fields{
		"order_id":     order.ID,
		"order_number": order.OrderNumber,
		"email":        toEmail,
	}).Info("Order confirmation email sent")
}

func (os *OrderService) sendOrderShippedEmail(ctx context.Context, order *interfaces.Order) {
	if order == nil {
		return
	}
	toEmail, toName, err := os.resolveOrderRecipient(ctx, order)
	if err != nil {
		os.logger.WithError(err).WithField("order_id", order.ID).Warn("Cannot send shipped email: no recipient email")
		return
	}
	if os.mailer == nil {
		os.logger.Warn("Mailer not configured, skipping order shipped email")
		return
	}

	data := template.OrderShippedData{
		CustomerName: toName,
		OrderNumber:  order.OrderNumber,
		OrderURL:     os.orderTrackingURL(order),
	}
	if order.TrackingNumber != nil && strings.TrimSpace(*order.TrackingNumber) != "" {
		data.TrackingNumber = strings.TrimSpace(*order.TrackingNumber)
		data.HasTracking = true
	}

	html, err := template.RenderOrderShipped(data)
	if err != nil {
		os.logger.WithError(err).WithField("order_id", order.ID).Error("Failed to render order shipped email")
		return
	}

	subject := fmt.Sprintf("Votre commande %s a été expédiée - Magic", order.OrderNumber)
	text := fmt.Sprintf(
		"Bonjour %s,\n\nVotre commande %s a été expédiée.",
		toName, order.OrderNumber,
	)
	if data.HasTracking {
		text += fmt.Sprintf("\nNuméro de suivi : %s", data.TrackingNumber)
	}
	text += fmt.Sprintf("\n\nVoir la commande : %s\n", data.OrderURL)

	if err := os.mailer.Send(ctx, os.mailFromName, os.mailFromEmail, toName, toEmail, subject, text, html); err != nil {
		os.logger.WithError(err).WithFields(logrus.Fields{
			"order_id":   order.ID,
			"email":      toEmail,
			"from_email": os.mailFromEmail,
		}).Error("Failed to send order shipped email")
		return
	}

	os.logger.WithFields(logrus.Fields{
		"order_id":     order.ID,
		"order_number": order.OrderNumber,
		"email":        toEmail,
	}).Info("Order shipped email sent")
}

func (os *OrderService) resolveOrderRecipient(ctx context.Context, order *interfaces.Order) (email, name string, err error) {
	si := order.ShippingInfo
	name = strings.TrimSpace(si.FirstName + " " + si.LastName)
	if name == "" {
		name = "Client"
	}

	email = strings.TrimSpace(strings.ToLower(si.Email))
	if email != "" {
		return email, name, nil
	}

	if order.UserID != nil && os.userStore != nil {
		user, uerr := os.userStore.Get(ctx, order.UserID.String())
		if uerr == nil && user != nil && strings.TrimSpace(user.Email) != "" {
			return strings.ToLower(strings.TrimSpace(user.Email)), name, nil
		}
	}

	return "", "", fmt.Errorf("no email on order")
}

func (os *OrderService) orderTrackingURL(order *interfaces.Order) string {
	base := strings.TrimRight(os.frontendURL, "/")
	if base == "" {
		base = "http://localhost:3000"
	}
	u := fmt.Sprintf("%s/orders/%s", base, order.ID.String())
	phone := strings.TrimSpace(order.ShippingInfo.Phone)
	if phone != "" {
		return u + "?phone=" + url.QueryEscape(phone)
	}
	return u
}

func (os *OrderService) buildOrderConfirmationData(order *interfaces.Order) template.OrderConfirmationData {
	si := order.ShippingInfo
	name := strings.TrimSpace(si.FirstName + " " + si.LastName)
	if name == "" {
		name = "Client"
	}

	addrParts := []string{si.Address}
	if si.PostalCode != "" {
		addrParts = append(addrParts, si.PostalCode)
	}
	if si.Gouvernorat != "" {
		addrParts = append(addrParts, si.Gouvernorat)
	}

	items := make([]template.OrderEmailItem, 0, len(order.Items))
	for _, it := range order.Items {
		item := template.OrderEmailItem{
			Name:      it.ProductName,
			Quantity:  it.Quantity,
			LineTotal: formatMoney(it.LineTotal, order.Currency),
		}
		if it.Size != "" {
			item.Size = it.Size
			item.HasSize = true
		}
		if it.Color != "" {
			item.Color = it.Color
			item.HasColor = true
		}
		items = append(items, item)
	}

	data := template.OrderConfirmationData{
		CustomerName:    name,
		OrderNumber:     order.OrderNumber,
		OrderDate:       order.CreatedAt.Format("02/01/2006 15:04"),
		OrderURL:        os.orderTrackingURL(order),
		Items:           items,
		Subtotal:        formatMoney(order.Subtotal, order.Currency),
		ShippingFee:     formatMoney(order.ShippingFee, order.Currency),
		TotalPrice:      formatMoney(order.TotalPrice, order.Currency),
		PaymentMethod:   formatPaymentMethod(order.PaymentMethod),
		ShippingAddress: strings.Join(addrParts, ", "),
		ShippingPhone:   si.Phone,
	}
	if order.DiscountAmount > 0 {
		data.HasDiscount = true
		data.DiscountAmount = formatMoney(order.DiscountAmount, order.Currency)
	}
	if order.PromoCode != nil && *order.PromoCode != "" {
		data.HasPromo = true
		data.PromoCode = *order.PromoCode
	}
	return data
}

func formatMoney(amount float64, currency string) string {
	if currency == "" {
		currency = "TND"
	}
	return fmt.Sprintf("%.2f %s", amount, currency)
}

func formatPaymentMethod(method string) string {
	switch method {
	case interfaces.PaymentMethodCash:
		return "Paiement à la livraison (espèces)"
	case interfaces.PaymentMethodCard:
		return "Carte bancaire"
	case interfaces.PaymentMethodD17:
		return "D17"
	case interfaces.PaymentMethodPaymee:
		return "Paymee"
	case interfaces.PaymentMethodKonnect:
		return "Konnect"
	default:
		return method
	}
}
