package appointment

import (
	"context"
	"time"

	"github.com/google/uuid"
)

type Repository interface {
	Create(ctx context.Context, a *Appointment) error
	GetByID(ctx context.Context, id uuid.UUID) (*Appointment, error)
	Update(ctx context.Context, id uuid.UUID, cmd *UpdateAppointmentCommand) (*Appointment, error)
	List(ctx context.Context, q *ListAppointmentsQuery) (*PagedAppointments, error)

	// UpdateStatus updates the status of appointment
	UpdateStatus(ctx context.Context, a *Appointment) error

	// HasConflict checks whether a doctor already has an appointment that overlaps.
	HasConflict(ctx context.Context, doctorID uuid.UUID, start, end time.Time, excludeID *uuid.UUID) (bool, error)

	// GetUpcoming returns appointments for the next N hours — used for reminder jobs.
	GetUpcoming(ctx context.Context, withinHours int) ([]*Appointment, error)
}
