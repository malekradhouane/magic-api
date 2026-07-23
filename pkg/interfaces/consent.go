package interfaces

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// Consent type discriminators.
const (
	ConsentTypeContact    = "contact"
	ConsentTypeNewsletter = "newsletter"
)

// Consent is a stored proof of a user's consent. It captures the date, the exact
// consent text shown, the source form, and request metadata (IP, user agent) so
// the consent can be evidenced when a complaint is raised.
type Consent struct {
	ID        uuid.UUID `gorm:"type:uuid;primaryKey;default:gen_random_uuid()" json:"id"`
	CreatedAt time.Time `gorm:"not null;default:now()" json:"createdAt"`

	Type   string `gorm:"type:varchar(32);not null" json:"type"`
	Source string `gorm:"type:varchar(100);not null" json:"source"`

	Email   string `gorm:"type:varchar(320);not null;index" json:"email"`
	Name    string `gorm:"type:varchar(200)" json:"name,omitempty"`
	Subject string `gorm:"type:varchar(300)" json:"subject,omitempty"`
	Message string `gorm:"type:text" json:"message,omitempty"`

	Consent     bool   `gorm:"not null;default:false" json:"consent"`
	ConsentText string `gorm:"type:text;not null" json:"consentText"`

	IPAddress string `gorm:"type:varchar(64)" json:"ipAddress,omitempty"`
	UserAgent string `gorm:"type:text" json:"userAgent,omitempty"`
}

func (Consent) TableName() string { return "consents" }

func (c *Consent) BeforeCreate(tx *gorm.DB) (err error) {
	if c.ID == uuid.Nil {
		c.ID = uuid.New()
	}
	if c.CreatedAt.IsZero() {
		c.CreatedAt = time.Now()
	}
	return
}
