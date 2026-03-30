package handler

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"

	reservationv1 "github.com/fesoliveira014/library-system/gen/reservation/v1"
	"github.com/fesoliveira014/library-system/services/reservation/internal/model"
)

type Service interface {
	CreateReservation(ctx context.Context, bookID uuid.UUID) (*model.Reservation, error)
	ReturnBook(ctx context.Context, reservationID uuid.UUID) (*model.Reservation, error)
	GetReservation(ctx context.Context, reservationID uuid.UUID) (*model.Reservation, error)
	ListUserReservations(ctx context.Context) ([]*model.Reservation, error)
}

type ReservationHandler struct {
	reservationv1.UnimplementedReservationServiceServer
	svc Service
}

func NewReservationHandler(svc Service) *ReservationHandler {
	return &ReservationHandler{svc: svc}
}

func (h *ReservationHandler) CreateReservation(ctx context.Context, req *reservationv1.CreateReservationRequest) (*reservationv1.CreateReservationResponse, error) {
	bookID, err := uuid.Parse(req.GetBookId())
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid book ID")
	}

	res, err := h.svc.CreateReservation(ctx, bookID)
	if err != nil {
		return nil, toGRPCError(err)
	}
	return &reservationv1.CreateReservationResponse{
		Reservation: reservationToProto(res),
	}, nil
}

func (h *ReservationHandler) ReturnBook(ctx context.Context, req *reservationv1.ReturnBookRequest) (*reservationv1.ReturnBookResponse, error) {
	resID, err := uuid.Parse(req.GetReservationId())
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid reservation ID")
	}

	res, err := h.svc.ReturnBook(ctx, resID)
	if err != nil {
		return nil, toGRPCError(err)
	}
	return &reservationv1.ReturnBookResponse{
		Reservation: reservationToProto(res),
	}, nil
}

func (h *ReservationHandler) GetReservation(ctx context.Context, req *reservationv1.GetReservationRequest) (*reservationv1.Reservation, error) {
	resID, err := uuid.Parse(req.GetReservationId())
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid reservation ID")
	}

	res, err := h.svc.GetReservation(ctx, resID)
	if err != nil {
		return nil, toGRPCError(err)
	}
	return reservationToProto(res), nil
}

func (h *ReservationHandler) ListUserReservations(ctx context.Context, _ *reservationv1.ListUserReservationsRequest) (*reservationv1.ListUserReservationsResponse, error) {
	list, err := h.svc.ListUserReservations(ctx)
	if err != nil {
		return nil, toGRPCError(err)
	}

	protos := make([]*reservationv1.Reservation, len(list))
	for i, r := range list {
		protos[i] = reservationToProto(r)
	}
	return &reservationv1.ListUserReservationsResponse{Reservations: protos}, nil
}

func reservationToProto(r *model.Reservation) *reservationv1.Reservation {
	pb := &reservationv1.Reservation{
		Id:         r.ID.String(),
		UserId:     r.UserID.String(),
		BookId:     r.BookID.String(),
		Status:     r.Status,
		ReservedAt: timestamppb.New(r.ReservedAt),
		DueAt:      timestamppb.New(r.DueAt),
	}
	if r.ReturnedAt != nil {
		pb.ReturnedAt = timestamppb.New(*r.ReturnedAt)
	}
	return pb
}

func toGRPCError(err error) error {
	switch {
	case errors.Is(err, model.ErrReservationNotFound):
		return status.Error(codes.NotFound, err.Error())
	case errors.Is(err, model.ErrMaxReservations):
		return status.Error(codes.ResourceExhausted, err.Error())
	case errors.Is(err, model.ErrNoAvailableCopies):
		return status.Error(codes.FailedPrecondition, "no copies available")
	case errors.Is(err, model.ErrAlreadyReturned):
		return status.Error(codes.FailedPrecondition, err.Error())
	default:
		if err.Error() == "permission denied" {
			return status.Error(codes.PermissionDenied, "permission denied")
		}
		return status.Error(codes.Internal, "internal error")
	}
}
