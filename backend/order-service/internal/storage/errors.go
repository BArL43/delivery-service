package storage

import "errors"

var (
	ErrCourierNotFound      = errors.New("courier not found")
	ErrCourierBusy          = errors.New("courier already has an active order")
	ErrCourierUnavailable   = errors.New("courier is offline")
	ErrOrderNotFound        = errors.New("order not found")
	ErrOrderNotAssignable   = errors.New("order cannot be assigned in its current status")
	ErrOrderAlreadyAssigned = errors.New("order is already assigned")
	ErrAssignmentNotFound   = errors.New("assignment not found")
	ErrAssignmentOwnership  = errors.New("assignment belongs to another courier")
	ErrInvalidTransition    = errors.New("invalid assignment status transition")
)
