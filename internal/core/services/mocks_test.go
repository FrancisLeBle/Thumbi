package services_test

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/mock"
	"github.com/thumbi/backend/internal/core/domain"
	"github.com/thumbi/backend/internal/core/ports"
	"github.com/thumbi/backend/internal/core/services"
)

// MockUserRepository
type MockUserRepository struct {
	mock.Mock
}

func (m *MockUserRepository) Create(ctx context.Context, user *domain.User) error {
	args := m.Called(ctx, user)
	return args.Error(0)
}

func (m *MockUserRepository) GetByID(ctx context.Context, id uuid.UUID) (*domain.User, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.User), args.Error(1)
}

func (m *MockUserRepository) GetByEmail(ctx context.Context, email string) (*domain.User, error) {
	args := m.Called(ctx, email)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.User), args.Error(1)
}

func (m *MockUserRepository) GetByProvider(ctx context.Context, provider domain.AuthProvider, providerID string) (*domain.User, error) {
	args := m.Called(ctx, provider, providerID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.User), args.Error(1)
}

func (m *MockUserRepository) Update(ctx context.Context, user *domain.User) error {
	args := m.Called(ctx, user)
	return args.Error(0)
}

// MockIdentityRepository
type MockIdentityRepository struct {
	mock.Mock
}

func (m *MockIdentityRepository) Create(ctx context.Context, identity *domain.UserIdentity) error {
	args := m.Called(ctx, identity)
	return args.Error(0)
}

func (m *MockIdentityRepository) GetByID(ctx context.Context, id uuid.UUID) (*domain.UserIdentity, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.UserIdentity), args.Error(1)
}

func (m *MockIdentityRepository) GetByProviderAndID(ctx context.Context, provider domain.AuthProvider, providerUserID string) (*domain.UserIdentity, error) {
	args := m.Called(ctx, provider, providerUserID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.UserIdentity), args.Error(1)
}

func (m *MockIdentityRepository) ListByUserID(ctx context.Context, userID uuid.UUID) ([]*domain.UserIdentity, error) {
	args := m.Called(ctx, userID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*domain.UserIdentity), args.Error(1)
}

func (m *MockIdentityRepository) Delete(ctx context.Context, id uuid.UUID) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}

// MockSessionRepository
type MockSessionRepository struct {
	mock.Mock
}

func (m *MockSessionRepository) Save(ctx context.Context, session *domain.Session, ttl time.Duration) error {
	args := m.Called(ctx, session, ttl)
	return args.Error(0)
}

func (m *MockSessionRepository) GetByToken(ctx context.Context, token string) (*domain.Session, error) {
	args := m.Called(ctx, token)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.Session), args.Error(1)
}

func (m *MockSessionRepository) Delete(ctx context.Context, sessionID uuid.UUID) error {
	args := m.Called(ctx, sessionID)
	return args.Error(0)
}

// MockOAuthProvider
type MockOAuthProvider struct {
	mock.Mock
	ProviderName domain.AuthProvider
}

func (m *MockOAuthProvider) ValidateToken(ctx context.Context, idToken string) (*ports.SocialUserProfile, error) {
	args := m.Called(ctx, idToken)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*ports.SocialUserProfile), args.Error(1)
}

func (m *MockOAuthProvider) GetProviderName() domain.AuthProvider {
	return m.ProviderName
}

// MockKYCRepository
type MockKYCRepository struct {
	mock.Mock
}

func (m *MockKYCRepository) Create(ctx context.Context, kyc *domain.KYCVerification) error {
	args := m.Called(ctx, kyc)
	return args.Error(0)
}

func (m *MockKYCRepository) GetByID(ctx context.Context, id uuid.UUID) (*domain.KYCVerification, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.KYCVerification), args.Error(1)
}

func (m *MockKYCRepository) GetLatestByUserID(ctx context.Context, userID uuid.UUID) (*domain.KYCVerification, error) {
	args := m.Called(ctx, userID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.KYCVerification), args.Error(1)
}

func (m *MockKYCRepository) Update(ctx context.Context, kyc *domain.KYCVerification) error {
	args := m.Called(ctx, kyc)
	return args.Error(0)
}

// MockBiometricsProvider
type MockBiometricsProvider struct {
	mock.Mock
}

func (m *MockBiometricsProvider) VerifyIdentity(ctx context.Context, docType, frontBase64, backBase64, selfie3DBase64 string) (*ports.BiometricEvaluationResult, error) {
	args := m.Called(ctx, docType, frontBase64, backBase64, selfie3DBase64)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*ports.BiometricEvaluationResult), args.Error(1)
}

// MockVehicleRepository
type MockVehicleRepository struct {
	mock.Mock
}

func (m *MockVehicleRepository) Create(ctx context.Context, vehicle *domain.Vehicle) error {
	args := m.Called(ctx, vehicle)
	return args.Error(0)
}

func (m *MockVehicleRepository) GetByID(ctx context.Context, id uuid.UUID) (*domain.Vehicle, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.Vehicle), args.Error(1)
}

func (m *MockVehicleRepository) GetByUserID(ctx context.Context, userID uuid.UUID) (*domain.Vehicle, error) {
	args := m.Called(ctx, userID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.Vehicle), args.Error(1)
}

func (m *MockVehicleRepository) GetByPlateNumber(ctx context.Context, plateNumber string) (*domain.Vehicle, error) {
	args := m.Called(ctx, plateNumber)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.Vehicle), args.Error(1)
}

func (m *MockVehicleRepository) Update(ctx context.Context, vehicle *domain.Vehicle) error {
	args := m.Called(ctx, vehicle)
	return args.Error(0)
}

// MockNotifier
type MockNotifier struct {
	mock.Mock
}

func (m *MockNotifier) SendResolutionNotification(ctx context.Context, payload ports.NotificationPayload) error {
	args := m.Called(ctx, payload)
	return args.Error(0)
}

// MockTripRepository
type MockTripRepository struct {
	mock.Mock
}

func (m *MockTripRepository) Save(ctx context.Context, trip *domain.Trip) error {
	args := m.Called(ctx, trip)
	return args.Error(0)
}

func (m *MockTripRepository) FindByID(ctx context.Context, id uuid.UUID) (*domain.Trip, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.Trip), args.Error(1)
}

func (m *MockTripRepository) FindByIDsForUpdate(ctx context.Context, ids []uuid.UUID) ([]*domain.Trip, error) {
	args := m.Called(ctx, ids)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*domain.Trip), args.Error(1)
}

func (m *MockTripRepository) SearchMatchingTrips(ctx context.Context, params ports.SearchTripsParams) ([]ports.MatchedTripRecord, error) {
	args := m.Called(ctx, params)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]ports.MatchedTripRecord), args.Error(1)
}

func (m *MockTripRepository) UpdateStatus(ctx context.Context, tripID uuid.UUID, newStatus domain.TripStatus) error {
	args := m.Called(ctx, tripID, newStatus)
	return args.Error(0)
}

func (m *MockTripRepository) ReleaseSeats(ctx context.Context, transition domain.SeatTransition) (bool, error) {
	args := m.Called(ctx, transition)
	return args.Bool(0), args.Error(1)
}

func (m *MockTripRepository) ApplySeatTransitionsBatch(ctx context.Context, transitions []domain.SeatTransition, updatedAt time.Time) (int, error) {
	args := m.Called(ctx, transitions, updatedAt)
	return args.Int(0), args.Error(1)
}

func (m *MockTripRepository) CalculateRouteDistanceKm(ctx context.Context, routePath domain.LineString) (float64, error) {
	args := m.Called(ctx, routePath)
	return args.Get(0).(float64), args.Error(1)
}

// MockRoutingProvider
type MockRoutingProvider struct {
	mock.Mock
}

func (m *MockRoutingProvider) CalculateRoute(ctx context.Context, req ports.RouteCalculationRequest) (*ports.RouteDetails, error) {
	args := m.Called(ctx, req)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*ports.RouteDetails), args.Error(1)
}

// MockBookingRepository
type MockBookingRepository struct {
	mock.Mock
}

func (m *MockBookingRepository) CreateWithHold(ctx context.Context, booking *domain.Booking, seatHold domain.SeatTransition) (bool, error) {
	args := m.Called(ctx, booking, seatHold)
	return args.Bool(0), args.Error(1)
}

func (m *MockBookingRepository) FindByID(ctx context.Context, id uuid.UUID) (*domain.Booking, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.Booking), args.Error(1)
}

func (m *MockBookingRepository) FindByPaymentIntentID(ctx context.Context, intentID string) (*domain.Booking, error) {
	for _, call := range m.ExpectedCalls {
		if call.Method == "FindByPaymentIntentID" {
			args := m.Called(ctx, intentID)
			if args.Get(0) == nil {
				return nil, args.Error(1)
			}
			return args.Get(0).(*domain.Booking), args.Error(1)
		}
	}
	return nil, nil
}

func (m *MockBookingRepository) UpdateStatus(ctx context.Context, bookingID uuid.UUID, newStatus domain.BookingStatus, reason *string) error {
	args := m.Called(ctx, bookingID, newStatus, reason)
	return args.Error(0)
}

func (m *MockBookingRepository) UpdatePaymentIntent(ctx context.Context, bookingID uuid.UUID, intentID string) error {
	for _, call := range m.ExpectedCalls {
		if call.Method == "UpdatePaymentIntent" {
			args := m.Called(ctx, bookingID, intentID)
			return args.Error(0)
		}
	}
	return nil
}

func (m *MockBookingRepository) GetActiveBookingsByTrip(ctx context.Context, tripID uuid.UUID) ([]*domain.Booking, error) {
	args := m.Called(ctx, tripID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*domain.Booking), args.Error(1)
}

func (m *MockBookingRepository) GetPassengerBookings(ctx context.Context, passengerID uuid.UUID, limit, offset int) ([]*domain.Booking, error) {
	args := m.Called(ctx, passengerID, limit, offset)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*domain.Booking), args.Error(1)
}

func (m *MockBookingRepository) CheckPassengerBookingConstraints(ctx context.Context, passengerID, targetTripID uuid.UUID, departureTime, estimatedArrival, now time.Time) (bool, int, error) {
	args := m.Called(ctx, passengerID, targetTripID, departureTime, estimatedArrival, now)
	return args.Bool(0), args.Int(1), args.Error(2)
}

func (m *MockBookingRepository) LockExpiredPendingBookings(ctx context.Context, status domain.BookingStatus, overdueAt time.Time, limit int) ([]*domain.Booking, error) {
	args := m.Called(ctx, status, overdueAt, limit)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*domain.Booking), args.Error(1)
}

func (m *MockBookingRepository) MarkBookingsExpiredBatch(ctx context.Context, bookingIDs []uuid.UUID, expectedStatus, newStatus domain.BookingStatus, updatedAt time.Time) (int, error) {
	args := m.Called(ctx, bookingIDs, expectedStatus, newStatus, updatedAt)
	return args.Int(0), args.Error(1)
}

// MockEscrowRepository
type MockEscrowRepository struct {
	mock.Mock
}

func (m *MockEscrowRepository) CreateEscrow(ctx context.Context, escrow *domain.EscrowTransaction) error {
	args := m.Called(ctx, escrow)
	return args.Error(0)
}

func (m *MockEscrowRepository) FindByBookingID(ctx context.Context, bookingID uuid.UUID) (*domain.EscrowTransaction, error) {
	args := m.Called(ctx, bookingID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.EscrowTransaction), args.Error(1)
}

func (m *MockEscrowRepository) UpdateEscrowStatus(ctx context.Context, escrowID uuid.UUID, newStatus domain.EscrowStatus) error {
	args := m.Called(ctx, escrowID, newStatus)
	return args.Error(0)
}

func (m *MockEscrowRepository) RecordRefund(ctx context.Context, refund *domain.RefundTransaction) error {
	args := m.Called(ctx, refund)
	return args.Error(0)
}

// MockPaymentGateway
type MockPaymentGateway struct {
	mock.Mock
}

func (m *MockPaymentGateway) CreatePaymentIntent(ctx context.Context, bookingID uuid.UUID, amount domain.Money) (*ports.PaymentIntentResult, error) {
	args := m.Called(ctx, bookingID, amount)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*ports.PaymentIntentResult), args.Error(1)
}

func (m *MockPaymentGateway) ConfirmPayment(ctx context.Context, gatewayRef string) (bool, error) {
	args := m.Called(ctx, gatewayRef)
	return args.Bool(0), args.Error(1)
}

func (m *MockPaymentGateway) ProcessRefund(ctx context.Context, gatewayRef string, amount domain.Money, reason string) (*ports.RefundResult, error) {
	args := m.Called(ctx, gatewayRef, amount, reason)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*ports.RefundResult), args.Error(1)
}

// MockReviewRepository
type MockReviewRepository struct {
	mock.Mock
}

func (m *MockReviewRepository) Save(ctx context.Context, review *domain.Review) error {
	args := m.Called(ctx, review)
	return args.Error(0)
}

func (m *MockReviewRepository) FindByID(ctx context.Context, id uuid.UUID) (*domain.Review, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.Review), args.Error(1)
}

func (m *MockReviewRepository) FindByBookingAndReviewer(ctx context.Context, bookingID, reviewerID uuid.UUID) (*domain.Review, error) {
	args := m.Called(ctx, bookingID, reviewerID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.Review), args.Error(1)
}

func (m *MockReviewRepository) ListByUserID(ctx context.Context, userID uuid.UUID, limit, offset int) ([]*domain.Review, error) {
	args := m.Called(ctx, userID, limit, offset)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*domain.Review), args.Error(1)
}

func (m *MockReviewRepository) GetAverageRating(ctx context.Context, userID uuid.UUID) (float64, int, error) {
	args := m.Called(ctx, userID)
	return args.Get(0).(float64), args.Int(1), args.Error(2)
}

// MockDisputeRepository
type MockDisputeRepository struct {
	mock.Mock
}

func (m *MockDisputeRepository) Save(ctx context.Context, dispute *domain.Dispute) error {
	args := m.Called(ctx, dispute)
	return args.Error(0)
}

func (m *MockDisputeRepository) FindByID(ctx context.Context, id uuid.UUID) (*domain.Dispute, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.Dispute), args.Error(1)
}

func (m *MockDisputeRepository) FindByEscrowID(ctx context.Context, escrowID uuid.UUID) (*domain.Dispute, error) {
	args := m.Called(ctx, escrowID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.Dispute), args.Error(1)
}

func (m *MockDisputeRepository) Update(ctx context.Context, dispute *domain.Dispute) error {
	args := m.Called(ctx, dispute)
	return args.Error(0)
}

func (m *MockDisputeRepository) List(ctx context.Context, limit, offset int) ([]*domain.Dispute, error) {
	args := m.Called(ctx, limit, offset)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*domain.Dispute), args.Error(1)
}

// MockReputationService
type MockReputationService struct {
	mock.Mock
}

func (m *MockReputationService) CreateReview(ctx context.Context, input ports.CreateReviewInput) (*ports.ReviewDTO, error) {
	args := m.Called(ctx, input)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*ports.ReviewDTO), args.Error(1)
}

func (m *MockReputationService) GetUserReviews(ctx context.Context, userID uuid.UUID, limit, offset int) (*ports.UserReputationDTO, error) {
	args := m.Called(ctx, userID, limit, offset)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*ports.UserReputationDTO), args.Error(1)
}

// MockDisputeService
type MockDisputeService struct {
	mock.Mock
}

func (m *MockDisputeService) OpenDispute(ctx context.Context, input ports.OpenDisputeInput) (*ports.DisputeDTO, error) {
	args := m.Called(ctx, input)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*ports.DisputeDTO), args.Error(1)
}

func (m *MockDisputeService) ResolveDispute(ctx context.Context, input ports.ResolveDisputeInput) (*ports.DisputeDTO, error) {
	args := m.Called(ctx, input)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*ports.DisputeDTO), args.Error(1)
}

func (m *MockDisputeService) GetDisputeByID(ctx context.Context, disputeID uuid.UUID) (*ports.DisputeDTO, error) {
	args := m.Called(ctx, disputeID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*ports.DisputeDTO), args.Error(1)
}

func (m *MockDisputeService) ListDisputes(ctx context.Context, limit, offset int) ([]*ports.DisputeDTO, error) {
	args := m.Called(ctx, limit, offset)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*ports.DisputeDTO), args.Error(1)
}

type mockTxCtxKey struct{}

// MockTransactionManager simula TransactionManager marcando el contexto con una transacción activa.
type MockTransactionManager struct {
	mock.Mock
}

func (m *MockTransactionManager) WithinTransaction(ctx context.Context, fn func(txCtx context.Context) error) error {
	args := m.Called(ctx)
	if err := args.Error(0); err != nil {
		return err
	}
	txCtx := context.WithValue(ctx, mockTxCtxKey{}, true)
	return fn(txCtx)
}

// MockSettlementService simula las interfaces consumidoras de liquidación en pruebas unitarias de servicios.
type MockSettlementService struct {
	mock.Mock
}

func (m *MockSettlementService) CreatePaymentIntent(ctx context.Context, bookingID uuid.UUID, amount domain.Money) (*ports.PaymentIntentResult, error) {
	args := m.Called(ctx, bookingID, amount)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*ports.PaymentIntentResult), args.Error(1)
}

func (m *MockSettlementService) ConfirmPayment(ctx context.Context, gatewayRef string) error {
	args := m.Called(ctx, gatewayRef)
	return args.Error(0)
}

func (m *MockSettlementService) HoldEscrow(ctx context.Context, input services.HoldEscrowInput) (*domain.EscrowTransaction, error) {
	args := m.Called(ctx, input)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.EscrowTransaction), args.Error(1)
}

func (m *MockSettlementService) GetEscrowByBookingID(ctx context.Context, bookingID uuid.UUID) (*domain.EscrowTransaction, error) {
	args := m.Called(ctx, bookingID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.EscrowTransaction), args.Error(1)
}

func (m *MockSettlementService) FreezeEscrowForDispute(ctx context.Context, escrow *domain.EscrowTransaction) error {
	args := m.Called(ctx, escrow)
	return args.Error(0)
}

func (m *MockSettlementService) ReleaseEscrow(ctx context.Context, escrow *domain.EscrowTransaction) error {
	args := m.Called(ctx, escrow)
	return args.Error(0)
}

func (m *MockSettlementService) RestoreEscrowHeld(ctx context.Context, escrow *domain.EscrowTransaction) error {
	args := m.Called(ctx, escrow)
	return args.Error(0)
}

func (m *MockSettlementService) SettleCancellationRefund(ctx context.Context, input services.CancellationRefundInput) (*domain.RefundTransaction, error) {
	args := m.Called(ctx, input)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.RefundTransaction), args.Error(1)
}

func (m *MockSettlementService) SettleDisputeRefund(ctx context.Context, input services.DisputeRefundInput, withinTxHooks ...func(txCtx context.Context) error) (*domain.RefundTransaction, error) {
	args := m.Called(ctx, input)
	if err := args.Error(1); err != nil {
		return nil, err
	}
	for _, hook := range withinTxHooks {
		if hook != nil {
			if err := hook(ctx); err != nil {
				return nil, err
			}
		}
	}
	if args.Get(0) == nil {
		return nil, nil
	}
	return args.Get(0).(*domain.RefundTransaction), nil
}

