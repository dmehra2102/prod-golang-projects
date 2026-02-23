package domain

import (
	"time"

	"github.com/google/uuid"
)

type Role string

const (
	RoleAdmin        Role = "admin"
	RoleDoctor       Role = "doctor"
	RoleNurse        Role = "nurse"
	RoleReceptionist Role = "receptionist"
	RolePatient      Role = "patient"
)

func (r Role) IsValid() bool {
	switch r {
	case RoleAdmin, RoleDoctor, RoleNurse, RoleReceptionist, RolePatient:
		return true
	}
	return false
}

type User struct {
	ID        uuid.UUID  `gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
	CreatedAt time.Time  `gorm:"autoCreateTime"`
	UpdatedAt time.Time  `gorm:"autoUpdateTime"`
	DeletedAt *time.Time `gorm:"index"`

	Email        string `gorm:"column:email;type:varchar(255);uniqueIndex;not null"`
	PasswordHash string `gorm:"column:password_hash;type:varchar(255);not null"`
	FirstName    string `gorm:"column:first_name;type:varchar(100);not null"`
	LastName     string `gorm:"column:last_name;type:varchar(100);not null"`
	Role         Role   `gorm:"column:role;type:varchar(30);not null;index"`

	// For doctor/nurse roles, links to their staff record
	StaffID *uuid.UUID `gorm:"column:staff_id;type:uuid;index"`
	// For patient role, links to their patient record
	PatientID *uuid.UUID `gorm:"column:patient_id;type:uuid;index"`

	IsActive          bool       `gorm:"column:is_active;default:true;index"`
	FailedLoginCount  int        `gorm:"column:failed_login_count;default:0"`
	LockedUntil       *time.Time `gorm:"column:locked_until"`
	LastLoginAt       *time.Time `gorm:"column:last_login_at"`
	PasswordChangedAt time.Time  `gorm:"column:password_changed_at"`

	MFAEnabled bool   `gorm:"column:mfa_enabled;default:false"`
	MFASecret  string `gorm:"column:mfa_secret;type:varchar(100)"`
}

func (User) TableName() string {
	return "auth.users"
}

// IsLocked returns true if the account is temporarily locked due to failed logins.
func (u *User) IsLocked() bool {
	return u.LockedUntil != nil && time.Now().Before(*u.LockedUntil)
}

type AuditAction string

const (
	ActionCreate AuditAction = "create"
	ActionRead   AuditAction = "read"
	ActionUpdate AuditAction = "update"
	ActionDelete AuditAction = "delete"
	ActionLogin  AuditAction = "login"
	ActionLogout AuditAction = "logout"
)

type AuditLog struct {
	ID         uuid.UUID `gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
	OccurredAt time.Time `gorm:"autoCreateTime;index"`

	// Who
	UserID    uuid.UUID `gorm:"column:user_id;type:uuid;not null;index"`
	UserRole  Role      `gorm:"column:user_role;type:varchar(30);not null"`
	IPAddress string    `gorm:"column:ip_address;type:varchar(45)"` // Supports IPv6

	// What
	Action       AuditAction `gorm:"column:action;type:varchar(20);not null;index"`
	ResourceType string      `gorm:"column:resource_type;type:varchar(50);not null;index"`
	ResourceID   string      `gorm:"column:resource_id;type:varchar(50);index"`

	RequestID  string `gorm:"column:request_id;type:varchar(50);index"`
	UserAgent  string `gorm:"column:user_agent;type:text"`
	StatusCode int    `gorm:"column:status_code"`

	Changes string `gorm:"column:changes;type:jsonb"`
}

func (AuditLog) TableName() string {
	return "audit.logs"
}

type TokenPair struct {
	AccessToken  string    `json:"access_token"`
	RefreshToken string    `json:"refresh_token"`
	ExpiresAt    time.Time `json:"expires_at"`
	TokenType    string    `json:"token_type"` // Always "Bearer"
}

type Claims struct {
	UserID    uuid.UUID  `json:"sub"`
	Email     string     `json:"email"`
	Role      Role       `json:"role"`
	StaffID   *uuid.UUID `json:"staff_id,omitempty"`
	PatientID *uuid.UUID `json:"patient_id,omitempty"`
}
