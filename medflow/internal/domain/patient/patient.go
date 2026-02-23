package patient

import (
	"strings"
	"time"

	"github.com/google/uuid"
)

type Gender string

const (
	GenderMale    Gender = "male"
	GenderFemale  Gender = "female"
	GenderOther   Gender = "other"
	GenderUnknown Gender = "unknown"
)

func (g Gender) IsValid() bool {
	switch g {
	case GenderMale, GenderFemale, GenderOther, GenderUnknown:
		return true
	}
	return false
}

type BloodType string

const (
	BloodTypeAPos    BloodType = "A+"
	BloodTypeANeg    BloodType = "A-"
	BloodTypeBPos    BloodType = "B+"
	BloodTypeBNeg    BloodType = "B-"
	BloodTypeABPos   BloodType = "AB+"
	BloodTypeABNeg   BloodType = "AB-"
	BloodTypeOPos    BloodType = "O+"
	BloodTypeONeg    BloodType = "O-"
	BloodTypeUnknown BloodType = "unknown"
)

// Status represents the lifecycle state of a patient record.
type Status string

const (
	StatusActive   Status = "active"
	StatusInactive Status = "inactive"
	StatusDeceased Status = "deceased"
)

type ContactInfo struct {
	Phone   string `gorm:"column:phone;type:varchar(20)"`
	Email   string `gorm:"column:email;type:varchar(255)"`
	Address string `gorm:"column:address;type:text"`
	City    string `gorm:"column:city;type:varchar(100)"`
	State   string `gorm:"column:state;type:varchar(50)"`
	ZipCode string `gorm:"column:zip_code;type:varchar(20)"`
	Country string `gorm:"column:country;type:varchar(100)"`
}

type EmergencyContact struct {
	Name         string `json:"name"`
	Relationship string `json:"relationship"`
	Phone        string `json:"phone"`
}

type Insurance struct {
	Provider      string `json:"provider"`
	PolicyNumber  string `json:"policy_number"`
	GroupNumber   string `json:"group_number"`
	PrimaryHolder string `json:"primary_holder"`
}

type Patient struct {
	ID        uuid.UUID  `gorm:"type:uuid;primaryKey:default:gen_random_uuid()"`
	CreatedAt time.Time  `gorm:"autoCreateTime;index"`
	UpdatedAt time.Time  `gorm:"autoUpdateTime"`
	DeletedAt *time.Time `gorm:"index"` // Soft Delete

	FirstName   string    `gorm:"column:first_name;type:varchar(100);not null"`
	LastName    string    `gorm:"column:last_name;type:varchar(100);not null"`
	DateOfBirth time.Time `gorm:"column:date_of_birth;not null"`
	Gender      Gender    `gorm:"column:gender;type:varchar(20);not null"`
	BloodType   BloodType `gorm:"column:blood_type;type:varchar(5)"`
	NationalID  string    `gorm:"column:national_id;type:varchar(50);uniqueIndex"`

	ContactInfo

	EmergencyContact *EmergencyContact `gorm:"column:emergency_contact;serializer:json"`
	Insurance        *Insurance        `gorm:"column:insurance;serializer:json"`

	Allergies         []string `gorm:"column:allergies;serializer:json"`
	ChronicConditions []string `gorm:"column:chronic_conditions;serializer:json"`

	Status           Status     `gorm:"column:status;type:varchar(20);default:'active';index"`
	AssignedDoctorID *uuid.UUID `gorm:"column:assigned_doctor_id;type:uuid;index"`
	Notes            string     `gorm:"column:notes;type:text"` // PHI

	// Audit: who registered this patient and when
	CreatedBy uuid.UUID `gorm:"column:created_by;type:uuid;not null"`
}

func (Patient) TableName() string {
	return "clinical.patients"
}

func (p *Patient) FullName() string {
	return strings.TrimSpace(p.FirstName + " " + p.LastName)
}

func (p *Patient) Age() int {
	now := time.Now()
	years := now.Year() - p.DateOfBirth.Year()
	if now.Month() < p.DateOfBirth.Month() ||
		(now.Month() == p.DateOfBirth.Month() && now.Day() < p.DateOfBirth.Day()) {
		years--
	}
	return years
}

func (p *Patient) IsActive() bool {
	return p.Status == StatusActive && p.DeletedAt == nil
}

func (p *Patient) Deactivate() error {
	if p.Status == StatusDeceased {
		return ErrPatientDeceased
	}
	p.Status = StatusInactive
	return nil
}

func (p *Patient) MarkDeceased() {
	p.Status = StatusDeceased
}

type CreatePatientCommand struct {
	FirstName         string
	LastName          string
	DateOfBirth       time.Time
	Gender            Gender
	BloodType         BloodType
	NationalID        string
	Phone             string
	Email             string
	Address           string
	City              string
	State             string
	ZipCode           string
	Country           string
	EmergencyContact  *EmergencyContact
	Insurance         *Insurance
	Allergies         []string
	ChronicConditions []string
	AssignedDoctorID  *uuid.UUID
	Notes             string
	CreatedBy         uuid.UUID
}

type UpdatePatientCommand struct {
	FirstName         *string
	LastName          *string
	Gender            *Gender
	BloodType         *BloodType
	Phone             *string
	Email             *string
	Address           *string
	City              *string
	State             *string
	ZipCode           *string
	Country           *string
	EmergencyContact  *EmergencyContact
	Insurance         *Insurance
	Allergies         *[]string
	ChronicConditions *[]string
	AssignedDoctorID  *uuid.UUID
	Notes             *string
	UpdatedBy         uuid.UUID
}

// ListPatientsQuery defines filtering and pagination for patient list queries.
type ListPatientsQuery struct {
	Search           string // Full-text search on name
	Status           *Status
	AssignedDoctorID *uuid.UUID
	Page             int
	PageSize         int
	SortBy           string
	SortOrder        string // "asc" | "desc"
}

type PagedPatients struct {
	Patients   []*Patient
	TotalCount int64
	Page       int
	PageSize   int
	TotalPages int
}
