package interfaces

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// Category represents a product category (abayas, caftans, djellabas, accessoires, ...)
type Category struct {
	ID        uuid.UUID  `gorm:"type:uuid;primaryKey;default:gen_random_uuid()" json:"id"`
	CreatedAt time.Time  `gorm:"not null;default:now()" json:"created_at"`
	UpdatedAt time.Time  `gorm:"not null;default:now()" json:"updated_at"`
	DeletedAt *time.Time `gorm:"index" json:"-"`

	Slug        string     `gorm:"type:varchar(100);uniqueIndex;not null" json:"slug"`
	Name        string     `gorm:"type:varchar(150);not null" json:"name"`
	Description string     `gorm:"type:text" json:"description"`
	ImageURL    string     `gorm:"type:text" json:"image_url"`
	ParentID    *uuid.UUID `gorm:"type:uuid" json:"parent_id"`
	Position    int        `gorm:"not null;default:0" json:"position"`
	IsActive    bool       `gorm:"not null;default:true" json:"is_active"`

	Metadata JSONMap `gorm:"type:jsonb" json:"metadata,omitempty"`

	// Children holds direct sub-categories when the tree is loaded (parents only).
	Children []*Category `gorm:"foreignKey:ParentID" json:"children,omitempty"`
}

// TableName overrides default GORM naming
func (Category) TableName() string { return "categories" }

// BeforeCreate hook
func (c *Category) BeforeCreate(tx *gorm.DB) (err error) {
	if c.ID == uuid.Nil {
		c.ID = uuid.New()
	}
	now := time.Now()
	c.CreatedAt = now
	c.UpdatedAt = now
	return
}

// BeforeUpdate hook
func (c *Category) BeforeUpdate(tx *gorm.DB) (err error) {
	c.UpdatedAt = time.Now()
	return
}
