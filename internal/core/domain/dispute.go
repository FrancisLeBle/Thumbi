package domain

import (
	"strings"
	"time"

	"github.com/google/uuid"
)

// DisputeStatus enumera los estados de tramitación de un reclamo sobre fondos en custodia
type DisputeStatus string

const (
	DisputeStatusOpened                  DisputeStatus = "OPENED"
	DisputeStatusInReview                DisputeStatus = "IN_REVIEW"
	DisputeStatusResolvedPassengerRefund DisputeStatus = "RESOLVED_PASSENGER_REFUND"
	DisputeStatusResolvedDriverPayout    DisputeStatus = "RESOLVED_DRIVER_PAYOUT"
	DisputeStatusRejected                DisputeStatus = "REJECTED"
)

// IsValid verifica que el estado de la disputa pertenezca a los valores válidos del dominio.
func (s DisputeStatus) IsValid() bool {
	switch s {
	case DisputeStatusOpened,
		DisputeStatusInReview,
		DisputeStatusResolvedPassengerRefund,
		DisputeStatusResolvedDriverPayout,
		DisputeStatusRejected:
		return true
	default:
		return false
	}
}

// String retorna la representación textual del estado de la disputa.
func (s DisputeStatus) String() string {
	return string(s)
}

// IsValidResolution valida si el estado corresponde a un dictamen terminal válido.
func (s DisputeStatus) IsValidResolution() bool {
	return s == DisputeStatusResolvedPassengerRefund ||
		s == DisputeStatusResolvedDriverPayout ||
		s == DisputeStatusRejected
}

// DisputeReason clasifica la tipología de incidentes o incumplimientos reportados
type DisputeReason string

const (
	DisputeReasonNoShow               DisputeReason = "NO_SHOW"
	DisputeReasonRouteDeviation       DisputeReason = "ROUTE_DEVIATION"
	DisputeReasonRecklessDriving      DisputeReason = "RECKLESS_DRIVING"
	DisputeReasonVehicleMismatch      DisputeReason = "VEHICLE_MISMATCH"
	DisputeReasonPassengerMisconduct  DisputeReason = "PASSENGER_MISCONDUCT"
	DisputeReasonExtraChargeRequested DisputeReason = "EXTRA_CHARGE_REQUESTED"
	DisputeReasonOther                DisputeReason = "OTHER"
)

// IsValid verifica que el motivo de la disputa pertenezca al conjunto tipado del dominio.
func (r DisputeReason) IsValid() bool {
	switch r {
	case DisputeReasonNoShow,
		DisputeReasonRouteDeviation,
		DisputeReasonRecklessDriving,
		DisputeReasonVehicleMismatch,
		DisputeReasonPassengerMisconduct,
		DisputeReasonExtraChargeRequested,
		DisputeReasonOther:
		return true
	default:
		return false
	}
}

// String retorna la representación textual del motivo de disputa.
func (r DisputeReason) String() string {
	return string(r)
}

// DisputeParams agrupa los datos necesarios para registrar una nueva disputa en el dominio
type DisputeParams struct {
	ID                  uuid.UUID
	EscrowTransactionID uuid.UUID
	BookingID           uuid.UUID
	TripID              uuid.UUID
	ReporterID          uuid.UUID
	DefendantID         uuid.UUID
	Reason              DisputeReason
	Description         string
	EvidenceURLs        []string
	EscrowStatus        EscrowStatus
}

// Dispute representa un reclamo formal que congela la liquidación de fondos en custodia
type Dispute struct {
	ID                  uuid.UUID     `json:"id"`
	EscrowTransactionID uuid.UUID     `json:"escrow_transaction_id"`
	BookingID           uuid.UUID     `json:"booking_id"`
	TripID              uuid.UUID     `json:"trip_id"`
	ReporterID          uuid.UUID     `json:"reporter_id"`
	DefendantID         uuid.UUID     `json:"defendant_id"`
	Reason              DisputeReason `json:"reason"`
	Description         string        `json:"description"`
	EvidenceURLs        []string      `json:"evidence_urls"`
	Status              DisputeStatus `json:"status"`
	AdminNotes          *string       `json:"admin_notes,omitempty"`
	ResolvedBy          *uuid.UUID    `json:"resolved_by,omitempty"`
	ResolvedAt          *time.Time    `json:"resolved_at,omitempty"`
	CreatedAt           time.Time     `json:"created_at"`
	UpdatedAt           time.Time     `json:"updated_at"`
}

// NewDispute construye y valida una nueva disputa garantizando las invariantes de negocio
func NewDispute(params DisputeParams) (*Dispute, error) {
	if params.ID == uuid.Nil ||
		params.EscrowTransactionID == uuid.Nil ||
		params.BookingID == uuid.Nil ||
		params.TripID == uuid.Nil ||
		params.ReporterID == uuid.Nil ||
		params.DefendantID == uuid.Nil {
		return nil, ErrInvalidDisputeData
	}

	if !params.Reason.IsValid() {
		return nil, ErrInvalidDisputeData
	}

	// Invariante: No se puede disputar contra uno mismo
	if params.ReporterID == params.DefendantID {
		return nil, ErrDisputeSelfNotAllowed
	}

	// Invariante: Descripción obligatoria de los hechos denunciados
	trimmedDesc := strings.TrimSpace(params.Description)
	if trimmedDesc == "" {
		return nil, ErrInvalidDisputeData
	}

	if !params.EscrowStatus.IsValid() {
		return nil, ErrInvalidStatus
	}

	// Invariante: Solo fondos retenidos o congelados pueden disputarse
	if params.EscrowStatus != EscrowStatusHeld && params.EscrowStatus != EscrowStatusDisputed {
		return nil, ErrEscrowAlreadySettled
	}

	evidence := params.EvidenceURLs
	if evidence == nil {
		evidence = make([]string, 0)
	}

	nowUTC := time.Now().UTC()
	return &Dispute{
		ID:                  params.ID,
		EscrowTransactionID: params.EscrowTransactionID,
		BookingID:           params.BookingID,
		TripID:              params.TripID,
		ReporterID:          params.ReporterID,
		DefendantID:         params.DefendantID,
		Reason:              params.Reason,
		Description:         trimmedDesc,
		EvidenceURLs:        evidence,
		Status:              DisputeStatusOpened,
		CreatedAt:           nowUTC,
		UpdatedAt:           nowUTC,
	}, nil
}

// StartReview transiciona la disputa a IN_REVIEW cuando un mediador toma el caso
func (d *Dispute) StartReview(adminID uuid.UUID, now time.Time) error {
	if d.IsClosed() {
		return ErrDisputeAlreadyResolved
	}
	if adminID == uuid.Nil {
		return ErrUnauthorized
	}

	nowUTC := now.UTC()
	d.Status = DisputeStatusInReview
	d.ResolvedBy = &adminID
	d.UpdatedAt = nowUTC
	return nil
}

// ResolvePassengerRefund dictamina la devolución total de fondos al pasajero
func (d *Dispute) ResolvePassengerRefund(adminID uuid.UUID, adminNotes string, now time.Time) error {
	if d.IsClosed() {
		return ErrDisputeAlreadyResolved
	}
	if adminID == uuid.Nil {
		return ErrUnauthorized
	}

	nowUTC := now.UTC()
	trimmedNotes := strings.TrimSpace(adminNotes)
	d.Status = DisputeStatusResolvedPassengerRefund
	d.AdminNotes = &trimmedNotes
	d.ResolvedBy = &adminID
	d.ResolvedAt = &nowUTC
	d.UpdatedAt = nowUTC
	return nil
}

// ResolveDriverPayout dictamina la liberación de fondos custodiados a favor del conductor
func (d *Dispute) ResolveDriverPayout(adminID uuid.UUID, adminNotes string, now time.Time) error {
	if d.IsClosed() {
		return ErrDisputeAlreadyResolved
	}
	if adminID == uuid.Nil {
		return ErrUnauthorized
	}

	nowUTC := now.UTC()
	trimmedNotes := strings.TrimSpace(adminNotes)
	d.Status = DisputeStatusResolvedDriverPayout
	d.AdminNotes = &trimmedNotes
	d.ResolvedBy = &adminID
	d.ResolvedAt = &nowUTC
	d.UpdatedAt = nowUTC
	return nil
}

// Reject desestima el reclamo por falta de mérito probatorio
func (d *Dispute) Reject(adminID uuid.UUID, adminNotes string, now time.Time) error {
	if d.IsClosed() {
		return ErrDisputeAlreadyResolved
	}
	if adminID == uuid.Nil {
		return ErrUnauthorized
	}

	nowUTC := now.UTC()
	trimmedNotes := strings.TrimSpace(adminNotes)
	d.Status = DisputeStatusRejected
	d.AdminNotes = &trimmedNotes
	d.ResolvedBy = &adminID
	d.ResolvedAt = &nowUTC
	d.UpdatedAt = nowUTC
	return nil
}

// IsClosed indica si el reclamo ya concluyó con una resolución definitiva
func (d *Dispute) IsClosed() bool {
	return d.Status.IsValidResolution()
}

// IsValidDisputeResolution valida que el dictamen corresponda a un estado terminal permitido
func IsValidDisputeResolution(status DisputeStatus) bool {
	return status.IsValidResolution()
}
