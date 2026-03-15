// P1 Test Fixture: Go Sample Code
// Purpose: Test Go parsing regression for gdc sync --direction code
// Requirements: R4 (AC-R4-1, AC-R4-2) - Existing Go behavior preservation

package fixtures

import (
	"context"
	"errors"
	"time"
)

// User represents a system user
type User struct {
	ID        string    `json:"id"`
	Email     string    `json:"email"`
	Name      string    `json:"name"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// UserRepository defines the interface for user data access
type UserRepository interface {
	// FindByID retrieves a user by their ID
	FindByID(ctx context.Context, id string) (*User, error)

	// FindByEmail retrieves a user by their email address
	FindByEmail(ctx context.Context, email string) (*User, error)

	// Create creates a new user
	Create(ctx context.Context, user *User) error

	// Update updates an existing user
	Update(ctx context.Context, user *User) error

	// Delete removes a user by ID
	Delete(ctx context.Context, id string) error
}

// AuthService handles authentication logic
type AuthService struct {
	userRepo UserRepository
	tokenSvc TokenService
	logger   Logger
}

// NewAuthService creates a new AuthService instance
func NewAuthService(
	userRepo UserRepository,
	tokenSvc TokenService,
	logger Logger,
) *AuthService {
	return &AuthService{
		userRepo: userRepo,
		tokenSvc: tokenSvc,
		logger:   logger,
	}
}

// Login authenticates a user and returns a token
// Returns ErrInvalidCredentials if authentication fails
func (s *AuthService) Login(ctx context.Context, email, password string) (string, error) {
	s.logger.Info("attempting login", "email", email)

	user, err := s.userRepo.FindByEmail(ctx, email)
	if err != nil {
		s.logger.Error("user lookup failed", "error", err)
		return "", ErrInvalidCredentials
	}

	if !s.verifyPassword(password, user) {
		s.logger.Warn("invalid password", "email", email)
		return "", ErrInvalidCredentials
	}

	token, err := s.tokenSvc.Generate(user.ID)
	if err != nil {
		s.logger.Error("token generation failed", "error", err)
		return "", err
	}

	s.logger.Info("login successful", "user_id", user.ID)
	return token, nil
}

// Logout invalidates a user's token
func (s *AuthService) Logout(ctx context.Context, token string) error {
	s.logger.Info("logging out user")
	return s.tokenSvc.Invalidate(token)
}

// ValidateToken checks if a token is valid and returns the user ID
func (s *AuthService) ValidateToken(ctx context.Context, token string) (string, error) {
	userID, err := s.tokenSvc.Validate(token)
	if err != nil {
		return "", ErrInvalidToken
	}
	return userID, nil
}

// verifyPassword checks if the provided password matches the user's password
func (s *AuthService) verifyPassword(password string, user *User) bool {
	// Simplified for testing - in real code, use bcrypt or similar
	return password != ""
}

// TokenService defines the interface for token operations
type TokenService interface {
	Generate(userID string) (string, error)
	Validate(token string) (string, error)
	Invalidate(token string) error
}

// Logger defines the interface for logging
type Logger interface {
	Info(msg string, keysAndValues ...interface{})
	Error(msg string, keysAndValues ...interface{})
	Warn(msg string, keysAndValues ...interface{})
}

// OrderService handles order operations
type OrderService struct {
	authSvc   *AuthService
	orderRepo OrderRepository
	logger    Logger
}

// NewOrderService creates a new OrderService
func NewOrderService(
	authSvc *AuthService,
	orderRepo OrderRepository,
	logger Logger,
) *OrderService {
	return &OrderService{
		authSvc:   authSvc,
		orderRepo: orderRepo,
		logger:    logger,
	}
}

// CreateOrder creates a new order for a user
func (s *OrderService) CreateOrder(ctx context.Context, userID string, items []OrderItem) (*Order, error) {
	order := &Order{
		ID:        generateID(),
		UserID:    userID,
		Items:     items,
		Status:    OrderStatusPending,
		CreatedAt: time.Now(),
	}

	if err := s.orderRepo.Create(ctx, order); err != nil {
		s.logger.Error("failed to create order", "error", err)
		return nil, err
	}

	s.logger.Info("order created", "order_id", order.ID, "user_id", userID)
	return order, nil
}

// GetOrder retrieves an order by ID
func (s *OrderService) GetOrder(ctx context.Context, orderID string) (*Order, error) {
	return s.orderRepo.FindByID(ctx, orderID)
}

// Order represents an order in the system
type Order struct {
	ID        string
	UserID    string
	Items     []OrderItem
	Status    OrderStatus
	Total     float64
	CreatedAt time.Time
}

// OrderItem represents an item in an order
type OrderItem struct {
	ProductID string
	Quantity  int
	UnitPrice float64
}

// OrderStatus represents the status of an order
type OrderStatus int

const (
	OrderStatusPending OrderStatus = iota
	OrderStatusProcessing
	OrderStatusShipped
	OrderStatusDelivered
	OrderStatusCancelled
)

// OrderRepository defines the interface for order data access
type OrderRepository interface {
	FindByID(ctx context.Context, id string) (*Order, error)
	Create(ctx context.Context, order *Order) error
	Update(ctx context.Context, order *Order) error
	Delete(ctx context.Context, id string) error
}

// Common errors
var (
	ErrInvalidCredentials = errors.New("invalid credentials")
	ErrInvalidToken       = errors.New("invalid token")
	ErrUserNotFound       = errors.New("user not found")
	ErrOrderNotFound      = errors.New("order not found")
)

// generateID generates a unique ID (simplified for testing)
func generateID() string {
	return time.Now().Format("20060102150405")
}
