package service

import (
	"context"
	"errors"

	commonv1 "github.com/dmehra2102/prod-golang-projects/buildmart/gen/go/common/v1"
	userv1 "github.com/dmehra2102/prod-golang-projects/buildmart/gen/go/user/v1"
	"github.com/dmehra2102/prod-golang-projects/buildmart/services/user-service/internal/repository"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type UserService struct {
	userv1.UnimplementedUserServiceServer
	repo repository.UserRepository
}

func NewUserService(repo repository.UserRepository) *UserService {
	return &UserService{repo: repo}
}

func (s *UserService) CreateUser(ctx context.Context, req *userv1.CreateUserRequest) (*userv1.CreateUserResponse, error) {

}

func mapRepoError(err error, resource, identifier string) error {
	switch {
	case errors.Is(err, repository.ErrNotFound):
		return status.Errorf(codes.NotFound, "%s %q not found", resource, identifier)

	case errors.Is(err, repository.ErrAlreadyExists):
		return status.Errorf(codes.AlreadyExists, "%s %q already exists", resource, identifier)

	case errors.Is(err, repository.ErrInvalidInput):
		return status.Errorf(codes.InvalidArgument, "invalid %s: %s", resource, identifier)

	default:
		return status.Errorf(codes.Internal, "internal error processing %s", resource)
	}
}

func newValidationError(violations []*commonv1.FieldViolation) error {
	detail := &commonv1.ErrorDetail{
		ErrorCode:       "VALIDATION_ERROR",
		FieldViolations: violations,
	}

	st, err := status.New(codes.InvalidArgument, "request validation failed").WithDetails(detail)
	if err != nil {
		return status.Error(codes.InvalidArgument, "request invalidation failed")
	}

	return st.Err()
}

func validateCreateUserRequest(req *userv1.CreateUserRequest) error {
	var violations []*commonv1.FieldViolation

	if req.Email == "" {
		violations = append(violations, &commonv1.FieldViolation{
			Field:       "email",
			Description: "email is required",
		})
	}
}
