package auth

import (
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
	"testing"
	"time"
)

type ValidatePasswordHashing struct {
	suite.Suite
	password string
}

func (s *ValidatePasswordHashing) SetupSuite() {
	s.password = "password"
}

func (s *ValidatePasswordHashing) TestHashPassword() {
	password := "password"
	hash, err := HashPassword(password)
	if err != nil {
		s.T().Error(err)
	}
	assert.NotEqual(s.T(), password, hash)
}

func (s *ValidatePasswordHashing) TestComparePassword() {
	password := "password"
	hash, err := HashPassword(password)
	if err != nil {
		s.T().Error(err)
	}
	err = CheckPasswordHash(password, hash)
	assert.Nil(s.T(), err)
}

func TestValidatePasswordHashing(t *testing.T) {
	suite.Run(t, new(ValidatePasswordHashing))
}

const (
	tokenSecret = "tokenSecret"
)

type ValidateJWTTestSuite struct {
	suite.Suite
	userID       uuid.UUID
	validToken   string
	expiredToken string
	expiration   time.Duration
}

func (s *ValidateJWTTestSuite) SetupSuite() {
	s.userID = uuid.New()
	s.validToken, _ = generateJWT(s.userID, tokenSecret, time.Hour)
	s.expiredToken, _ = generateJWT(s.userID, tokenSecret, -1*time.Hour)
}

func generateJWT(userID uuid.UUID, secret string, expiration time.Duration) (string, error) {
	claims := CustomClaims{
		UserID: userID,
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    "chirpy",
			IssuedAt:  jwt.NewNumericDate(time.Now().UTC()),
			ExpiresAt: jwt.NewNumericDate(time.Now().UTC().Add(expiration)),
			Subject:   userID.String(),
		},
	}
	/*
		claims := jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			Issuer:    "chirpy",
			Subject:   userID.String(),
		}
	*/
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(secret))
}
func (s *ValidateJWTTestSuite) TestValidToken() {
	UID, err := ValidateJWT(s.validToken, tokenSecret)
	assert.NoError(s.T(), err)
	assert.Equal(s.T(), s.userID, UID)
}
func (s *ValidateJWTTestSuite) TestExpiredToken() {
	_, err := ValidateJWT(s.expiredToken, tokenSecret)
	assert.Error(s.T(), err)
	assert.Equal(s.T(), "token has invalid claims: token is expired", err.Error())
}
func (s *ValidateJWTTestSuite) TestInvalidSignature() {
	wrongToken, err := generateJWT(s.userID, "wrong secret", time.Hour)
	assert.NoError(s.T(), err)
	_, err = ValidateJWT(wrongToken, tokenSecret)
	assert.Error(s.T(), err)
}
func (s *ValidateJWTTestSuite) TestInvalidClaims() {
	claims := jwt.MapClaims{
		"user_id": "invalid-uuid",
		"exp":     jwt.NewNumericDate(time.Now().Add(time.Hour)),
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signedString, err := token.SignedString([]byte(tokenSecret))
	assert.NoError(s.T(), err)

	_, err = ValidateJWT(signedString, tokenSecret)
	assert.Error(s.T(), err)
	assert.Contains(s.T(), err.Error(), "invalid")
}

func TestValidateJWT(t *testing.T) {
	suite.Run(t, new(ValidateJWTTestSuite))
}
