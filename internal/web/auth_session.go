package web

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/labstack/echo/v4"
)

const (
	sessionCookieName = "spond_webcal_session"
	sessionLifetime   = 14 * 24 * time.Hour
)

type authSession struct {
	Token      string   `json:"token"`
	Name       string   `json:"name"`
	Email      string   `json:"email"`
	ProfileID  string   `json:"profileId"`
	GroupCount int      `json:"groupCount"`
	ActorIDs   []string `json:"actorIds"`
}

func (s *Server) sessionSecret() string {
	return s.cfg.CookieSecret
}

func (s *Server) readSession(c echo.Context) (authSession, bool) {
	cookie, err := c.Cookie(sessionCookieName)
	if err != nil {
		return authSession{}, false
	}

	session, err := decodeSession(s.sessionSecret(), cookie.Value)
	if err != nil {
		return authSession{}, false
	}

	if strings.TrimSpace(session.Token) == "" {
		return authSession{}, false
	}

	return session, true
}

func (s *Server) writeSession(c echo.Context, session authSession) error {
	value, err := encodeSession(s.sessionSecret(), session)
	if err != nil {
		return err
	}

	cookie := &http.Cookie{
		Name:     sessionCookieName,
		Value:    value,
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		Secure:   c.Request().TLS != nil,
		MaxAge:   int(sessionLifetime.Seconds()),
	}
	c.SetCookie(cookie)
	return nil
}

func (s *Server) clearSession(c echo.Context) {
	cookie := &http.Cookie{
		Name:     sessionCookieName,
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		Secure:   c.Request().TLS != nil,
		MaxAge:   -1,
	}
	c.SetCookie(cookie)
}

func encodeSession(secret string, session authSession) (string, error) {
	payload, err := json.Marshal(session)
	if err != nil {
		return "", fmt.Errorf("marshal session: %w", err)
	}

	payloadEnc := base64.RawURLEncoding.EncodeToString(payload)
	sig := signValue(secret, payloadEnc)
	return payloadEnc + "." + sig, nil
}

func decodeSession(secret, value string) (authSession, error) {
	parts := strings.Split(value, ".")
	if len(parts) != 2 {
		return authSession{}, fmt.Errorf("invalid session format")
	}

	payloadEnc, sig := parts[0], parts[1]
	expectedSig := signValue(secret, payloadEnc)
	if !hmac.Equal([]byte(sig), []byte(expectedSig)) {
		return authSession{}, fmt.Errorf("invalid session signature")
	}

	payload, err := base64.RawURLEncoding.DecodeString(payloadEnc)
	if err != nil {
		return authSession{}, fmt.Errorf("decode session payload: %w", err)
	}

	var session authSession
	if err := json.Unmarshal(payload, &session); err != nil {
		return authSession{}, fmt.Errorf("unmarshal session payload: %w", err)
	}

	return session, nil
}

func signValue(secret, payload string) string {
	mac := hmac.New(sha256.New, []byte(secret))
	_, _ = mac.Write([]byte(payload))
	return base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
}
