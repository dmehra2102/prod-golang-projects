package appointment

import "errors"

var (
	ErrAppointmentNotFound     = errors.New("appointment not found")
	ErrAppointmentConflict     = errors.New("appointment time slot is already booked")
	ErrInvalidStatusTransition = errors.New("invalid appointment status transition")
	ErrScheduledInPast         = errors.New("cannot schedule appointment in the past")
	ErrInvalidDuration         = errors.New("appointment duration must be between 5 and 480 minutes")
	ErrInvalidAppointmentType  = errors.New("invalid appointment type")
)