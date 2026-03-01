package service

import (
	"context"
	"fmt"

	mr "github.com/dmehra2102/prod-golang-projects/medflow/internal/domain/medical_record"
	"github.com/dmehra2102/prod-golang-projects/medflow/internal/domain/patient"
	"github.com/dmehra2102/prod-golang-projects/medflow/internal/domain/prescription"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

type MedicalRecordService struct {
	repo        mr.Repository
	patientRepo patient.Repository
	auditSvc    *AuditService
	log         *zap.Logger
}

func NewMedicalRecordService(repo mr.Repository, patientRepo patient.Repository, auditSvc *AuditService, log *zap.Logger) *MedicalRecordService {
	return &MedicalRecordService{repo: repo, patientRepo: patientRepo, auditSvc: auditSvc, log: log}
}

func (s *MedicalRecordService) CreateRecord(ctx context.Context, cmd *mr.CreateRecordCommand, callerID uuid.UUID, callerRole string, ip string) (*mr.MedicalRecord, error) {
	if callerRole != "doctor" && callerRole != "nurse" && callerRole != "admin" {
		return nil, ErrForbidden
	}

	// Checking Patient Existence
	if _, err := s.patientRepo.GetByID(ctx, cmd.PatientID); err != nil {
		return nil, fmt.Errorf("verifying patient: %w", err)
	}

	medicalRecord := &mr.MedicalRecord{
		PatientID:     cmd.PatientID,
		AppointmentID: cmd.AppointmentID,
		DoctorID:      cmd.DoctorID,
		Type:          cmd.Type,
		SOAPNote:      cmd.SOAPNote,
		Vitals:        cmd.Vitals,
		Diagnoses:     cmd.Diagnoses,
		Notes:         cmd.Notes,
		CreatedBy:     cmd.CreatedBy,
	}

	if err := s.repo.Create(ctx, medicalRecord); err != nil {
		return nil, fmt.Errorf("creating medical record: %w", err)
	}

	s.auditSvc.LogAsync(ctx, AuditEntry{
		UserID:       callerID,
		UserRole:     callerRole,
		Action:       "create",
		ResourceType: "medical_record",
		ResourceID:   medicalRecord.ID.String(),
		IPAddress:    ip,
	})

	return medicalRecord, nil
}

func (s *MedicalRecordService) GetRecord(ctx context.Context, id uuid.UUID, callerID uuid.UUID, callerRole string, callerPatientID *uuid.UUID, ip string) (*mr.MedicalRecord, error) {
	record, err := s.repo.GetBydID(ctx, id)
	if err != nil {
		return nil, err
	}

	if callerRole == "patient" {
		if callerPatientID == nil || *callerPatientID != record.PatientID {
			return nil, ErrForbidden
		}
	}

	s.auditSvc.LogAsync(ctx, AuditEntry{
		UserID: callerID, UserRole: callerRole,
		Action: "read", ResourceType: "medical_record", ResourceID: id.String(), IPAddress: ip,
	})

	return record, nil
}

// AddAddendum appends a correction to an existing record without modifying it.
func (s *MedicalRecordService) AddAddendum(ctx context.Context, cmd *mr.AddAddendumCommand, callerID uuid.UUID, callerRole string, ip string) (*mr.Addendum, error) {
	if callerRole != "doctor" && callerRole != "admin" {
		return nil, ErrForbidden
	}

	addendum := &mr.Addendum{
		MedicalRecordID: cmd.MedicalRecordID,
		Content:         cmd.Content,
		CreatedBy:       cmd.CreatedBy,
	}

	if err := s.repo.AddAddendum(ctx, addendum); err != nil {
		return nil, err
	}

	s.auditSvc.LogAsync(ctx, AuditEntry{
		UserID: callerID, UserRole: callerRole,
		Action: "update", ResourceType: "medical_record", ResourceID: cmd.MedicalRecordID.String(), IPAddress: ip,
		Changes: `{"action":"addendum_added"}`,
	})

	return addendum, nil
}

func (s *MedicalRecordService) ListRecords(ctx context.Context, q *mr.ListRecordsQuery, callerRole string, callerPatientID *uuid.UUID) (*mr.PagedRecords, error) {
	if callerRole == "patient" && callerPatientID != nil {
		q.PatientID = callerPatientID
	}
	if q.PageSize <= 0 || q.PageSize > 100 {
		q.PageSize = 20
	}
	return s.repo.List(ctx, q)
}

type PrescriptionService struct {
	repo        prescription.Repository
	patientRepo patient.Repository
	auditSvc    *AuditService
	log         *zap.Logger
}

func NewPrescriptionService(repo prescription.Repository, patientRepo patient.Repository, auditSvc *AuditService, log *zap.Logger) *PrescriptionService {
	return &PrescriptionService{repo: repo, patientRepo: patientRepo, auditSvc: auditSvc, log: log}
}

// Only doctors can prescribe medications.
func (s *PrescriptionService) CreatePrescription(ctx context.Context, cmd *prescription.CreatePrescriptionCommand, callerID uuid.UUID, callerRole string, ip string) (*prescription.Prescription, error) {
	if callerRole != "doctor" && callerRole != "admin" {
		return nil, ErrForbidden
	}

	// Validate DEA schedule for controlled substances
	if cmd.IsControlledSubstance {
		if cmd.DEASchedule == nil || *cmd.DEASchedule < 1 || *cmd.DEASchedule > 5 {
			return nil, prescription.ErrInvalidDEASchedule
		}
	}

	p := &prescription.Prescription{
		PatientID:             cmd.PatientID,
		DoctorID:              cmd.DoctorID,
		AppointmentID:         cmd.AppointmentID,
		MedicationName:        cmd.MedicationName,
		GenericName:           cmd.GenericName,
		DosageAmount:          cmd.DosageAmount,
		DosageFrequency:       cmd.DosageFrequency,
		Route:                 cmd.Route,
		Duration:              cmd.Duration,
		Quantity:              cmd.Quantity,
		RefillsAllowed:        cmd.RefillsAllowed,
		IsControlledSubstance: cmd.IsControlledSubstance,
		DEASchedule:           cmd.DEASchedule,
		IssuedAt:              cmd.IssuedAt,
		ExpiresAt:             cmd.ExpiresAt,
		Status:                prescription.StatusActive,
		Instructions:          cmd.Instructions,
		Warnings:              cmd.Warnings,
		CreatedBy:             cmd.CreatedBy,
	}

	if err := s.repo.Create(ctx, p); err != nil {
		return nil, fmt.Errorf("creating prescription: %w", err)
	}

	s.auditSvc.LogAsync(ctx, AuditEntry{
		UserID: callerID, UserRole: callerRole,
		Action: "create", ResourceType: "prescription", ResourceID: p.ID.String(), IPAddress: ip,
	})

	return p, nil
}

// RefillPrescription processes a refill request.
func (s *PrescriptionService) RefillPrescription(ctx context.Context, id uuid.UUID, callerID uuid.UUID, callerRole string, ip string) (*prescription.Prescription, error) {
	updated, err := s.repo.Refill(ctx, id)
	if err != nil {
		return nil, err
	}

	s.auditSvc.LogAsync(ctx, AuditEntry{
		UserID: callerID, UserRole: callerRole,
		Action: "update", ResourceType: "prescription", ResourceID: id.String(), IPAddress: ip,
		Changes: `{"action":"refill"}`,
	})

	return updated, nil
}
