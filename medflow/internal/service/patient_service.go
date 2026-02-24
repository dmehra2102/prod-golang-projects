package service

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/dmehra2102/prod-golang-projects/medflow/internal/domain/patient"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

type PatientService struct {
	repo     patient.Repository
	auditSvc *AuditService
	log      *zap.Logger
}

func NewPatientService(repo patient.Repository, auditSvc *AuditService, log *zap.Logger) *PatientService {
	return &PatientService{
		repo:     repo,
		auditSvc: auditSvc,
		log:      log,
	}
}

func (s *PatientService) CreatePatient(ctx context.Context, cmd *patient.CreatePatientCommand, callerID uuid.UUID, callerRole string, ip string) (*patient.Patient, error) {
	if err := validateCreateCommand(cmd); err != nil {
		return nil, err
	}

	exists, err := s.repo.ExistsByNationalID(ctx, cmd.NationalID, nil)
	if err != nil {
		s.log.Error("failed to check national ID uniqueness", zap.Error(err))
		return nil, fmt.Errorf("checking uniqueness: %w", err)
	}
	if exists {
		return nil, patient.ErrPatientAlreadyExists
	}

	p := &patient.Patient{
		FirstName:   strings.TrimSpace(cmd.FirstName),
		LastName:    strings.TrimSpace(cmd.LastName),
		DateOfBirth: cmd.DateOfBirth,
		Gender:      cmd.Gender,
		BloodType:   cmd.BloodType,
		NationalID:  strings.TrimSpace(cmd.NationalID),
		ContactInfo: patient.ContactInfo{
			Phone:   strings.TrimSpace(cmd.Phone),
			Email:   strings.ToLower(strings.TrimSpace(cmd.Email)),
			Address: cmd.Address,
			City:    cmd.City,
			State:   cmd.State,
			ZipCode: cmd.ZipCode,
			Country: cmd.Country,
		},
		EmergencyContact:  cmd.EmergencyContact,
		Insurance:         cmd.Insurance,
		Allergies:         cmd.Allergies,
		ChronicConditions: cmd.ChronicConditions,
		AssignedDoctorID:  cmd.AssignedDoctorID,
		Notes:             cmd.Notes,
		Status:            patient.StatusActive,
		CreatedBy:         cmd.CreatedBy,
	}

	if err := s.repo.Create(ctx, p); err != nil {
		s.log.Error("failed to create patient", zap.Error(err))
		return nil, fmt.Errorf("creating patient: %w", err)
	}

	s.auditSvc.LogAsync(ctx, AuditEntry{
		UserID:       callerID,
		UserRole:     callerRole,
		Action:       "create",
		ResourceType: "patient",
		ResourceID:   p.ID.String(),
		IPAddress:    ip,
	})

	s.log.Info("patient created",
		zap.String("patient_id", p.ID.String()),
		zap.String("created_by", callerID.String()),
	)

	return p, nil
}

func (s *PatientService) GetPatient(ctx context.Context, id uuid.UUID, callerID uuid.UUID, callerRole string, callerPatientID *uuid.UUID, ip string) (*patient.Patient, error) {
	// RBAC: patients can only read their own record
	if callerRole == "patient" {
		if callerPatientID == nil || *callerPatientID != id {
			return nil, ErrForbidden
		}
	}

	p, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}

	s.auditSvc.LogAsync(ctx, AuditEntry{
		UserID:       callerID,
		UserRole:     callerRole,
		Action:       "read",
		ResourceType: "patient",
		ResourceID:   id.String(),
		IPAddress:    ip,
	})

	return p, nil
}

func (s *PatientService) DeactivatePatient(ctx context.Context, id uuid.UUID, callerID uuid.UUID, callerRole string, ip string) error {
	p, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return err
	}

	if err := p.Deactivate(); err != nil {
		return err
	}

	s.auditSvc.LogAsync(ctx, AuditEntry{
		UserID:       callerID,
		UserRole:     callerRole,
		Action:       "delete",
		ResourceType: "patient",
		ResourceID:   id.String(),
		IPAddress:    ip,
	})

	return s.repo.SoftDelete(ctx, id)
}

func (s *PatientService) ListPatients(ctx context.Context, q *patient.ListPatientsQuery, callerID uuid.UUID, callerRole string) (*patient.PagedPatients, error) {
	if q.PageSize <= 0 || q.PageSize > 100 {
		q.PageSize = 20
	}
	if q.Page <= 0 {
		q.Page = 1
	}

	return s.repo.List(ctx, q)
}

func validateCreateCommand(cmd *patient.CreatePatientCommand) error {
	var errs []string

	if strings.TrimSpace(cmd.FirstName) == "" {
		errs = append(errs, "first_name is required")
	}
	if strings.TrimSpace(cmd.LastName) == "" {
		errs = append(errs, "last_name is required")
	}
	if cmd.DateOfBirth.IsZero() {
		errs = append(errs, "date_of_birth is required")
	}
	if cmd.DateOfBirth.After(time.Now()) {
		errs = append(errs, "date_of_birth cannot be in the future")
	}
	if !cmd.Gender.IsValid() {
		errs = append(errs, "gender is invalid")
	}
	if strings.TrimSpace(cmd.NationalID) == "" {
		errs = append(errs, "national_id is required")
	}

	if len(errs) > 0 {
		return &ValidationError{Fields: errs}
	}
	return nil
}
