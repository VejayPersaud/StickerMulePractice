package main

import (
	"database/sql"
	"fmt"
	"log/slog"
	"time"
	
	"golang.org/x/crypto/bcrypt"
	"github.com/golang-jwt/jwt/v5"
)

//User represents a user in the system
type User struct {
	ID           int       `json:"id"`
	Email        string    `json:"email"`
	PasswordHash string    `json:"-"` //"-" means never include in JSON output
	CreatedAt    time.Time `json:"created_at"`
}

//AuthService handles authentication logic
type AuthService struct {
	db     *sql.DB
	logger *slog.Logger
	jwtSecret string //Secret key for signing JWT tokens
}

//NewAuthService creates a new auth service
func NewAuthService(db *sql.DB, logger *slog.Logger, jwtSecret string) *AuthService {
	return &AuthService{
		db:        db,
		logger:    logger,
		jwtSecret: jwtSecret,
	}
}

//Register creates a new user account
func (a *AuthService) Register(email, password string) (*User, string, error) {
	// 1. Validate input
	if email == "" || password == "" {
		return nil, "", fmt.Errorf("email and password are required")
	}
	
	if len(password) < 8 {
		return nil, "", fmt.Errorf("password must be at least 8 characters")
	}
	
	a.logger.Info("registering new user", "email", email)
	
	// 2. Hash the password using bcrypt
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		a.logger.Error("failed to hash password", "error", err.Error())
		return nil, "", fmt.Errorf("failed to hash password")
	}
	
	// 3. Insert user into database
	var userID int
	query := "INSERT INTO users (email, password_hash) VALUES ($1, $2) RETURNING id"
	err = a.db.QueryRow(query, email, string(hashedPassword)).Scan(&userID)
	
	if err != nil {
		a.logger.Error("failed to create user", "email", email, "error", err.Error())
		//Check if it's a duplicate email error
		if err.Error() == `pq: duplicate key value violates unique constraint "users_email_key"` {
			return nil, "", fmt.Errorf("email already registered")
		}
		return nil, "", fmt.Errorf("failed to create user")
	}
	
	a.logger.Info("user created successfully", "user_id", userID, "email", email)
	
	// 4. Create the user object
	user := &User{
		ID:    userID,
		Email: email,
	}
	
	// 5. Generate JWT token
	token, err := a.generateToken(user)
	if err != nil {
		a.logger.Error("failed to generate token", "user_id", userID, "error", err.Error())
		return nil, "", fmt.Errorf("failed to generate token")
	}
	
	return user, token, nil
}

//generateToken creates a JWT token for a user
func (a *AuthService) generateToken(user *User) (string, error) {
	//Create claims (the data we put inside the token)
	claims := jwt.MapClaims{
		"user_id": user.ID,
		"email":   user.Email,
		"exp":     time.Now().Add(24 * time.Hour).Unix(), //Token expires in 24 hours
	}
	
	//Create token with claims
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	
	//Sign the token with our secret key
	signedToken, err := token.SignedString([]byte(a.jwtSecret))
	if err != nil {
		return "", err
	}
	
	return signedToken, nil
}