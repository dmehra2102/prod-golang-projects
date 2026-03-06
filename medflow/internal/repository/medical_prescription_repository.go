package repository

import (
	"context"
	"errors"
	"math"

	mr "github.com/dmehra2102/prod-golang-projects/medflow/internal/domain/medical_record"
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
