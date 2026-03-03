package repository

import (
	"context"
	"errors"
	"math"

	"github.com/dmehra2102/prod-golang-projects/medflow/internal/domain/patient"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

type PatientRepository struct {
	db *gorm.DB
}

func NewPatientRepository(db *gorm.DB) patient.Repository {
	return &PatientRepository{db: db}
}

func (r *PatientRepository) Create(ctx context.Context, p *patient.Patient) error {
	result := r.db.WithContext(ctx).Create(p)
	if result.Error != nil {
		if isDuplicateKeyError(result.Error) {
			return patient.ErrPatientAlreadyExists
		}
		return result.Error
	}
	return nil
}

func (r *PatientRepository) GetByID(ctx context.Context, id uuid.UUID) (*patient.Patient, error) {
	var p patient.Patient
	result := r.db.WithContext(ctx).Where("id = ? AND deleted_at IS NULL", id).First(&p)

	if result.Error != nil {
		if errors.Is(result.Error, gorm.ErrRecordNotFound) {
			return nil, patient.ErrPatientNotFound
		}
		return nil, result.Error
	}
	return &p, nil
}

func (r *PatientRepository) GetByNationalID(ctx context.Context, nationalID string) (*patient.Patient, error) {
	var p patient.Patient
	result := r.db.WithContext(ctx).
		Where("national_id = ? AND deleted_at IS NULL", nationalID).
		First(&p)

	if result.Error != nil {
		if errors.Is(result.Error, gorm.ErrRecordNotFound) {
			return nil, patient.ErrPatientNotFound
		}
		return nil, result.Error
	}
	return &p, nil
}

func (r *PatientRepository) Update(ctx context.Context, id uuid.UUID, cmd *patient.UpdatePatientCommand) (*patient.Patient, error) {
	updates := buildPatientUpdates(cmd)
	if len(updates) == 0 {
		return r.GetByID(ctx, id)
	}

	result := r.db.WithContext(ctx).Model(&patient.Patient{}).
		Where("id = ? AND deleted_at IS NULL", id).
		Updates(updates)

	if result.Error != nil {
		return nil, result.Error
	}
	if result.RowsAffected == 0 {
		return nil, patient.ErrPatientNotFound
	}

	return r.GetByID(ctx, id)
}

func (r *PatientRepository) SoftDelete(ctx context.Context, id uuid.UUID) error {
	result := r.db.WithContext(ctx).
		Model(&patient.Patient{}).
		Where("id = ? AND deleted_at IS NULL", id).
		Update("deleted_at", gorm.Expr("NOW()"))

	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return patient.ErrPatientNotFound
	}
	return nil
}

func (r *PatientRepository) List(ctx context.Context, q *patient.ListPatientsQuery) (*patient.PagedPatients, error) {
	query := r.db.WithContext(ctx).Model(&patient.Patient{}).Where("delete_at IS NULL")

	if q.Search != "" {
		search := "%" + q.Search + "%"
		query = query.Where("(first_name ILIKE ? OR last_name ILIKE ? OR national_id ILIKE ?)",
			search, search, search)
	}
	if q.Status != nil {
		query = query.Where("status = ?", *q.Status)
	}
	if q.AssignedDoctorID != nil {
		query = query.Where("assigned_doctor_id = ?", *q.AssignedDoctorID)
	}

	// Count total before pagination
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, err
	}

	// Apply sorting with whitelist to prevent SQL injection
	sortColumn := sanitizeSortColumn(q.SortBy, []string{"last_name", "first_name", "created_at", "date_of_birth"}, "last_name")
	sortDir := "ASC"
	if q.SortOrder == "desc" {
		sortDir = "DESC"
	}
	query = query.Order(sortColumn + " " + sortDir)

	// Apply pagination
	offset := (q.Page - 1) * q.PageSize
	query = query.Offset(offset).Limit(q.PageSize)

	var patients []*patient.Patient
	if err := query.Find(&patients).Error; err != nil {
		return nil, err
	}

	totalPages := int(math.Ceil(float64(total) / float64(q.PageSize)))

	return &patient.PagedPatients{
		Patients:   patients,
		TotalCount: total,
		Page:       q.Page,
		PageSize:   q.PageSize,
		TotalPages: totalPages,
	}, nil
}

func (r *PatientRepository) ExistsByNationalID(ctx context.Context, nationalID string, excludeID *uuid.UUID) (bool, error) {
	query := r.db.WithContext(ctx).Model(&patient.Patient{}).
		Where("national_id = ? AND deleted_at IS NULL", nationalID)

	if excludeID != nil {
		query = query.Where("id != ?", *excludeID)
	}

	var count int64
	if err := query.Count(&count).Error; err != nil {
		return false, err
	}
	return count > 0, nil
}

func buildPatientUpdates(cmd *patient.UpdatePatientCommand) map[string]any {
	updates := make(map[string]any)
	if cmd.FirstName != nil {
		updates["first_name"] = *cmd.FirstName
	}
	if cmd.LastName != nil {
		updates["last_name"] = *cmd.LastName
	}
	if cmd.Gender != nil {
		updates["gender"] = *cmd.Gender
	}
	if cmd.BloodType != nil {
		updates["blood_type"] = *cmd.BloodType
	}
	if cmd.Phone != nil {
		updates["phone"] = *cmd.Phone
	}
	if cmd.Email != nil {
		updates["email"] = *cmd.Email
	}
	if cmd.Address != nil {
		updates["address"] = *cmd.Address
	}
	if cmd.City != nil {
		updates["city"] = *cmd.City
	}
	if cmd.State != nil {
		updates["state"] = *cmd.State
	}
	if cmd.ZipCode != nil {
		updates["zip_code"] = *cmd.ZipCode
	}
	if cmd.Country != nil {
		updates["country"] = *cmd.Country
	}
	if cmd.EmergencyContact != nil {
		updates["emergency_contact"] = cmd.EmergencyContact
	}
	if cmd.Insurance != nil {
		updates["insurance"] = cmd.Insurance
	}
	if cmd.Allergies != nil {
		updates["allergies"] = *cmd.Allergies
	}
	if cmd.ChronicConditions != nil {
		updates["chronic_conditions"] = *cmd.ChronicConditions
	}
	if cmd.Notes != nil {
		updates["notes"] = *cmd.Notes
	}
	return updates
}

func sanitizeSortColumn(input string, allowed []string, defaultCol string) string {
	for _, col := range allowed {
		if col == input {
			return col
		}
	}
	return defaultCol
}

func isDuplicateKeyError(err error) bool {
	if err == nil {
		return false
	}

	return contains(err.Error(), "unique constraint") ||
		contains(err.Error(), "duplicate key") ||
		contains(err.Error(), "23505") // PostgreSQL error code
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsHelper(s, substr))
}

func containsHelper(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
