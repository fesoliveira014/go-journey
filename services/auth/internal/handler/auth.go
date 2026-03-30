package handler

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"

	authv1 "github.com/fesoliveira014/library-system/gen/auth/v1"
	"github.com/fesoliveira014/library-system/services/auth/internal/model"
	"github.com/fesoliveira014/library-system/services/auth/internal/service"
)

// AuthHandler implements the generated authv1.AuthServiceServer interface.
type AuthHandler struct {
	authv1.UnimplementedAuthServiceServer
	svc         *service.AuthService
	oauthConfig *oauth2.Config
	states      map[string]time.Time
	mu          sync.Mutex
}

// NewAuthHandler creates a new gRPC handler backed by the given service.
func NewAuthHandler(svc *service.AuthService) *AuthHandler {
	return &AuthHandler{
		svc:    svc,
		states: make(map[string]time.Time),
	}
}

// NewAuthHandlerWithOAuth creates a handler with OAuth2 configuration.
func NewAuthHandlerWithOAuth(svc *service.AuthService, clientID, clientSecret, redirectURL string) *AuthHandler {
	h := NewAuthHandler(svc)
	if clientID != "" {
		h.oauthConfig = &oauth2.Config{
			ClientID:     clientID,
			ClientSecret: clientSecret,
			RedirectURL:  redirectURL,
			Scopes:       []string{"openid", "email", "profile"},
			Endpoint:     google.Endpoint,
		}
	}
	return h
}

func (h *AuthHandler) Register(ctx context.Context, req *authv1.RegisterRequest) (*authv1.AuthResponse, error) {
	if req.GetEmail() == "" {
		return nil, status.Error(codes.InvalidArgument, "email is required")
	}
	if req.GetPassword() == "" {
		return nil, status.Error(codes.InvalidArgument, "password is required")
	}
	if req.GetName() == "" {
		return nil, status.Error(codes.InvalidArgument, "name is required")
	}

	token, user, err := h.svc.Register(ctx, req.GetEmail(), req.GetPassword(), req.GetName())
	if err != nil {
		return nil, toGRPCError(err)
	}

	return &authv1.AuthResponse{
		Token: token,
		User:  userToProto(user),
	}, nil
}

func (h *AuthHandler) Login(ctx context.Context, req *authv1.LoginRequest) (*authv1.AuthResponse, error) {
	if req.GetEmail() == "" {
		return nil, status.Error(codes.InvalidArgument, "email is required")
	}
	if req.GetPassword() == "" {
		return nil, status.Error(codes.InvalidArgument, "password is required")
	}

	token, user, err := h.svc.Login(ctx, req.GetEmail(), req.GetPassword())
	if err != nil {
		return nil, toGRPCError(err)
	}

	return &authv1.AuthResponse{
		Token: token,
		User:  userToProto(user),
	}, nil
}

func (h *AuthHandler) ValidateToken(ctx context.Context, req *authv1.ValidateTokenRequest) (*authv1.ValidateTokenResponse, error) {
	if req.GetToken() == "" {
		return nil, status.Error(codes.InvalidArgument, "token is required")
	}

	userID, role, err := h.svc.ValidateToken(ctx, req.GetToken())
	if err != nil {
		return nil, toGRPCError(err)
	}

	return &authv1.ValidateTokenResponse{
		UserId: userID.String(),
		Role:   role,
	}, nil
}

func (h *AuthHandler) GetUser(ctx context.Context, req *authv1.GetUserRequest) (*authv1.User, error) {
	id, err := uuid.Parse(req.GetId())
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid user ID")
	}

	user, err := h.svc.GetUser(ctx, id)
	if err != nil {
		return nil, toGRPCError(err)
	}

	return userToProto(user), nil
}

func (h *AuthHandler) InitOAuth2(ctx context.Context, req *authv1.InitOAuth2Request) (*authv1.InitOAuth2Response, error) {
	if h.oauthConfig == nil {
		return nil, status.Error(codes.Unavailable, "OAuth2 not configured")
	}

	stateBytes := make([]byte, 16)
	if _, err := rand.Read(stateBytes); err != nil {
		return nil, status.Error(codes.Internal, "failed to generate state")
	}
	state := hex.EncodeToString(stateBytes)

	h.mu.Lock()
	h.states[state] = time.Now().Add(5 * time.Minute)
	now := time.Now()
	for k, v := range h.states {
		if now.After(v) {
			delete(h.states, k)
		}
	}
	h.mu.Unlock()

	url := h.oauthConfig.AuthCodeURL(state)
	return &authv1.InitOAuth2Response{RedirectUrl: url}, nil
}

func (h *AuthHandler) CompleteOAuth2(ctx context.Context, req *authv1.CompleteOAuth2Request) (*authv1.AuthResponse, error) {
	if h.oauthConfig == nil {
		return nil, status.Error(codes.Unavailable, "OAuth2 not configured")
	}
	if req.GetCode() == "" {
		return nil, status.Error(codes.InvalidArgument, "code is required")
	}
	if req.GetState() == "" {
		return nil, status.Error(codes.InvalidArgument, "state is required")
	}

	h.mu.Lock()
	expiry, ok := h.states[req.GetState()]
	if ok {
		delete(h.states, req.GetState())
	}
	h.mu.Unlock()

	if !ok || time.Now().After(expiry) {
		return nil, status.Error(codes.InvalidArgument, "invalid or expired state")
	}

	oauthToken, err := h.oauthConfig.Exchange(ctx, req.GetCode())
	if err != nil {
		return nil, status.Error(codes.Internal, fmt.Sprintf("failed to exchange code: %v", err))
	}

	client := h.oauthConfig.Client(ctx, oauthToken)
	resp, err := client.Get("https://www.googleapis.com/oauth2/v2/userinfo")
	if err != nil {
		return nil, status.Error(codes.Internal, "failed to fetch user info")
	}
	defer resp.Body.Close()

	var googleUser struct {
		ID    string `json:"id"`
		Email string `json:"email"`
		Name  string `json:"name"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&googleUser); err != nil {
		return nil, status.Error(codes.Internal, "failed to parse user info")
	}

	token, user, err := h.svc.FindOrCreateOAuthUser(ctx, "google", googleUser.ID, googleUser.Email, googleUser.Name)
	if err != nil {
		return nil, toGRPCError(err)
	}

	return &authv1.AuthResponse{
		Token: token,
		User:  userToProto(user),
	}, nil
}

func userToProto(u *model.User) *authv1.User {
	return &authv1.User{
		Id:        u.ID.String(),
		Email:     u.Email,
		Name:      u.Name,
		Role:      u.Role,
		CreatedAt: timestamppb.New(u.CreatedAt),
		UpdatedAt: timestamppb.New(u.UpdatedAt),
	}
}

func toGRPCError(err error) error {
	switch {
	case errors.Is(err, model.ErrUserNotFound):
		return status.Error(codes.NotFound, err.Error())
	case errors.Is(err, model.ErrDuplicateEmail):
		return status.Error(codes.AlreadyExists, err.Error())
	case errors.Is(err, model.ErrInvalidCredentials):
		return status.Error(codes.Unauthenticated, err.Error())
	case errors.Is(err, model.ErrInvalidToken):
		return status.Error(codes.Unauthenticated, err.Error())
	case errors.Is(err, model.ErrTokenExpired):
		return status.Error(codes.Unauthenticated, err.Error())
	case errors.Is(err, model.ErrOAuthFailed):
		return status.Error(codes.Internal, err.Error())
	default:
		return status.Error(codes.Internal, "internal error")
	}
}
