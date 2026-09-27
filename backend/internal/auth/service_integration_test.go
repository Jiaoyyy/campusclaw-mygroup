package auth

import (
	"context"
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"campusclaw/backend/internal/config"
	"campusclaw/backend/internal/db"
	_ "github.com/go-sql-driver/mysql"
)

func TestSessionLifecycle(t *testing.T) {
	dsn := os.Getenv("TEST_MYSQL_DSN")
	if dsn == "" {
		t.Skip("set TEST_MYSQL_DSN to run the MySQL integration test")
	}
	database, err := sql.Open("mysql", dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	cfg := config.Config{
		SessionSecret:         "integration-session-secret",
		SessionTTL:            time.Hour,
		LoginFailureThreshold: 3,
		UploadDir:             t.TempDir(),
		SeedTeacherAPassword:  "teacher-secret",
		SeedStudentA1Password: "student-a-secret",
		SeedStudentB1Password: "student-b-secret",
	}
	ctx := context.Background()
	if err := db.Migrate(ctx, database); err != nil {
		t.Fatal(err)
	}
	if err := db.Seed(ctx, database, cfg); err != nil {
		t.Fatal(err)
	}
	service, err := New(database, cfg)
	if err != nil {
		t.Fatal(err)
	}
	mux := http.NewServeMux()
	mux.HandleFunc("/api/login", service.Login)
	mux.HandleFunc("/api/logout", service.Logout)
	mux.HandleFunc("/api/me", service.Me)

	call := func(method, path, body string, cookie *http.Cookie, csrf string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(method, path, strings.NewReader(body))
		if cookie != nil {
			req.AddCookie(cookie)
		}
		if csrf != "" {
			req.Header.Set("X-CSRF-Token", csrf)
		}
		response := httptest.NewRecorder()
		mux.ServeHTTP(response, req)
		return response
	}
	login := func(username, password string, cookie *http.Cookie) *httptest.ResponseRecorder {
		return call(http.MethodPost, "/api/login", `{"username":"`+username+`","password":"`+password+`"}`, cookie, "")
	}
	first := login("teacher_a", cfg.SeedTeacherAPassword, nil)
	if first.Code != http.StatusOK || len(first.Result().Cookies()) != 1 {
		t.Fatalf("teacher login: %d %s", first.Code, first.Body.String())
	}
	firstCookie := first.Result().Cookies()[0]
	if !firstCookie.HttpOnly || firstCookie.SameSite != http.SameSiteLaxMode || firstCookie.Secure {
		t.Fatalf("unexpected local HTTP cookie flags: %#v", firstCookie)
	}
	me := call(http.MethodGet, "/api/me", "", firstCookie, "")
	var profile struct {
		Role      string `json:"role"`
		ClassName string `json:"class_name"`
		CSRFToken string `json:"csrf_token"`
	}
	if me.Code != http.StatusOK || json.Unmarshal(me.Body.Bytes(), &profile) != nil || profile.Role != "teacher" || profile.ClassName != "A" || profile.CSRFToken == "" {
		t.Fatalf("teacher profile: %d %s", me.Code, me.Body.String())
	}
	second := login("teacher_a", cfg.SeedTeacherAPassword, firstCookie)
	if second.Code != http.StatusOK {
		t.Fatalf("second login: %d %s", second.Code, second.Body.String())
	}
	secondCookie := second.Result().Cookies()[0]
	if secondCookie.Value == firstCookie.Value || call(http.MethodGet, "/api/me", "", firstCookie, "").Code != http.StatusUnauthorized {
		t.Fatal("login did not rotate and revoke the previous session")
	}
	me = call(http.MethodGet, "/api/me", "", secondCookie, "")
	if err := json.Unmarshal(me.Body.Bytes(), &profile); err != nil {
		t.Fatal(err)
	}
	if call(http.MethodPost, "/api/logout", "", secondCookie, "").Code != http.StatusForbidden {
		t.Fatal("logout without CSRF token was accepted")
	}
	logout := call(http.MethodPost, "/api/logout", "", secondCookie, profile.CSRFToken)
	if logout.Code != http.StatusOK || call(http.MethodGet, "/api/me", "", secondCookie, "").Code != http.StatusUnauthorized {
		t.Fatalf("logout did not revoke session: %d", logout.Code)
	}
	unknown := login("missing_user", "bad", nil)
	wrong := login("student_a1", "bad", nil)
	if unknown.Code != http.StatusUnauthorized || wrong.Code != unknown.Code || wrong.Body.String() != unknown.Body.String() {
		t.Fatal("unknown user and wrong password returned distinguishable errors")
	}
	bStudent := login("student_b1", cfg.SeedStudentB1Password, nil)
	if bStudent.Code != http.StatusOK {
		t.Fatalf("B student login: %d %s", bStudent.Code, bStudent.Body.String())
	}
	var bProfile struct {
		Role      string `json:"role"`
		ClassName string `json:"class_name"`
	}
	bMe := call(http.MethodGet, "/api/me", "", bStudent.Result().Cookies()[0], "")
	if err := json.Unmarshal(bMe.Body.Bytes(), &bProfile); err != nil || bProfile.Role != "student" || bProfile.ClassName != "B" {
		t.Fatalf("B student profile: %d %s", bMe.Code, bMe.Body.String())
	}
	for i := 0; i < cfg.LoginFailureThreshold; i++ {
		login("student_b1", "bad", nil)
	}
	locked := login("student_b1", cfg.SeedStudentB1Password, nil)
	if locked.Code != http.StatusUnauthorized || locked.Body.String() != unknown.Body.String() {
		t.Fatal("locked login did not use the same credential error")
	}
	student := login("student_a1", cfg.SeedStudentA1Password, nil)
	if student.Code != http.StatusOK {
		t.Fatalf("student login: %d %s", student.Code, student.Body.String())
	}
	studentCookie := student.Result().Cookies()[0]
	studentMe := call(http.MethodGet, "/api/me", "", studentCookie, "")
	if err := json.Unmarshal(studentMe.Body.Bytes(), &profile); err != nil || profile.Role != "student" || profile.ClassName != "A" {
		t.Fatalf("student profile: %d %s", studentMe.Code, studentMe.Body.String())
	}
	if _, err := database.ExecContext(ctx, "UPDATE users SET role = 'teacher' WHERE username = 'student_a1'"); err != nil {
		t.Fatal(err)
	}
	changed := call(http.MethodGet, "/api/me", "", studentCookie, "")
	if err := json.Unmarshal(changed.Body.Bytes(), &profile); err != nil || profile.Role != "teacher" {
		t.Fatalf("role was not reread from database: %d %s", changed.Code, changed.Body.String())
	}
	if _, err := database.ExecContext(ctx, "UPDATE sessions SET expires_at = UTC_TIMESTAMP(6) - INTERVAL 1 SECOND WHERE user_id = (SELECT id FROM users WHERE username = 'student_a1')"); err != nil {
		t.Fatal(err)
	}
	if call(http.MethodGet, "/api/me", "", studentCookie, "").Code != http.StatusUnauthorized {
		t.Fatal("expired session was accepted")
	}
	fake := &http.Cookie{Name: cookieName, Value: strings.Repeat("0", 64)}
	if call(http.MethodGet, "/api/me", "", fake, "").Code != http.StatusUnauthorized {
		t.Fatal("forged session was accepted")
	}
}
