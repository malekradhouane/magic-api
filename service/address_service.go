package service

import (
	"context"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/sirupsen/logrus"

	"github.com/malekradhouane/magic/api"
	"github.com/malekradhouane/magic/errs"
	"github.com/malekradhouane/magic/pkg/interfaces"
	"github.com/malekradhouane/magic/store/types"
)

// AddressService handles user address business logic.
type AddressService struct {
	store  types.AddressStore
	logger *logrus.Logger
}

// NewAddressService constructs an AddressService.
func NewAddressService(store types.AddressStore, logger *logrus.Logger) *AddressService {
	if logger == nil {
		logger = logrus.New()
	}
	return &AddressService{store: store, logger: logger}
}

// ListByUser returns addresses for the authenticated user only.
func (s *AddressService) ListByUser(ctx context.Context, userID string) ([]*interfaces.Address, error) {
	if userID == "" {
		return nil, fmt.Errorf("user id is required")
	}
	return s.store.ListByUserID(ctx, userID)
}

// Create adds a new address for the user.
func (s *AddressService) Create(ctx context.Context, userID string, req *api.CreateAddressRequest) (*interfaces.Address, error) {
	uid, err := uuid.Parse(userID)
	if err != nil {
		return nil, fmt.Errorf("invalid user id")
	}
	if err := validateAddressRequest(req); err != nil {
		return nil, err
	}

	label := strings.TrimSpace(req.Label)
	if label == "" {
		label = "Domicile"
	}

	addr := &interfaces.Address{
		UserID:      uid,
		Label:       label,
		FirstName:   strings.TrimSpace(req.FirstName),
		LastName:    strings.TrimSpace(req.LastName),
		Phone:       normalizePhone(req.Phone),
		Gouvernorat: strings.TrimSpace(req.Gouvernorat),
		Address:     strings.TrimSpace(req.Address),
		PostalCode:  strings.TrimSpace(req.PostalCode),
		IsDefault:   req.IsDefault,
	}
	return s.store.Create(ctx, addr)
}

// Update modifies an address owned by the user.
func (s *AddressService) Update(ctx context.Context, userID, id string, req *api.UpdateAddressRequest) (*interfaces.Address, error) {
	existing, err := s.store.GetByID(ctx, userID, id)
	if err != nil {
		if errs.IsNoSuchEntityError(err) {
			return nil, errs.ErrNoSuchEntity
		}
		return nil, err
	}

	if req.Label != nil {
		existing.Label = strings.TrimSpace(*req.Label)
	}
	if req.FirstName != nil {
		existing.FirstName = strings.TrimSpace(*req.FirstName)
	}
	if req.LastName != nil {
		existing.LastName = strings.TrimSpace(*req.LastName)
	}
	if req.Phone != nil {
		existing.Phone = normalizePhone(*req.Phone)
	}
	if req.Gouvernorat != nil {
		existing.Gouvernorat = strings.TrimSpace(*req.Gouvernorat)
	}
	if req.Address != nil {
		existing.Address = strings.TrimSpace(*req.Address)
	}
	if req.PostalCode != nil {
		existing.PostalCode = strings.TrimSpace(*req.PostalCode)
	}
	if req.IsDefault != nil {
		existing.IsDefault = *req.IsDefault
	}

	if existing.Label == "" {
		existing.Label = "Domicile"
	}
	if existing.FirstName == "" || existing.LastName == "" || existing.Phone == "" ||
		existing.Gouvernorat == "" || existing.Address == "" {
		return nil, fmt.Errorf("address fields cannot be empty")
	}

	return s.store.Update(ctx, existing)
}

// Delete removes an address owned by the user.
func (s *AddressService) Delete(ctx context.Context, userID, id string) error {
	err := s.store.Delete(ctx, userID, id)
	if errs.IsNoSuchEntityError(err) {
		return errs.ErrNoSuchEntity
	}
	return err
}

// SetDefault marks an address as default for the user.
func (s *AddressService) SetDefault(ctx context.Context, userID, id string) error {
	err := s.store.SetDefault(ctx, userID, id)
	if errs.IsNoSuchEntityError(err) {
		return errs.ErrNoSuchEntity
	}
	return err
}

// SaveFromShipping persists the shipping address used at checkout for authenticated users.
func (s *AddressService) SaveFromShipping(ctx context.Context, userID string, shipping interfaces.ShippingInfo) error {
	if userID == "" {
		return nil
	}
	uid, err := uuid.Parse(userID)
	if err != nil {
		return nil
	}

	phone := normalizePhone(shipping.Phone)
	gouvernorat := strings.TrimSpace(shipping.Gouvernorat)
	addressLine := strings.TrimSpace(shipping.Address)
	if phone == "" || gouvernorat == "" || addressLine == "" {
		return nil
	}

	existing, err := s.store.FindMatching(ctx, userID, phone, gouvernorat, addressLine)
	if err == nil && existing != nil {
		existing.FirstName = strings.TrimSpace(shipping.FirstName)
		existing.LastName = strings.TrimSpace(shipping.LastName)
		if shipping.PostalCode != "" {
			existing.PostalCode = strings.TrimSpace(shipping.PostalCode)
		}
		_, err = s.store.Update(ctx, existing)
		return err
	}

	label := "Livraison"
	_, err = s.store.Create(ctx, &interfaces.Address{
		UserID:      uid,
		Label:       label,
		FirstName:   strings.TrimSpace(shipping.FirstName),
		LastName:    strings.TrimSpace(shipping.LastName),
		Phone:       phone,
		Gouvernorat: gouvernorat,
		Address:     addressLine,
		PostalCode:  strings.TrimSpace(shipping.PostalCode),
		IsDefault:   false,
	})
	return err
}

func validateAddressRequest(req *api.CreateAddressRequest) error {
	if req == nil {
		return fmt.Errorf("request is required")
	}
	if strings.TrimSpace(req.FirstName) == "" ||
		strings.TrimSpace(req.LastName) == "" ||
		strings.TrimSpace(req.Phone) == "" ||
		strings.TrimSpace(req.Gouvernorat) == "" ||
		strings.TrimSpace(req.Address) == "" {
		return fmt.Errorf("all address fields are required")
	}
	return nil
}

func normalizePhone(phone string) string {
	return strings.ReplaceAll(strings.TrimSpace(phone), " ", "")
}
