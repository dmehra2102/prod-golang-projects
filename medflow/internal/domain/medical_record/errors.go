package medical_record

import "errors"

var (
	ErrRecordNotFound    = errors.New("medical record not found")
	ErrRecordImmutable   = errors.New("medical records cannot be modified; use addenda")
	ErrInvalidRecordType = errors.New("invalid medical record type")
)
