package repository

import (
	"context"
	"errors"
	"math"
	"time"

	mr "github.com/dmehra2102/prod-golang-projects/medflow/internal/domain/medical_record"
	"github.com/dmehra2102/prod-golang-projects/medflow/internal/domain/prescription"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

type MedicalRecordRepository struct {
	db *gorm.DB
}

func NewMedicalRecordRepository(db *gorm.DB) mr.Repository {
	return &MedicalRecordRepository{db: db}
}

func (r *MedicalRecordRepository) Create(ctx context.Context, record *mr.MedicalRecord) error {
	return r.db.WithContext(ctx).Create(record).Error
}

func (r *MedicalRecordRepository) GetBydID(ctx context.Context, id uuid.UUID) (*mr.MedicalRecord, error) {
	var record mr.MedicalRecord
	result := r.db.WithContext(ctx).
		Preload("Addenda").
		Where("id = ?", id).
		First(&record)

	if result.Error != nil {
		if errors.Is(result.Error, gorm.ErrRecordNotFound) {
			return nil, mr.ErrRecordNotFound
		}
		return nil, result.Error
	}
	return &record, nil
}

func (r *MedicalRecordRepository) AddAddendum(ctx context.Context, a *mr.Addendum) error {
	// Verify parent record exists
	var count int64
	if err := r.db.WithContext(ctx).Model(&mr.MedicalRecord{}).
		Where("id = ?", a.MedicalRecordID).Count(&count).Error; err != nil {
		return err
	}
	if count == 0 {
		return mr.ErrRecordNotFound
	}
	return r.db.WithContext(ctx).Create(a).Error
}

func (r *MedicalRecordRepository) List(ctx context.Context, q *mr.ListRecordsQuery) (*mr.PagedRecords, error) {
	query := r.db.WithContext(ctx).Model(&mr.MedicalRecord{})

	if q.PatientID != nil {
		query = query.Where("patient_id = ?", *q.PatientID)
	}
	if q.DoctorID != nil {
		query = query.Where("doctor_id = ?", *q.DoctorID)
	}
	if q.Type != nil {
		query = query.Where("type = ?", *q.Type)
	}
	if q.AppointmentID != nil {
		query = query.Where("appointment_id = ?", *q.AppointmentID)
	}
	if q.DateFrom != nil {
		query = query.Where("created_at >= ?", *q.DateFrom)
	}
	if q.DateTo != nil {
		query = query.Where("created_at <= ?", *q.DateTo)
	}

	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, err
	}

	offset := (q.Page - 1) * q.PageSize
	var records []*mr.MedicalRecord
	if err := query.Preload("Addenda").Order("created_at DESC").Offset(offset).Limit(q.PageSize).Find(&records).Error; err != nil {
		return nil, err
	}

	return &mr.PagedRecords{
		Records:    records,
		TotalCount: total,
		Page:       q.Page,
		PageSize:   q.PageSize,
		TotalPages: int(math.Ceil(float64(total) / float64(q.PageSize))),
	}, nil
}

func (r *MedicalRecordRepository) GetByAppointmentID(ctx context.Context, appointmentID uuid.UUID) (*mr.MedicalRecord, error) {
	var record mr.MedicalRecord
	result := r.db.WithContext(ctx).Where("appointment_id = ?", appointmentID).First(&record)
	if result.Error != nil {
		if errors.Is(result.Error, gorm.ErrRecordNotFound) {
			return nil, mr.ErrRecordNotFound
		}
		return nil, result.Error
	}
	return &record, nil
}

// -------- Prescription Repository -------------------------------
type PrescriptionRepository struct {
	db *gorm.DB
}

func NewPrescriptionRepository(db *gorm.DB) prescription.Repository {
	return &PrescriptionRepository{db: db}
}

func (r *PrescriptionRepository) Create(ctx context.Context, p *prescription.Prescription) error {
	return r.db.WithContext(ctx).Create(p).Error
}

func (r *PrescriptionRepository) GetByID(ctx context.Context, id uuid.UUID) (*prescription.Prescription, error) {
	var p prescription.Prescription
	result := r.db.WithContext(ctx).Where("id = ? AND deleted_at IS NULL", id).First(&p)
	if result.Error != nil {
		if errors.Is(result.Error, gorm.ErrRecordNotFound) {
			return nil, prescription.ErrPrescriptionNotFound
		}
		return nil, result.Error
	}
	return &p, nil
}

func (r *PrescriptionRepository) UpdateStatus(ctx context.Context, id uuid.UUID, status prescription.PrescriptionStatus) error {
	return r.db.WithContext(ctx).Model(&prescription.Prescription{}).
		Where("id = ?", id).
		Update("status", status).Error
}

func (r *PrescriptionRepository) Refill(ctx context.Context, id uuid.UUID) (*prescription.Prescription, error) {
	return r.GetByID(ctx, id)
}

func (r *PrescriptionRepository) List(ctx context.Context, q *prescription.ListPrescriptionsQuery) (*prescription.PagedPrescriptions, error) {
	query := r.db.WithContext(ctx).Model(&prescription.Prescription{}).Where("deleted_at IS NULL")

	if q.PatientID != nil {
		query = query.Where("patient_id = ?", *q.PatientID)
	}
	if q.DoctorID != nil {
		query = query.Where("doctor_id = ?", *q.DoctorID)
	}
	if q.Status != nil {
		query = query.Where("status = ?", *q.Status)
	}
	if q.IsControlledSubstance != nil {
		query = query.Where("is_controlled_substance = ?", *q.IsControlledSubstance)
	}

	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, err
	}

	offset := (q.Page - 1) * q.PageSize
	var prescriptions []*prescription.Prescription
	if err := query.Order("issued_at DESC").Offset(offset).Limit(q.PageSize).Find(&prescriptions).Error; err != nil {
		return nil, err
	}

	return &prescription.PagedPrescriptions{
		Prescriptions: prescriptions,
		TotalCount:    total,
		Page:          q.Page,
		PageSize:      q.PageSize,
		TotalPages:    int(math.Ceil(float64(total) / float64(q.PageSize))),
	}, nil
}

func (r *PrescriptionRepository) GetActiveByPatient(ctx context.Context, patientID uuid.UUID) ([]*prescription.Prescription, error) {
	var prescriptions []*prescription.Prescription
	err := r.db.WithContext(ctx).
		Where("patient_id = ? AND status = ? AND expires_at > ? AND deleted_at IS NULL",
			patientID, prescription.StatusActive, time.Now()).
		Find(&prescriptions).Error
	return prescriptions, err
}
