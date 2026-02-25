package database

import (
	"fmt"
	"time"

	"github.com/dmehra2102/prod-golang-projects/medflow/internal/config"
	"github.com/dmehra2102/prod-golang-projects/medflow/internal/domain"
	"github.com/dmehra2102/prod-golang-projects/medflow/internal/domain/appointment"
	mr "github.com/dmehra2102/prod-golang-projects/medflow/internal/domain/medical_record"
	"github.com/dmehra2102/prod-golang-projects/medflow/internal/domain/patient"
	"github.com/dmehra2102/prod-golang-projects/medflow/internal/domain/prescription"
	"go.uber.org/zap"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"
)

func Connect(cfg config.DatabaseConfig) (*gorm.DB, error) {
	gormCfg := &gorm.Config{
		Logger:                                   gormlogger.Default.LogMode(gormlogger.Silent),
		PrepareStmt:                              true,
		DisableForeignKeyConstraintWhenMigrating: false,
		DisableAutomaticPing:                     false,
	}

	db, err := gorm.Open(postgres.New(postgres.Config{
		DSN:                  cfg.DNS(),
		PreferSimpleProtocol: false,
	}), gormCfg)
	if err != nil {
		return nil, fmt.Errorf("connecting to database: %w", err)
	}

	// Configure connection pool
	sqlDB, err := db.DB()
	if err != nil {
		return nil, fmt.Errorf("getting underlying sql.DB: %w", err)
	}

	sqlDB.SetMaxOpenConns(cfg.MaxOpenConns)
	sqlDB.SetMaxIdleConns(cfg.MaxIdleConns)
	sqlDB.SetConnMaxLifetime(cfg.ConnMaxLifetime)
	sqlDB.SetConnMaxIdleTime(cfg.ConnMaxIdleTime)

	if err := sqlDB.Ping(); err != nil {
		return nil, fmt.Errorf("pinging database: %w", err)
	}

	return db, nil
}

func Migrate(db *gorm.DB, log *zap.Logger) error {
	log.Info("running database migrations")
	start := time.Now()

	schemas := []string{"clinical", "auth", "audit"} // logical namespace
	for _, schema := range schemas {
		if err := db.Exec(fmt.Sprintf("CREATE SCHEMA IF NOT EXISTS %s", schema)).Error; err != nil {
			return fmt.Errorf("creating schema %s: %w", schema, err)
		}
	}

	models := []any{
		&domain.User{},
		&domain.AuditLog{},
		&patient.Patient{},
		&appointment.Appointment{},
		&mr.MedicalRecord{},
		&mr.Addendum{},
		&prescription.Prescription{},
	}

	if err := db.AutoMigrate(models...); err != nil {
		return fmt.Errorf("auto-migrating models: %w", err)
	}

	if err := createIndexes(db); err != nil {
		return fmt.Errorf("creating indexes: %w", err)
	}

	log.Info("migrations completed", zap.Duration("duration", time.Since(start)))
	return nil
}

func createIndexes(db *gorm.DB) error {
	indexes := []struct {
		name  string
		query string
	}{
		{
			name:  "idx_appointments_doctor_schedule",
			query: `CREATE INDEX IF NOT EXISTS idx_appointments_doctor_schedule ON clinical.appointments (doctor_id, scheduled_at, duration_mins) WHERE deleted_at IS NULL AND status NOT IN ('cancelled', 'no_show')`,
		},
		// Patient search: GIN index for full-text search on name fields
		{
			name:  "idx_patients_name_search",
			query: `CREATE INDEX IF NOT EXISTS idx_patients_name_trgm ON clinical.patients USING gin ((first_name || ' ' || last_name) gin_trgm_ops) WHERE deleted_at IS NULL`,
		},
		{
			name:  "idx_prescriptions_active",
			query: `CREATE INDEX IF NOT EXISTS idx_prescriptions_Active ON clinical.prescriptions (patient_id, status, expires_at) WHERE deleted_at IS NULL AND status = 'active'`,
		},
		{
			name:  "idx_appointments_time_range",
			query: `CREATE INDEX IF NOT EXISTS idx_appointments_time_range ON clinical.appointments (scheduled_at, status) WHERE deleted_at IS NULL`,
		},
	}

	for _, idx := range indexes {
		_ = db.Exec("CREATE EXTENSION IF NOT EXISTS pg_trgm").Error

		if err := db.Exec(idx.query).Error; err != nil {
			_ = err
		}
	}

	return nil
}
