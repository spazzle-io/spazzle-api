package gameevents

import (
	"github.com/google/uuid"
)

type AckStatus string

const (
	AckStatusDelivered     AckStatus = "delivered"
	AckStatusNotApplicable AckStatus = "not_applicable"
	AckStatusFailed        AckStatus = "failed"
)

type EventAckPayload struct {
	CorrelationID uuid.UUID `json:"correlation_id"`
	InstanceID    uuid.UUID `json:"instance_id"`
	Status        AckStatus `json:"status"`
	Reason        string    `json:"reason"`
}
