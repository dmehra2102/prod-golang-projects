package patient

import (
	"context"

	"github.com/google/uuid"
)

type Repository interface {
	// Create persists a new patient. Returns ErrPatientAlreadyExists on duplicate NationalID.
	Create(ctx context.Context, p *Patient) error

	// GetByID retrieves a patient by primary key. Returns ErrPatientNotFound if not found.
	GetByID(ctx context.Context, id uuid.UUID) (*Patient, error)

	// GetByNationalID retrieves a patient by their national identifier.
	GetByNationalID(ctx context.Context, nationalID string) (*Patient, error)

	// Update applies partial updates to an existing patient record.
	Update(ctx context.Context, id uuid.UUID, cmd *UpdatePatientCommand) (*Patient, error)

	// SoftDelete marks the patient as deleted (HIPAA retention requirement).
	SoftDelete(ctx context.Context, id uuid.UUID) error

	// List returns a paginated, filtered list of patients.
	List(ctx context.Context, q *ListPatientsQuery) (*PagedPatients, error)

	// ExistsByNationalID checks for uniqueness without fetching the full record.
	ExistsByNationalID(ctx context.Context, nationalID string, excludeID *uuid.UUID) (bool, error)
}
