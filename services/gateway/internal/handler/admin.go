package handler

import (
	"net/http"

	authv1 "github.com/fesoliveira014/library-system/gen/auth/v1"
	reservationv1 "github.com/fesoliveira014/library-system/gen/reservation/v1"
)

func (s *Server) AdminDashboard(w http.ResponseWriter, r *http.Request) {
	if !s.requireAdmin(w, r) {
		return
	}
	s.render(w, r, "admin_dashboard.html", nil)
}

func (s *Server) AdminUserList(w http.ResponseWriter, r *http.Request) {
	if !s.requireAdmin(w, r) {
		return
	}
	resp, err := s.auth.ListUsers(r.Context(), &authv1.ListUsersRequest{})
	if err != nil {
		s.handleGRPCError(w, r, err, "Failed to load users")
		return
	}
	s.render(w, r, "admin_users.html", map[string]any{
		"Users": resp.Users,
	})
}

func (s *Server) AdminReservationList(w http.ResponseWriter, r *http.Request) {
	if !s.requireAdmin(w, r) {
		return
	}
	resp, err := s.reservation.ListAllReservations(r.Context(), &reservationv1.ListAllReservationsRequest{})
	if err != nil {
		s.handleGRPCError(w, r, err, "Failed to load reservations")
		return
	}
	s.render(w, r, "admin_reservations.html", map[string]any{
		"Reservations": resp.Reservations,
	})
}
