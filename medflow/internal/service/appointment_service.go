package service

import (
	"context"
	"fmt"
	"time"

	"github.com/dmehra2102/prod-golang-projects/medflow/internal/domain/appointment"
	"github.com/dmehra2102/prod-golang-projects/medflow/internal/domain/patient"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

type AppointmentService struct {
	repo        appointment.Repository
	patientRepo patient.Repository
	auditSvc    *AuditService
	log         *zap.Logger
}

func NewAppointmentService(
	repo appointment.Repository,
	patientRepo patient.Repository,
	auditSvc *AuditService,
	log *zap.Logger,
) *AppointmentService {
	return &AppointmentService{repo: repo, patientRepo: patientRepo, auditSvc: auditSvc, log: log}
}

func (s *AppointmentService) ScheduleAppointment(
	ctx context.Context,
	cmd *appointment.CreateAppointmentCommand,
	callerID uuid.UUID,
	callerRole string,
	ip string,
) (*appointment.Appointment, error) {
	// -------- Input Validation -----------
	if cmd.ScheduledAt.Before(time.Now()) {
		return nil, appointment.ErrScheduledInPast
	}
	if cmd.DurationMins < 5 || cmd.DurationMins > 480 {
		return nil, appointment.ErrInvalidDuration
	}
	if !cmd.Type.IsValid() {
		return nil, appointment.ErrInvalidAppointmentType
	}

	// ── Verify patient is active ───────────────────────────────────────────
	p, err := s.patientRepo.GetByID(ctx, cmd.PatientID)
	if err != nil {
		return nil, fmt.Errorf("verifying patient: %w", err)
	}
	if !p.IsActive() {
		return nil, fmt.Errorf("patient is not active")
	}

	endsAt := cmd.ScheduledAt.Add(time.Duration(cmd.DurationMins) * time.Minute)
	conflict, err := s.repo.HasConflict(ctx, cmd.DoctorID, cmd.ScheduledAt, endsAt, nil)
	if err != nil {
		return nil, fmt.Errorf("checking conflicts: %w", err)
	}
	if conflict {
		return nil, appointment.ErrAppointmentConflict
	}

	a := &appointment.Appointment{
		PatientID:      cmd.PatientID,
		DoctorID:       cmd.DoctorID,
		ScheduledAt:    cmd.ScheduledAt,
		DurationMins:   cmd.DurationMins,
		Type:           cmd.Type,
		Status:         appointment.StatusScheduled,
		ChiefComplaint: cmd.ChiefComplaint,
		Notes:          cmd.Notes,
		Room:           cmd.Room,
		CreatedBy:      cmd.CreatedBy,
	}

	if err := s.repo.Create(ctx, a); err != nil {
		s.log.Error("failed to create appointment", zap.Error(err))
		return nil, fmt.Errorf("creating appointment: %w", err)
	}

	s.auditSvc.LogAsync(ctx, AuditEntry{
		UserID:       callerID,
		UserRole:     callerRole,
		Action:       "create",
		ResourceType: "appointment",
		ResourceID:   a.ID.String(),
		IPAddress:    ip,
	})

	return a, nil
}

func (s *AppointmentService) GetAppointment(ctx context.Context, id uuid.UUID, callerID uuid.UUID, callerRole string, callerPatientID *uuid.UUID, ip string) (*appointment.Appointment, error) {
	a, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}

	if callerRole == "patient" {
		if callerPatientID == nil || *callerPatientID != a.PatientID {
			return nil, ErrForbidden
		}
	}

	s.auditSvc.LogAsync(ctx, AuditEntry{
		UserID: callerID, UserRole: callerRole,
		Action: "read", ResourceType: "appointment", ResourceID: id.String(), IPAddress: ip,
	})

	return a, nil
}

func (s *AppointmentService) CancelAppointment(ctx context.Context, id uuid.UUID, cmd *appointment.CancelAppointmentCommand, callerID uuid.UUID, callerRole string, callerPatientID *uuid.UUID, ip string) (*appointment.Appointment, error) {
	a, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}

	if callerRole == "patient" {
		if callerPatientID == nil || *callerPatientID != a.PatientID {
			return nil, ErrForbidden
		}
	}

	if err := a.Cancel(cmd.Reason, cmd.CancelledBy); err != nil {
		return nil, err
	}

	if err := s.repo.UpdateStatus(ctx, a); err != nil {
		return nil, fmt.Errorf("updating appointment status: %w", err)
	}

	s.auditSvc.LogAsync(ctx, AuditEntry{
		UserID: callerID, UserRole: callerRole,
		Action: "update", ResourceType: "appointment", ResourceID: id.String(), IPAddress: ip,
		Changes: fmt.Sprintf(`{"status":"cancelled","reason":"%s"}`, cmd.Reason),
	})

	return a, nil
}

func (s *AppointmentService) ConfirmAppointment(ctx context.Context, id uuid.UUID, callerID uuid.UUID, callerRole string, ip string) (*appointment.Appointment, error) {
	a, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if !a.CanTransitionTo(appointment.StatusConfirmed) {
		return nil, appointment.ErrInvalidStatusTransition
	}
	a.Status = appointment.StatusConfirmed
	if err := s.repo.UpdateStatus(ctx, a); err != nil {
		return nil, err
	}
	return a, nil
}

func (s *AppointmentService) CompleteAppointment(ctx context.Context, id uuid.UUID, cmd *appointment.CompleteAppointmentCommand, callerID uuid.UUID, callerRole string, ip string) (*appointment.Appointment, error) {
	a, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if err := a.Complete(cmd.ActualDurationMins); err != nil {
		return nil, err
	}
	if err := s.repo.UpdateStatus(ctx, a); err != nil {
		return nil, err
	}
	return a, nil
}

func (s *AppointmentService) ListAppointments(ctx context.Context, q *appointment.ListAppointmentsQuery, callerRole string, callerPatientID *uuid.UUID) (*appointment.PagedAppointments, error) {
	// Patients can only see their own appointments
	if callerRole == "patient" && callerPatientID != nil {
		q.PatientID = callerPatientID
	}
	if q.PageSize <= 0 || q.PageSize > 100 {
		q.PageSize = 20
	}
	if q.Page <= 0 {
		q.Page = 1
	}
	return s.repo.List(ctx, q)
}
