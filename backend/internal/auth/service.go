package auth

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"campusclaw/backend/internal/config"
	"golang.org/x/crypto/bcrypt"
)

const cookieName = "campusclaw_session"

type Identity struct {
	ID        int64  `json:"id"`
	Username  string `json:"username"`
	Role      string `json:"role"`
	ClassID   int64  `json:"class_id"`
	ClassName string `json:"class_name"`
}

type loginAttempt struct {
	failures    int
	lockedUntil time.Time
	lastSeen    time.Time
}

type Service struct {
	database  *sql.DB
	cfg       config.Config
	dummyHash []byte
	mu        sync.Mutex
	attempts  map[string]loginAttempt
}

func New(database *sql.DB, cfg config.Config) (*Service, error) {
	dummy, err := bcrypt.GenerateFromPassword([]byte("unknown-user-comparison"), bcrypt.DefaultCost)
	if err != nil {
		return nil, err
	}
	return &Service{database: database, cfg: cfg, dummyHash: dummy, attempts: make(map[string]loginAttempt)}, nil
}

func (s *Service) Login(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		WriteError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	if !sameOrigin(r) {
		WriteError(w, http.StatusForbidden, "request origin rejected")
		return
	}
	var input struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	decoder := json.NewDecoder(io.LimitReader(r.Body, 4096))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&input); err != nil || decoder.Decode(new(any)) != io.EOF || input.Username == "" || input.Password == "" {
		WriteError(w, http.StatusBadRequest, "invalid login request")
		return
	}
	key := strings.ToLower(input.Username) + "|" + clientIP(r)
	if s.locked(key) {
		WriteError(w, http.StatusUnauthorized, "invalid credentials")
		return
	}
	var id int64
	var hash string
	err := s.database.QueryRowContext(r.Context(), "SELECT id, password_hash FROM users WHERE username = ?", input.Username).Scan(&id, &hash)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		WriteError(w, http.StatusServiceUnavailable, "service unavailable")
		return
	}
	comparisonHash := []byte(hash)
	if errors.Is(err, sql.ErrNoRows) {
		comparisonHash = s.dummyHash
	}
	matched := bcrypt.CompareHashAndPassword(comparisonHash, []byte(input.Password)) == nil
	if !matched || errors.Is(err, sql.ErrNoRows) {
		s.fail(key)
		WriteError(w, http.StatusUnauthorized, "invalid credentials")
		return
	}
	if s.locked(key) {
		WriteError(w, http.StatusUnauthorized, "invalid credentials")
		return
	}
	token, err := randomToken()
	if err != nil {
		WriteError(w, http.StatusInternalServerError, "session unavailable")
		return
	}
	tx, err := s.database.BeginTx(r.Context(), nil)
	if err != nil {
		WriteError(w, http.StatusServiceUnavailable, "service unavailable")
		return
	}
	defer tx.Rollback()
	if previous, err := r.Cookie(cookieName); err == nil && previous.Value != "" {
		if _, err := tx.ExecContext(r.Context(), "DELETE FROM sessions WHERE token_hash = ?", s.sessionHash(previous.Value)); err != nil {
			WriteError(w, http.StatusServiceUnavailable, "service unavailable")
			return
		}
	}
	expires := time.Now().UTC().Add(s.cfg.SessionTTL)
	if _, err := tx.ExecContext(r.Context(), "INSERT INTO sessions (token_hash, user_id, expires_at) VALUES (?, ?, ?)", s.sessionHash(token), id, expires); err != nil {
		WriteError(w, http.StatusServiceUnavailable, "service unavailable")
		return
	}
	if err := tx.Commit(); err != nil {
		WriteError(w, http.StatusServiceUnavailable, "service unavailable")
		return
	}
	s.clear(key)
	http.SetCookie(w, &http.Cookie{Name: cookieName, Value: token, Path: "/", HttpOnly: true, SameSite: http.SameSiteLaxMode, Secure: secureCookie(r), Expires: expires, MaxAge: int(s.cfg.SessionTTL.Seconds())})
	WriteJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (s *Service) Logout(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		WriteError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	identity, token, status := s.Current(r)
	if status != 0 {
		WriteError(w, status, statusMessage(status))
		return
	}
	_ = identity
	if !s.ValidCSRF(r, token) {
		WriteError(w, http.StatusForbidden, "invalid csrf token")
		return
	}
	if _, err := s.database.ExecContext(r.Context(), "DELETE FROM sessions WHERE token_hash = ?", s.sessionHash(token)); err != nil {
		WriteError(w, http.StatusServiceUnavailable, "service unavailable")
		return
	}
	http.SetCookie(w, &http.Cookie{Name: cookieName, Value: "", Path: "/", HttpOnly: true, SameSite: http.SameSiteLaxMode, Secure: secureCookie(r), MaxAge: -1})
	WriteJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (s *Service) Me(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		WriteError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	identity, token, status := s.Current(r)
	if status != 0 {
		WriteError(w, status, statusMessage(status))
		return
	}
	WriteJSON(w, http.StatusOK, struct {
		Identity
		CSRFToken string `json:"csrf_token"`
	}{Identity: identity, CSRFToken: s.csrfToken(token)})
}

func (s *Service) Current(r *http.Request) (Identity, string, int) {
	cookie, err := r.Cookie(cookieName)
	if err != nil || cookie.Value == "" {
		return Identity{}, "", http.StatusUnauthorized
	}
	var identity Identity
	err = s.database.QueryRowContext(r.Context(), `SELECT u.id, u.username, u.role, u.class_id, c.name
		FROM sessions s JOIN users u ON u.id = s.user_id JOIN classes c ON c.id = u.class_id
		WHERE s.token_hash = ? AND s.expires_at > UTC_TIMESTAMP(6)`, s.sessionHash(cookie.Value)).Scan(
		&identity.ID, &identity.Username, &identity.Role, &identity.ClassID, &identity.ClassName)
	if errors.Is(err, sql.ErrNoRows) {
		return Identity{}, "", http.StatusUnauthorized
	}
	if err != nil {
		return Identity{}, "", http.StatusServiceUnavailable
	}
	if identity.ClassID <= 0 || (identity.Role != "teacher" && identity.Role != "student") {
		return Identity{}, "", http.StatusUnauthorized
	}
	return identity, cookie.Value, 0
}

func (s *Service) ValidCSRF(r *http.Request, sessionToken string) bool {
	if !sameOrigin(r) {
		return false
	}
	provided := r.Header.Get("X-CSRF-Token")
	expected := s.csrfToken(sessionToken)
	return provided != "" && hmac.Equal([]byte(provided), []byte(expected))
}

func (s *Service) sessionHash(token string) string {
	mac := hmac.New(sha256.New, []byte(s.cfg.SessionSecret))
	_, _ = mac.Write([]byte("session:" + token))
	return hex.EncodeToString(mac.Sum(nil))
}

func (s *Service) csrfToken(token string) string {
	mac := hmac.New(sha256.New, []byte(s.cfg.SessionSecret))
	_, _ = mac.Write([]byte("csrf:" + token))
	return hex.EncodeToString(mac.Sum(nil))
}

func randomToken() (string, error) {
	bytes := make([]byte, 32)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return hex.EncodeToString(bytes), nil
}

func (s *Service) locked(key string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	attempt := s.attempts[key]
	return time.Now().Before(attempt.lockedUntil)
}

func (s *Service) fail(key string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	now := time.Now()
	attempt := s.attempts[key]
	if now.Sub(attempt.lastSeen) > 15*time.Minute {
		attempt.failures = 0
	}
	attempt.failures++
	attempt.lastSeen = now
	if attempt.failures >= s.cfg.LoginFailureThreshold {
		attempt.lockedUntil = now.Add(15 * time.Minute)
	}
	s.attempts[key] = attempt
}

func (s *Service) clear(key string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.attempts, key)
}

func clientIP(r *http.Request) string {
	if forwarded := net.ParseIP(r.Header.Get("X-Real-IP")); forwarded != nil {
		return forwarded.String()
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}

func sameOrigin(r *http.Request) bool {
	origin := r.Header.Get("Origin")
	if origin == "" {
		return true
	}
	expected := "http://" + r.Host
	if secureCookie(r) {
		expected = "https://" + r.Host
	}
	return origin == expected
}

func secureCookie(r *http.Request) bool {
	return r.TLS != nil || r.Header.Get("X-Forwarded-Proto") == "https"
}

func statusMessage(status int) string {
	if status == http.StatusServiceUnavailable {
		return "service unavailable"
	}
	return "authentication required"
}

func WriteJSON(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(data)
}

func WriteError(w http.ResponseWriter, status int, message string) {
	WriteJSON(w, status, map[string]string{"error": message})
}
