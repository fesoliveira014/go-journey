package handler_test

import (
	"context"
	"html/template"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	reservationv1 "github.com/fesoliveira014/library-system/gen/reservation/v1"
	"github.com/fesoliveira014/library-system/services/gateway/internal/handler"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type mockReservationClient struct {
	createFn func(ctx context.Context, in *reservationv1.CreateReservationRequest, opts ...grpc.CallOption) (*reservationv1.CreateReservationResponse, error)
	returnFn func(ctx context.Context, in *reservationv1.ReturnBookRequest, opts ...grpc.CallOption) (*reservationv1.ReturnBookResponse, error)
	getFn    func(ctx context.Context, in *reservationv1.GetReservationRequest, opts ...grpc.CallOption) (*reservationv1.Reservation, error)
	listFn   func(ctx context.Context, in *reservationv1.ListUserReservationsRequest, opts ...grpc.CallOption) (*reservationv1.ListUserReservationsResponse, error)
}

func (m *mockReservationClient) CreateReservation(ctx context.Context, in *reservationv1.CreateReservationRequest, opts ...grpc.CallOption) (*reservationv1.CreateReservationResponse, error) {
	if m.createFn != nil {
		return m.createFn(ctx, in, opts...)
	}
	return nil, status.Error(codes.Unimplemented, "not implemented")
}

func (m *mockReservationClient) ReturnBook(ctx context.Context, in *reservationv1.ReturnBookRequest, opts ...grpc.CallOption) (*reservationv1.ReturnBookResponse, error) {
	if m.returnFn != nil {
		return m.returnFn(ctx, in, opts...)
	}
	return nil, status.Error(codes.Unimplemented, "not implemented")
}

func (m *mockReservationClient) GetReservation(ctx context.Context, in *reservationv1.GetReservationRequest, opts ...grpc.CallOption) (*reservationv1.Reservation, error) {
	if m.getFn != nil {
		return m.getFn(ctx, in, opts...)
	}
	return nil, status.Error(codes.Unimplemented, "not implemented")
}

func (m *mockReservationClient) ListUserReservations(ctx context.Context, in *reservationv1.ListUserReservationsRequest, opts ...grpc.CallOption) (*reservationv1.ListUserReservationsResponse, error) {
	if m.listFn != nil {
		return m.listFn(ctx, in, opts...)
	}
	return nil, status.Error(codes.Unimplemented, "not implemented")
}

func reservationTestTemplates(t *testing.T) map[string]*template.Template {
	t.Helper()

	reservationsSet := template.Must(template.New("base.html").Parse(
		`{{range .Data.Reservations}}RES:{{.Id}}:{{.Status}} {{end}}`,
	))
	template.Must(reservationsSet.New("book_cards").Parse(
		`{{define "book_cards"}}{{end}}`,
	))

	errSet := template.Must(template.New("base.html").Parse(
		`ERROR:{{.Data.Status}}:{{.Data.Message}}`,
	))
	template.Must(errSet.New("book_cards").Parse(
		`{{define "book_cards"}}{{end}}`,
	))

	bookSet := template.Must(template.New("base.html").Parse(
		`DETAIL:{{.Data.Title}}`,
	))
	template.Must(bookSet.New("book_cards").Parse(
		`{{define "book_cards"}}{{end}}`,
	))

	return map[string]*template.Template{
		"reservations.html": reservationsSet,
		"error.html":        errSet,
		"book.html":         bookSet,
	}
}

func TestReserveBook_RequiresAuth(t *testing.T) {
	tmpl := reservationTestTemplates(t)
	srv := handler.New(nil, nil, nil, tmpl)

	req := httptest.NewRequest(http.MethodPost, "/books/123/reserve", nil)
	req.SetPathValue("id", "123")
	rec := httptest.NewRecorder()

	srv.ReserveBook(rec, req)

	if rec.Code != http.StatusSeeOther {
		t.Errorf("expected 303, got %d", rec.Code)
	}
	if loc := rec.Header().Get("Location"); loc != "/login" {
		t.Errorf("expected redirect to /login, got %q", loc)
	}
}

func TestReserveBook_Success(t *testing.T) {
	mock := &mockReservationClient{
		createFn: func(_ context.Context, in *reservationv1.CreateReservationRequest, _ ...grpc.CallOption) (*reservationv1.CreateReservationResponse, error) {
			return &reservationv1.CreateReservationResponse{
				Reservation: &reservationv1.Reservation{Id: "res-1", BookId: in.BookId},
			}, nil
		},
	}
	tmpl := reservationTestTemplates(t)
	srv := handler.New(nil, nil, mock, tmpl)

	req := httptest.NewRequest(http.MethodPost, "/books/abc/reserve", nil)
	req.SetPathValue("id", "abc")
	req = withMember(req)
	rec := httptest.NewRecorder()

	srv.ReserveBook(rec, req)

	if rec.Code != http.StatusSeeOther {
		t.Errorf("expected 303, got %d", rec.Code)
	}
	if loc := rec.Header().Get("Location"); loc != "/reservations" {
		t.Errorf("expected redirect to /reservations, got %q", loc)
	}
}

func TestReserveBook_ResourceExhausted(t *testing.T) {
	mock := &mockReservationClient{
		createFn: func(_ context.Context, _ *reservationv1.CreateReservationRequest, _ ...grpc.CallOption) (*reservationv1.CreateReservationResponse, error) {
			return nil, status.Error(codes.ResourceExhausted, "max reservations reached")
		},
	}
	tmpl := reservationTestTemplates(t)
	srv := handler.New(nil, nil, mock, tmpl)

	req := httptest.NewRequest(http.MethodPost, "/books/abc/reserve", nil)
	req.SetPathValue("id", "abc")
	req = withMember(req)
	rec := httptest.NewRecorder()

	srv.ReserveBook(rec, req)

	if rec.Code != http.StatusTooManyRequests {
		t.Errorf("expected 429, got %d", rec.Code)
	}
}

func TestReserveBook_FailedPrecondition(t *testing.T) {
	mock := &mockReservationClient{
		createFn: func(_ context.Context, _ *reservationv1.CreateReservationRequest, _ ...grpc.CallOption) (*reservationv1.CreateReservationResponse, error) {
			return nil, status.Error(codes.FailedPrecondition, "no copies available")
		},
	}
	tmpl := reservationTestTemplates(t)
	srv := handler.New(nil, nil, mock, tmpl)

	req := httptest.NewRequest(http.MethodPost, "/books/abc/reserve", nil)
	req.SetPathValue("id", "abc")
	req = withMember(req)
	rec := httptest.NewRecorder()

	srv.ReserveBook(rec, req)

	if rec.Code != http.StatusPreconditionFailed {
		t.Errorf("expected 412, got %d", rec.Code)
	}
}

func TestMyReservations_Success(t *testing.T) {
	mock := &mockReservationClient{
		listFn: func(_ context.Context, _ *reservationv1.ListUserReservationsRequest, _ ...grpc.CallOption) (*reservationv1.ListUserReservationsResponse, error) {
			return &reservationv1.ListUserReservationsResponse{
				Reservations: []*reservationv1.Reservation{
					{Id: "r1", Status: "active", ReservedAt: timestamppb.Now(), DueAt: timestamppb.Now()},
					{Id: "r2", Status: "returned", ReservedAt: timestamppb.Now(), DueAt: timestamppb.Now()},
				},
			}, nil
		},
	}
	tmpl := reservationTestTemplates(t)
	srv := handler.New(nil, nil, mock, tmpl)

	req := httptest.NewRequest(http.MethodGet, "/reservations", nil)
	req = withMember(req)
	rec := httptest.NewRecorder()

	srv.MyReservations(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rec.Code)
	}
	body := rec.Body.String()
	if !strings.Contains(body, "RES:r1:active") {
		t.Errorf("expected active reservation, got %q", body)
	}
}

func TestReturnBook_Success(t *testing.T) {
	mock := &mockReservationClient{
		returnFn: func(_ context.Context, in *reservationv1.ReturnBookRequest, _ ...grpc.CallOption) (*reservationv1.ReturnBookResponse, error) {
			return &reservationv1.ReturnBookResponse{
				Reservation: &reservationv1.Reservation{Id: in.ReservationId, Status: "returned"},
			}, nil
		},
	}
	tmpl := reservationTestTemplates(t)
	srv := handler.New(nil, nil, mock, tmpl)

	req := httptest.NewRequest(http.MethodPost, "/reservations/r1/return", nil)
	req.SetPathValue("id", "r1")
	req = withMember(req)
	rec := httptest.NewRecorder()

	srv.ReturnBook(rec, req)

	if rec.Code != http.StatusSeeOther {
		t.Errorf("expected 303, got %d", rec.Code)
	}
	if loc := rec.Header().Get("Location"); loc != "/reservations" {
		t.Errorf("expected redirect to /reservations, got %q", loc)
	}
}
