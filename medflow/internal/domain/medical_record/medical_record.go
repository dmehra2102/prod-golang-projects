package medical_record

import (
	"time"

	"github.com/google/uuid"
)

type RecordType string

const (
	TypeSOAP             RecordType = "soap"
	TypeDischargeSummary RecordType = "discharge_summary"
	TypeLabReport        RecordType = "lap_report"
	TypeImagingReport    RecordType = "imaging_report"
	TypeProcedureNote    RecordType = "procedure_note"
	TypeProgressNote     RecordType = "progress_note"
)

// SOAPNote represents the structured clinical note format.
type SOAPNote struct {
	Subjective string `json:"subjective"`
	Objective  string `json:"objective"`
	Assessment string `json:"assessment"`
	Plan       string `json:"plan"`
}

type Vitals struct {
	BloodPressureSystolic  *int     `json:"pb_systolic"`
	BloodPressureDiastolic *int     `json:"bp_diastolic"`
	HeartRateBPM           *int     `json:"heart_rate_bpm"`
	TemperatureCelsius     *float64 `json:"temperature_celsius"`
	WeightKg               *float64 `json:"weight_kg"`
	HeightCm               *float64 `json:"height_cm"`
	OxygenSaturation       *float64 `json:"oxygen_saturation"`
	RespiratoryRate        *int     `json:"respiratory_rate_bpm"`
}

// Attachment represents a file attached to a medical record (e.g., lab PDF).
type Attachment struct {
	ID          uuid.UUID `json:"id"`
	FileName    string    `json:"file_name"`
	ContentType string    `json:"content_type"`
	S3Key       string    `json:"s3_key"`
	SizeBytes   int64     `json:"size_bytes"`
	UploadedAt  time.Time `json:"uploaded_at"`
}

// Once created, records cannot be deleted or edited
type MedicalRecord struct {
	ID        uuid.UUID `gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
	CreatedAt time.Time `gorm:"autoCreateTime;index"`

	PatientID     uuid.UUID  `gorm:"column:patient_id;type:uuid;not null;index"`
	AppointmentID *uuid.UUID `gorm:"column:appointment_id;type:uuid;index"`
	DoctorID      uuid.UUID  `gorm:"column:doctor_id;type:uuid;not null;index"`

	Type RecordType `gorm:"column:type;type:varchar(50);not null;index"`

	SOAPNote    *SOAPNote    `gorm:"column:soap_note;serializer:json"`
	Vitals      *Vitals      `gorm:"column:vitals;serializer:json"`
	Diagnoses   []string     `gorm:"column:diagnoses;serializer:json"`
	Attachments []Attachment `gorm:"column:attachments;serializer:json"`

	Notes string `gorm:"column:notes;type:text"`

	// Addenda: corrections appended without modifying original
	Addenda []Addendum `gorm:"foreignKey:MedicalRecordID"`

	CreatedBy uuid.UUID `gorm:"column:created_by;type:uuid;not null"`
}

func (MedicalRecord) TableName() string {
	return "clinical.medical_records"
}

// Addendum is an append-only correction to an existing medical record.
// Addenda preserve the original record while allowing corrections.
type Addendum struct {
	ID              uuid.UUID `gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
	CreatedAt       time.Time `gorm:"autoCreateTime"`
	MedicalRecordID uuid.UUID `gorm:"column:medical_record_id;type:uuid;not null;index"`
	Content         string    `gorm:"column:content;type:text;not null"`
	CreatedBy       uuid.UUID `gorm:"column:created_by;type:uuid;not null"`
}

func (Addendum) TableName() string {
	return "clinical.medical_record_addenda"
}

type CreateRecordCommand struct {
	PatientID     uuid.UUID
	AppointmentID *uuid.UUID
	DoctorID      uuid.UUID
	Type          RecordType
	SOAPNote      *SOAPNote
	Vitals        *Vitals
	Diagnoses     []string
	Notes         string
	CreatedBy     uuid.UUID
}

type AddAddendumCommand struct {
	MedicalRecordID uuid.UUID
	Content         string
	CreatedBy       uuid.UUID
}

type ListRecordsQuery struct {
	PatientID     *uuid.UUID
	DoctorID      *uuid.UUID
	Type          *RecordType
	AppointmentID *uuid.UUID
	DateFrom      *time.Time
	DateTo        *time.Time
	Page          int
	PageSize      int
}

type PagedRecords struct {
	Records    []*MedicalRecord
	TotalCount int64
	Page       int
	PageSize   int
	TotalPages int
}
