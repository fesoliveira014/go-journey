package repository

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/fesoliveira014/library-system/services/reservation/internal/model"
)

type ReservationRepository struct {
	db *gorm.DB
}

func NewReservationRepository(db *gorm.DB) *ReservationRepository {
	return &ReservationRepository{db: db}
}

func (r *ReservationRepository) Create(ctx context.Context, res *model.Reservation) (*model.Reservation, error) {
	if err := r.db.WithContext(ctx).Create(res).Error; err != nil {
		return nil, err
	}
	return res, nil
}

func (r *ReservationRepository) GetByID(ctx context.Context, id uuid.UUID) (*model.Reservation, error) {
	var res model.Reservation
	if err := r.db.WithContext(ctx).First(&res, "id = ?", id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, model.ErrReservationNotFound
		}
		return nil, err
	}
	return &res, nil
}

func (r *ReservationRepository) CountActive(ctx context.Context, userID uuid.UUID) (int64, error) {
	var count int64
	err := r.db.WithContext(ctx).Model(&model.Reservation{}).
		Where("user_id = ? AND status = ?", userID, model.StatusActive).
		Count(&count).Error
	return count, err
}

func (r *ReservationRepository) ListByUser(ctx context.Context, userID uuid.UUID) ([]*model.Reservation, error) {
	var reservations []*model.Reservation
	err := r.db.WithContext(ctx).
		Where("user_id = ?", userID).
		Order("reserved_at DESC").
		Find(&reservations).Error
	return reservations, err
}

func (r *ReservationRepository) ListAll(ctx context.Context) ([]*model.Reservation, error) {
	var reservations []*model.Reservation
	if err := r.db.WithContext(ctx).Order("reserved_at DESC").Find(&reservations).Error; err != nil {
		return nil, err
	}
	return reservations, nil
}

func (r *ReservationRepository) Update(ctx context.Context, res *model.Reservation) (*model.Reservation, error) {
	if err := r.db.WithContext(ctx).Save(res).Error; err != nil {
		return nil, err
	}
	return res, nil
}

// ListDueForExpiration returns active reservations whose due_at has passed
// the given cutoff. Used by the expiration reaper to find rows that need to
// flip to 'expired' even when no user has looked at them recently.
func (r *ReservationRepository) ListDueForExpiration(ctx context.Context, now time.Time) ([]*model.Reservation, error) {
	var reservations []*model.Reservation
	err := r.db.WithContext(ctx).
		Where("status = ? AND due_at < ?", model.StatusActive, now).
		Find(&reservations).Error
	return reservations, err
}
