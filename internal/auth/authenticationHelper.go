package auth

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
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
	decoded, parseErr := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return []byte(tokenSecret), nil
	})
	if parseErr != nil {
		return uuid.Nil, parseErr
	}
	issuer, issuerErr := decoded.Claims.GetIssuer()
	if issuerErr != nil {
		return [16]byte{}, issuerErr
	}
	subject, subjectErr := decoded.Claims.GetSubject()
	if subjectErr != nil {
		return [16]byte{}, subjectErr
	}
	expirationTime, expErr := decoded.Claims.GetExpirationTime()
	if expErr != nil {
		return [16]byte{}, expErr
	}
	at, atErr := decoded.Claims.GetIssuedAt()
	if atErr != nil {
		return [16]byte{}, atErr
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

func MakeRefreshToken() (string, error) {
	bytes := make([]byte, 32)
	_, err := rand.Read(bytes)
	if err != nil {
		return "", err
	}
	refreshToken := hex.EncodeToString(bytes)
	return refreshToken, nil
}
