package medical_record

import (
	"context"

	"github.com/google/uuid"
)

type Repository interface {
	Create(ctx context.Context, r *MedicalRecord) error
	GetBydID(ctx context.Context, id uuid.UUID) (*MedicalRecord, error)
	AddAddendum(ctx context.Context, a *Addendum) error
	List(ctx context.Context, q *ListRecordsQuery) (*PagedRecords, error)
	GetByAppointmentID(ctx context.Context, appointmentID uuid.UUID) (*MedicalRecord, error)
}
