package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	userv1 "github.com/dmehra2102/prod-golang-projects/buildmart/gen/go/user/v1"
	"github.com/gogo/protobuf/proto"
	"github.com/google/uuid"
	"google.golang.org/protobuf/types/known/timestamppb"
)

var (
	ErrNotFound      = errors.New("not found")
	ErrAlreadyExists = errors.New("already exists")
	ErrInvalidInput  = errors.New("invalid input")
)

type UserRepository interface {
	Create(ctx context.Context, user *userv1.User, hashedPassword string) (*userv1.User, error)
	GetByID(ctx context.Context, id string) (*userv1.User, error)
	GetByEmail(ctx context.Context, email string) (*userv1.User, string, error)
	Update(ctx context.Context, user *userv1.User, fields []string) (*userv1.User, error)
	Delete(ctx context.Context, id string, hard bool) error
	List(ctx context.Context, opts ListOptions) ([]*userv1.User, string, int64, error)
}

type ListOptions struct {
	PageToken string
	PageSize  int32
	Status    userv1.UserStatus
	Role      userv1.UserRole
	OrderBy   string
}

type PostgresUserRepository struct {
	db *sql.DB
}

func NewPostgresUserRepository(db *sql.DB) *PostgresUserRepository {
	return &PostgresUserRepository{db: db}
}

func (r *PostgresUserRepository) Create(ctx context.Context, user *userv1.User, hashedPassword string) (*userv1.User, error) {
	id := uuid.New().String()
	now := time.Now().UTC()

	query := `
		INSERT INTO users (id, email, first_name, last_name, phone, status, role, hashed_password, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
		RETURNING id, created_at, updated_at
	`

	var (
		returnedID        string
		returnedCreatedAt time.Time
		returnedUpdatedAt time.Time
	)

	err := r.db.QueryRowContext(ctx, query,
		id,
		user.Email,
		user.FirstName,
		user.LastName,
		user.Phone,
		user.Status.String(),
		user.Role.String(),
		hashedPassword,
		now,
		now,
	).Scan(&returnedID, &returnedCreatedAt, &returnedUpdatedAt)

	if err != nil {
		// PRODUCTION RULE
		// if isUnqiueViolation(err) {
		// 	return nil, fmt.Errorf("%w: email %s", ErrAlreadyExists, user.Email)
		// }
		return nil, fmt.Errorf("create user: %w", err)
	}

	result := proto.Clone(user).(*userv1.User)
	result.Id = returnedID
	result.CreatedAt = timestamppb.New(returnedCreatedAt)
	result.UpdatedAt = timestamppb.New(returnedUpdatedAt)
	result.Status = userv1.UserStatus_USER_STATUS_ACTIVE

	return result, nil
}
