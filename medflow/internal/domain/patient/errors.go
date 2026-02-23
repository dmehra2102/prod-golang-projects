package patient

import "errors"

var (
	ErrPatientNotFound      = errors.New("patient not found")
	ErrPatientAlreadyExists = errors.New("patient with this national ID already exists")
	ErrPatientDeceased      = errors.New("operation not permitted: patient is deceased")
	ErrInvalidGender        = errors.New("invalid gender value")
	ErrInvalidBloodType     = errors.New("invalid blood type")
	ErrInvalidDateOfBirth   = errors.New("date of birth cannot be in the future")
	ErrNationalIDRequired   = errors.New("national ID is required")
)
