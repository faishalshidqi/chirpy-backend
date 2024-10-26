package auth

import (
	"errors"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
	"net/http"
	"strings"
	"time"
)

func HashPassword(password string) (string, error) {
	hashed, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return "", err
	}
	return string(hashed), nil
}

func CheckPasswordHash(password, hash string) error {
	err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(password))
	if err != nil {
		return err
	}
	return nil
}

func MakeJWT(userID uuid.UUID, tokenSecret string, expiresIn time.Duration) (string, error) {
	token := jwt.NewWithClaims(
		jwt.SigningMethodHS256,
		jwt.RegisteredClaims{
			Issuer:    "chirpy",
			IssuedAt:  jwt.NewNumericDate(time.Now().UTC()),
			ExpiresAt: jwt.NewNumericDate(time.Now().UTC().Add(expiresIn * time.Second)),
			Subject:   userID.String(),
		},
	)
	signedString, err := token.SignedString([]byte(tokenSecret))
	if err != nil {
		return "", err
	}
	return signedString, nil
}

func ValidateJWT(tokenString, tokenSecret string) (uuid.UUID, error) {
	decoded, _ := jwt.Parse(tokenString, nil)
	issuer, err := decoded.Claims.GetIssuer()
	subject, err := decoded.Claims.GetSubject()
	expirationTime, err := decoded.Claims.GetExpirationTime()
	at, err := decoded.Claims.GetIssuedAt()
	if err != nil {
		return [16]byte{}, err
	}
	claims := &jwt.RegisteredClaims{
		Issuer:    issuer,
		IssuedAt:  at,
		ExpiresAt: expirationTime,
		Subject:   subject,
	}
	token, err := jwt.ParseWithClaims(tokenString, claims, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, errors.New("unexpected signing method")
		}
		return []byte(tokenSecret), nil
	})
	if err != nil {
		return uuid.Nil, err
	}
	if !token.Valid {
		return uuid.Nil, err
	}
	if claims.ExpiresAt.Before(time.Now().UTC()) {
		return uuid.Nil, errors.New("token expired")
	}
	parsed, err := uuid.Parse(subject)
	if err != nil {
		return [16]byte{}, err
	}
	return parsed, nil
}

func GetBearerToken(header http.Header) (string, error) {
	authHeader := header.Get("Authorization")
	if authHeader == "" {
		return "", errors.New("missing Authorization header")
	}
	return strings.Split(authHeader, " ")[1], nil
}
