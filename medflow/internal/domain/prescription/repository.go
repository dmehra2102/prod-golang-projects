package prescription

import (
	"context"

	"github.com/google/uuid"
)

type Repository interface {
	Create(ctx context.Context, p *Prescription) error
	GetByID(ctx context.Context, id uuid.UUID) (*Prescription, error)
	UpdateStatus(ctx context.Context, id uuid.UUID, status PrescriptionStatus) error
	Refill(ctx context.Context, id uuid.UUID) (*Prescription, error)
	List(ctx context.Context, q *ListPrescriptionsQuery) (*PagedPrescriptions, error)
	GetActiveByPatient(ctx context.Context, patientID uuid.UUID) ([]*Prescription, error)
}
