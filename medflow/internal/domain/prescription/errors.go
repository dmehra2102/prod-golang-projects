package prescription

import "errors"

var (
	ErrPrescriptionNotFound = errors.New("prescription not found")
	ErrNotRefillable        = errors.New("prescription cannot be refilled")
	ErrControlledSubstance  = errors.New("controlled substance requires additional authorization")
	ErrInvalidDEASchedule   = errors.New("DEA schedule must be between 1 and 5")
)
