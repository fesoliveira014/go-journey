package handler

import (
	"net/http"

	reservationv1 "github.com/fesoliveira014/library-system/gen/reservation/v1"
)

func (s *Server) ReserveBook(w http.ResponseWriter, r *http.Request) {
	if !s.requireAuth(w, r) {
		return
	}

	bookID := r.PathValue("id")
	_, err := s.reservation.CreateReservation(r.Context(), &reservationv1.CreateReservationRequest{
		BookId: bookID,
	})
	if err != nil {
		s.handleGRPCError(w, r, err, "Failed to reserve book")
		return
	}

	s.setFlash(w, "Book reserved successfully")
	http.Redirect(w, r, "/reservations", http.StatusSeeOther)
}

func (s *Server) ReturnBook(w http.ResponseWriter, r *http.Request) {
	if !s.requireAuth(w, r) {
		return
	}

	resID := r.PathValue("id")
	_, err := s.reservation.ReturnBook(r.Context(), &reservationv1.ReturnBookRequest{
		ReservationId: resID,
	})
	if err != nil {
		s.handleGRPCError(w, r, err, "Failed to return book")
		return
	}

	s.setFlash(w, "Book returned successfully")
	http.Redirect(w, r, "/reservations", http.StatusSeeOther)
}

func (s *Server) MyReservations(w http.ResponseWriter, r *http.Request) {
	if !s.requireAuth(w, r) {
		return
	}

	resp, err := s.reservation.ListUserReservations(r.Context(), &reservationv1.ListUserReservationsRequest{})
	if err != nil {
		s.handleGRPCError(w, r, err, "Failed to load reservations")
		return
	}

	s.render(w, r, "reservations.html", map[string]any{
		"Reservations": resp.Reservations,
	})
}
