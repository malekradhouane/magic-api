package store

import (
	"fmt"
	"sync"

	"github.com/malekradhouane/magic/store/postgres"
	"github.com/malekradhouane/magic/store/types"
)

type Options struct{}

var (
	createPostgresStoresOnce sync.Once
	stores                   StoreSet
)

// StoreSet holds instances of the concrete type implementing the data stores
type StoreSet struct {
	User     types.UserStore
	Category types.CategoryStore
	Product  types.ProductStore
	Order    types.OrderStore
	Promo    types.PromoStore
	Address  types.AddressStore
	Stats    types.StatsStore
	Settings types.SettingsStore
	Consent  types.ConsentStore
}

// CreatePostgresStores initializes all stores backed by PostgreSQL
func CreatePostgresStores(opts *Options) error {
	var err error

	createPostgresStoresOnce.Do(func() {
		if opts == nil {
			opts = &Options{}
		}

		if stores.User, err = postgres.NewUserStore(); err != nil {
			err = fmt.Errorf("CreateStores: NewUserStore err: %w", err)
			return
		}
		if stores.Category, err = postgres.NewCategoryStore(); err != nil {
			err = fmt.Errorf("CreateStores: NewCategoryStore err: %w", err)
			return
		}
		if stores.Product, err = postgres.NewProductStore(); err != nil {
			err = fmt.Errorf("CreateStores: NewProductStore err: %w", err)
			return
		}
		if stores.Order, err = postgres.NewOrderStore(); err != nil {
			err = fmt.Errorf("CreateStores: NewOrderStore err: %w", err)
			return
		}
		if stores.Promo, err = postgres.NewPromoStore(); err != nil {
			err = fmt.Errorf("CreateStores: NewPromoStore err: %w", err)
			return
		}
		if stores.Address, err = postgres.NewAddressStore(); err != nil {
			err = fmt.Errorf("CreateStores: NewAddressStore err: %w", err)
			return
		}
		if stores.Stats, err = postgres.NewStatsStore(); err != nil {
			err = fmt.Errorf("CreateStores: NewStatsStore err: %w", err)
			return
		}
		if stores.Settings, err = postgres.NewSettingsStore(); err != nil {
			err = fmt.Errorf("CreateStores: NewSettingsStore err: %w", err)
			return
		}
		if stores.Consent, err = postgres.NewConsentStore(); err != nil {
			err = fmt.Errorf("CreateStores: NewConsentStore err: %w", err)
			return
		}
	})

	return err
}

// Users returns the user store
func Users() types.UserStore { return stores.User }

// Categories returns the category store
func Categories() types.CategoryStore { return stores.Category }

// Products returns the product store
func Products() types.ProductStore { return stores.Product }

// Orders returns the order store
func Orders() types.OrderStore { return stores.Order }

// Promos returns the promo store
func Promos() types.PromoStore { return stores.Promo }

// Addresses returns the address store
func Addresses() types.AddressStore { return stores.Address }

// Stats returns the stats (dashboard aggregations) store
func Stats() types.StatsStore { return stores.Stats }

// Settings returns the application settings store
func Settings() types.SettingsStore { return stores.Settings }

// Consents returns the consent proof store
func Consents() types.ConsentStore { return stores.Consent }
