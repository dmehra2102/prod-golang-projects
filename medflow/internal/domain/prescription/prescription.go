package prescription

import (
	"time"

	"github.com/google/uuid"
)

type PrescriptionStatus string

const (
	StatusActive    PrescriptionStatus = "active"
	StatusDispensed PrescriptionStatus = "dispensed"
	StatusExpired   PrescriptionStatus = "expired"
	StatusCancelled PrescriptionStatus = "cancelled"
	StatusOnHold    PrescriptionStatus = "on_hold"
)

type RouteOfAdministration string

const (
	RouteOral          RouteOfAdministration = "oral"
	RouteIntravenous   RouteOfAdministration = "intravenous"
	RouteIntramuscular RouteOfAdministration = "intramuscular"
	RouteTopical       RouteOfAdministration = "topical"
	RouteInhaled       RouteOfAdministration = "inhaled"
	RouteSublingual    RouteOfAdministration = "sublingual"
)

type Prescription struct {
	ID        uuid.UUID  `gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
	CreatedAt time.Time  `gorm:"autoCreateTime;index"`
	UpdatedAt time.Time  `gorm:"autoUpdateTime"`
	DeletedAt *time.Time `gorm:"index"`

	PatientID     uuid.UUID  `gorm:"column:patient_id;type:uuid;not null;index"`
	DoctorID      uuid.UUID  `gorm:"column:doctor_id;type:uuid;not null;index"`
	AppointmentID *uuid.UUID `gorm:"column:appointment_id;type:uuid;index"`

	MedicationName  string                `gorm:"column:medication_name;type:varchar(255);not null;index"`
	GenericName     string                `gorm:"column:generic_name;type:varchar(255)"`
	DosageAmount    string                `gorm:"column:dosage_amount;type:varchar(50);not null"`     // e.g. "500mg"
	DosageFrequency string                `gorm:"column:dosage_frequency;type:varchar(100);not null"` // e.g. "twice daily"
	Route           RouteOfAdministration `gorm:"column:route;type:varchar(50);not null"`
	Duration        string                `gorm:"column:duration;type:varchar(100)"` // e.g. "7 days"
	Quantity        int                   `gorm:"column:quantity;not null"`
	RefillsAllowed  int                   `gorm:"column:refills_allowed;default:0"`
	RefillsUsed     int                   `gorm:"column:refills_used;default:0"`

	IsControlledSubstance bool `gorm:"column:is_controlled_substance;default:false;index"`
	DEASchedule           *int `gorm:"column:dea_schedule"`

	IssuedAt  time.Time `gorm:"column:issued_at;not null;index"`
	ExpiresAt time.Time `gorm:"column:expires_at;not null;index"`

	Status PrescriptionStatus `gorm:"column:status;type:varchar(30);not null;default:'active';index"`

	Instructions string   `gorm:"column:instructions;type:text"`
	Warnings     []string `gorm:"column:warnings;serializer:json"`

	CreatedBy uuid.UUID `gorm:"column:created_by;type:uuid;not null"`
}

func (Prescription) TableName() string {
	return "clinical.prescriptions"
}

// IsRefillable checks if a prescription can be refilled.
func (p *Prescription) IsRefillable() bool {
	return p.Status == StatusActive &&
		p.RefillsUsed < p.RefillsAllowed &&
		time.Now().Before(p.ExpiresAt)
}

// Refill increments the refill count.
func (p *Prescription) Refill() error {
	if !p.IsRefillable() {
		return ErrNotRefillable
	}
	p.RefillsUsed++
	if p.RefillsUsed >= p.RefillsAllowed {
		p.Status = StatusDispensed
	}
	return nil
}

func (p *Prescription) IsExpired() bool {
	return time.Now().After(p.ExpiresAt)
}

type CreatePrescriptionCommand struct {
	PatientID             uuid.UUID
	DoctorID              uuid.UUID
	AppointmentID         *uuid.UUID
	MedicationName        string
	GenericName           string
	DosageAmount          string
	DosageFrequency       string
	Route                 RouteOfAdministration
	Duration              string
	Quantity              int
	RefillsAllowed        int
	IsControlledSubstance bool
	DEASchedule           *int
	IssuedAt              time.Time
	ExpiresAt             time.Time
	Instructions          string
	Warnings              []string
	CreatedBy             uuid.UUID
}

type ListPrescriptionsQuery struct {
	PatientID             *uuid.UUID
	DoctorID              *uuid.UUID
	Status                *PrescriptionStatus
	IsControlledSubstance *bool
	Page                  int
	PageSize              int
}

type PagedPrescriptions struct {
	Prescriptions []*Prescription
	TotalCount    int64
	Page          int
	PageSize      int
	TotalPages    int
}
