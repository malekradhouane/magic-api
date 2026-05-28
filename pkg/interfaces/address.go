package interfaces

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// Address is a saved shipping address linked to a user profile.
type Address struct {
	ID        uuid.UUID `gorm:"type:uuid;primaryKey;default:gen_random_uuid()" json:"id"`
	CreatedAt time.Time `gorm:"not null;default:now()" json:"createdAt"`
	UpdatedAt time.Time `gorm:"not null;default:now()" json:"updatedAt"`

	UserID uuid.UUID `gorm:"type:uuid;not null;index" json:"userId"`

	Label       string `gorm:"type:varchar(100);not null;default:'Domicile'" json:"label"`
	FirstName   string `gorm:"type:varchar(100);not null" json:"firstName"`
	LastName    string `gorm:"type:varchar(100);not null" json:"lastName"`
	Phone       string `gorm:"type:varchar(50);not null" json:"phone"`
	Gouvernorat string `gorm:"type:varchar(100);not null" json:"gouvernorat"`
	Address     string `gorm:"type:text;not null" json:"address"`
	PostalCode  string `gorm:"type:varchar(20)" json:"postalCode,omitempty"`
	IsDefault   bool   `gorm:"not null;default:false" json:"isDefault"`
}

func (Address) TableName() string { return "addresses" }

func (a *Address) BeforeCreate(tx *gorm.DB) (err error) {
	if a.ID == uuid.Nil {
		a.ID = uuid.New()
	}
	now := time.Now()
	a.CreatedAt = now
	a.UpdatedAt = now
	return
}

func (a *Address) BeforeUpdate(tx *gorm.DB) (err error) {
	a.UpdatedAt = time.Now()
	return
}
