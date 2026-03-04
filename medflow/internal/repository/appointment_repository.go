package repository

import (
	"context"
	"errors"
	"math"
	"time"

	"github.com/dmehra2102/prod-golang-projects/medflow/internal/domain/appointment"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

type AppointmentRepository struct {
	db *gorm.DB
}

func NewAppointmentRepository(db *gorm.DB) appointment.Repository {
	return &AppointmentRepository{db: db}
}

func (r *AppointmentRepository) Create(ctx context.Context, a *appointment.Appointment) error {
	return r.db.WithContext(ctx).Create(a).Error
}

func (r *AppointmentRepository) GetByID(ctx context.Context, id uuid.UUID) (*appointment.Appointment, error) {
	var a appointment.Appointment
	result := r.db.WithContext(ctx).Where("id = ? AND deleted_at IS NULL", id).First(&a)

	if result.Error != nil {
		if errors.Is(result.Error, gorm.ErrRecordNotFound) {
			return nil, appointment.ErrAppointmentNotFound
		}
		return nil, result.Error
	}
	return &a, nil
}

func (r *AppointmentRepository) Update(ctx context.Context, id uuid.UUID, cmd *appointment.UpdateAppointmentCommand) (*appointment.Appointment, error) {
	updates := buildAppointmentUpdates(cmd)

	result := r.db.WithContext(ctx).Model(&appointment.Appointment{}).
		Where("id = ? AND deleted_at IS NULL", id).
		Updates(updates)

	if result.Error != nil {
		return nil, result.Error
	}
	return r.GetByID(ctx, id)
}

func (r *AppointmentRepository) UpdateStatus(ctx context.Context, a *appointment.Appointment) error {
	updates := map[string]any{
		"status": a.Status,
	}
	if a.CancelledAt != nil {
		updates["cancelled_at"] = a.CancelledAt
		updates["cancellation_reason"] = a.CancellationReason
		updates["cancelled_by"] = a.CancelledBy
	}
	if a.CompletedAt != nil {
		updates["completed_at"] = a.CompletedAt
		updates["actual_duration_mins"] = a.ActualDurationMins
	}

	return r.db.WithContext(ctx).
		Model(&appointment.Appointment{}).
		Where("id = ?", a.ID).
		Updates(updates).Error
}

func (r *AppointmentRepository) List(ctx context.Context, q *appointment.ListAppointmentsQuery) (*appointment.PagedAppointments, error) {
	query := r.db.WithContext(ctx).Model(&appointment.Appointment{}).Where("deleted_at IS NULL")

	if q.PatientID != nil {
		query = query.Where("patient_id = ?", *q.PatientID)
	}
	if q.DoctorID != nil {
		query = query.Where("doctor_id = ?", *q.DoctorID)
	}
	if q.Status != nil {
		query = query.Where("status = ?", *q.Status)
	}
	if q.Type != nil {
		query = query.Where("type = ?", *q.Type)
	}
	if q.DateFrom != nil {
		query = query.Where("scheduled_at >= ?", *q.DateFrom)
	}
	if q.DateTo != nil {
		query = query.Where("scheduled_at <= ?", *q.DateTo)
	}

	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, err
	}

	offset := (q.Page - 1) * q.PageSize
	var appointments []*appointment.Appointment
	if err := query.Order("scheduled_at DESC").Offset(offset).Limit(q.PageSize).Find(&appointments).Error; err != nil {
		return nil, err
	}

	return &appointment.PagedAppointments{
		Appointments: appointments,
		TotalCount:   total,
		Page:         q.Page,
		PageSize:     q.PageSize,
		TotalPages:   int(math.Ceil(float64(total) / float64(q.PageSize))),
	}, nil
}

// Two appointments [s1,e1] and [s2,e2] overlap iff s1 < e2 AND s2 < e1.
func (r *AppointmentRepository) HasConflict(ctx context.Context, doctorID uuid.UUID, start, end time.Time, excludeID *uuid.UUID) (bool, error) {
	query := r.db.WithContext(ctx).Model(&appointment.Appointment{}).
		Where("doctor_id = ?", doctorID).
		Where("deleted_at IS NULL").
		Where("status NOT IN ?", []string{"cancelled", "no_show"}).
		// Overlap condition: existing_start < new_end AND existing_end > new_start
		Where("scheduled_at < ? AND scheduled_at + (duration_mins * interval '1 minute') > ?", end, start)

	if excludeID != nil {
		query = query.Where("id != ?", *excludeID)
	}

	var count int64
	if err := query.Count(&count).Error; err != nil {
		return false, err
	}
	return count > 0, nil
}

func (r *AppointmentRepository) GetUpcoming(ctx context.Context, withinHours int) ([]*appointment.Appointment, error) {
	now := time.Now()
	future := now.Add(time.Duration(withinHours) * time.Hour)

	var appointments []*appointment.Appointment
	err := r.db.WithContext(ctx).
		Where("scheduled_at BETWEEN ? AND ?", now, future).
		Where("status IN ?", []string{"scheduled", "confirmed"}).
		Where("deleted_at IS NULL").
		Find(&appointments).Error

	return appointments, err
}

func buildAppointmentUpdates(cmd *appointment.UpdateAppointmentCommand) map[string]any {
	updates := make(map[string]any)

	if cmd.ScheduledAt != nil {
		updates["scheduled_at"] = *cmd.ScheduledAt
	}
	if cmd.DurationMins != nil {
		updates["duration_mins"] = *cmd.DurationMins
	}
	if cmd.Type != nil {
		updates["type"] = *cmd.Type
	}
	if cmd.ChiefComplaint != nil {
		updates["chief_complaint"] = *cmd.ChiefComplaint
	}
	if cmd.Notes != nil {
		updates["notes"] = *cmd.Notes
	}
	if cmd.Room != nil {
		updates["room"] = *cmd.Room
	}

	return updates
}
