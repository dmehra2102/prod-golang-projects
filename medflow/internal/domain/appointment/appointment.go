package appointment

import (
	"time"

	"github.com/google/uuid"
)

type AppointmentType string

const (
	TypeConsultation   AppointmentType = "consultation"
	TypeFollowUp       AppointmentType = "follow_up"
	TypeEmergency      AppointmentType = "emergency"
	TypeRoutineCheckup AppointmentType = "routine_checkup"
	TypeProcedure      AppointmentType = "procedure"
	TypeLabResults     AppointmentType = "lab_results"
)

func (t AppointmentType) IsValid() bool {
	switch t {
	case TypeConsultation, TypeFollowUp, TypeEmergency, TypeRoutineCheckup, TypeProcedure, TypeLabResults:
		return true
	}
	return false
}

// State transitions possibilities:
//
//	scheduled → confirmed → in_progress → completed
//	scheduled → cancelled
//	confirmed → cancelled
//	confirmed → no_show (if patient doesn't arrive)
type AppointmentStatus string

const (
	StatusScheduled  AppointmentStatus = "scheduled"
	StatusConfirmed  AppointmentStatus = "confirmed"
	StatusInProgress AppointmentStatus = "in_progress"
	StatusCompleted  AppointmentStatus = "completed"
	StatusCancelled  AppointmentStatus = "cancelled"
	StatusNoShow     AppointmentStatus = "no_show"
)

type Appointment struct {
	ID        uuid.UUID  `gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
	CreatedAt time.Time  `gorm:"autoCreateTime;index"`
	UpdatedAt time.Time  `gorm:"autoUpdateTime"`
	DeletedAt *time.Time `gorm:"index"`

	PatientID uuid.UUID `gorm:"column:patient_id;type:uuid;not null;index"`
	DoctorID  uuid.UUID `gorm:"column:doctor_id;type:uuid;not null;index"`

	ScheduledAt  time.Time         `gorm:"column:scheduled_at;not null;index"`
	DurationMins int               `gorm:"column:duration_mins;not null;default:30"`
	Type         AppointmentType   `gorm:"column:type;type:varchar(50);not null;index"`
	Status       AppointmentStatus `gorm:"column:status;type:varchar(30);not null;default:'scheduled';index"`

	ChiefComplaint string `gorm:"column:chief_complaint;type:text"`
	Notes          string `gorm:"column:notes;type:text"`
	Room           string `gorm:"column:room;type:varchar(50)"`

	// Cancellation tracking
	CancelledAt        *time.Time `gorm:"column:cancelled_at"`
	CancellationReason string     `gorm:"column:cancellation_reason;type:text"`
	CancelledBy        *uuid.UUID `gorm:"column:cancelled_by;type:uuid"`

	CompletedAt        *time.Time `gorm:"column:completed_at"`
	ActualDurationMins *int       `gorm:"column:actual_duration_mins"`

	CreatedBy uuid.UUID `gorm:"column:created_by;type:uuid;not null"`
}

func (Appointment) TableName() string {
	return "clinical.appointments"
}

func (a *Appointment) EndsAt() time.Time {
	return a.ScheduledAt.Add(time.Duration(a.DurationMins) * time.Minute)
}

func (a *Appointment) CanTransitionTo(newStatus AppointmentStatus) bool {
	allowed := map[AppointmentStatus][]AppointmentStatus{
		StatusScheduled:  {StatusConfirmed, StatusCancelled},
		StatusConfirmed:  {StatusInProgress, StatusNoShow, StatusCancelled},
		StatusInProgress: {StatusCompleted},
		StatusCompleted:  {},
		StatusCancelled:  {},
		StatusNoShow:     {},
	}

	for _, s := range allowed[a.Status] {
		if s == newStatus {
			return true
		}
	}
	return false
}

func (a *Appointment) Cancel(reason string, cancelledBy uuid.UUID) error {
	if !a.CanTransitionTo(StatusCancelled) {
		return ErrInvalidStatusTransition
	}
	now := time.Now()
	a.Status = StatusCancelled
	a.CancelledAt = &now
	a.CancellationReason = reason
	a.CancelledBy = &cancelledBy
	return nil
}

func (a *Appointment) Complete(actualDurationMins *int) error {
	if !a.CanTransitionTo(StatusCompleted) {
		return ErrInvalidStatusTransition
	}
	now := time.Now()
	a.Status = StatusCompleted
	a.CompletedAt = &now
	a.ActualDurationMins = actualDurationMins
	return nil
}

type CreateAppointmentCommand struct {
	PatientID      uuid.UUID
	DoctorID       uuid.UUID
	ScheduledAt    time.Time
	DurationMins   int
	Type           AppointmentType
	ChiefComplaint string
	Notes          string
	Room           string
	CreatedBy      uuid.UUID
}

type UpdateAppointmentCommand struct {
	ScheduledAt    *time.Time
	DurationMins   *int
	Type           *AppointmentType
	ChiefComplaint *string
	Notes          *string
	Room           *string
	UpdatedBy      uuid.UUID
}

type CancelAppointmentCommand struct {
	Reason      string
	CancelledBy uuid.UUID
}

type CompleteAppointmentCommand struct {
	ActualDurationMins *int
	CompletedBy        uuid.UUID
}

type ListAppointmentsQuery struct {
	PatientID  *uuid.UUID
	DoctorID   *uuid.UUID
	Status     *AppointmentStatus
	Type       *AppointmentType
	DateFrom   *time.Time
	DateTo     *time.Time
	Page       int
	PageSize   int
}

type PagedAppointments struct {
	Appointments []*Appointment
	TotalCount   int64
	Page         int
	PageSize     int
	TotalPages   int
}