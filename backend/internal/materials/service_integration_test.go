package materials

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"campusclaw/backend/internal/auth"
	"campusclaw/backend/internal/config"
	"campusclaw/backend/internal/db"
	_ "github.com/go-sql-driver/mysql"
)

func TestClassIsolationAndUpload(t *testing.T) {
	dsn := os.Getenv("TEST_MYSQL_DSN")
	if dsn == "" {
		t.Skip("set TEST_MYSQL_DSN to run the MySQL integration test")
	}
	database, err := sql.Open("mysql", dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	ctx := context.Background()
	cfg := config.Config{
		SessionSecret: "materials-test-secret", SessionTTL: time.Hour, LoginFailureThreshold: 5,
		UploadDir: t.TempDir(), MaxUploadBytes: 128,
		SeedTeacherAPassword: "teacher-test-password", SeedStudentA1Password: "a-test-password", SeedStudentB1Password: "b-test-password",
	}
	if err := db.Migrate(ctx, database); err != nil {
		t.Fatal(err)
	}
	if err := db.Seed(ctx, database, cfg); err != nil {
		t.Fatal(err)
	}
	authentication, err := auth.New(database, cfg)
	if err != nil {
		t.Fatal(err)
	}
	mux := http.NewServeMux()
	mux.HandleFunc("/api/login", authentication.Login)
	mux.HandleFunc("/api/me", authentication.Me)
	service := New(database, cfg, authentication)
	service.Register(mux)
	call := func(method, target string, body io.Reader, contentType, token, csrf string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(method, target, body)
		if contentType != "" {
			req.Header.Set("Content-Type", contentType)
		}
		if token != "" {
			req.Header.Set("Authorization", "Bearer "+token)
		}
		if csrf != "" {
			req.Header.Set("X-CSRF-Token", csrf)
		}
		response := httptest.NewRecorder()
		mux.ServeHTTP(response, req)
		return response
	}
	login := func(username, password string) (string, string) {
		t.Helper()
		body := `{"username":"` + username + `","password":"` + password + `"}`
		response := call(http.MethodPost, "/api/login", strings.NewReader(body), "application/json", "", "")
		if response.Code != http.StatusOK {
			t.Fatalf("login %s: %d %s", username, response.Code, response.Body.String())
		}
		var loginResult struct {
			AccessToken string `json:"access_token"`
		}
		if err := json.Unmarshal(response.Body.Bytes(), &loginResult); err != nil || loginResult.AccessToken == "" {
			t.Fatal("missing access token")
		}
		me := call(http.MethodGet, "/api/me", nil, "", loginResult.AccessToken, "")
		var profile struct {
			CSRFToken string `json:"csrf_token"`
		}
		if err := json.Unmarshal(me.Body.Bytes(), &profile); err != nil {
			t.Fatal(err)
		}
		return loginResult.AccessToken, profile.CSRFToken
	}
	makeUpload := func(filename string, content []byte) (*bytes.Buffer, string) {
		t.Helper()
		body := new(bytes.Buffer)
		writer := multipart.NewWriter(body)
		part, err := writer.CreateFormFile("file", filename)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := part.Write(content); err != nil {
			t.Fatal(err)
		}
		if err := writer.Close(); err != nil {
			t.Fatal(err)
		}
		return body, writer.FormDataContentType()
	}
	teacher, teacherCSRF := login("teacher_a", cfg.SeedTeacherAPassword)
	aStudent, aCSRF := login("student_a1", cfg.SeedStudentA1Password)
	bStudent, _ := login("student_b1", cfg.SeedStudentB1Password)
	list := func(token string) []Material {
		t.Helper()
		response := call(http.MethodGet, "/api/materials", nil, "", token, "")
		var result struct {
			Items []Material `json:"items"`
		}
		if response.Code != http.StatusOK || json.Unmarshal(response.Body.Bytes(), &result) != nil {
			t.Fatalf("list: %d %s", response.Code, response.Body.String())
		}
		return result.Items
	}
	aItems, bItems := list(aStudent), list(bStudent)
	if len(aItems) != 1 || len(bItems) != 1 || !strings.Contains(aItems[0].OriginalFilename, "A班") || !strings.Contains(bItems[0].OriginalFilename, "B班") {
		t.Fatalf("seed class lists leaked or missed data: A=%+v B=%+v", aItems, bItems)
	}
	pathB := "/api/materials/" + strconv.FormatInt(bItems[0].ID, 10)
	missing := call(http.MethodGet, "/api/materials/99999999", nil, "", aStudent, "")
	cross := call(http.MethodGet, pathB, nil, "", aStudent, "")
	if missing.Code != http.StatusNotFound || cross.Code != missing.Code || cross.Body.String() != missing.Body.String() {
		t.Fatal("cross-class detail differed from missing material")
	}
	if call(http.MethodGet, pathB+"/file", nil, "", aStudent, "").Code != http.StatusNotFound {
		t.Fatal("cross-class file was accessible")
	}
	filtered := call(http.MethodGet, "/api/materials?search=B班", nil, "", aStudent, "")
	if filtered.Code != http.StatusOK || !strings.Contains(filtered.Body.String(), `"total":0`) || strings.Contains(filtered.Body.String(), "B班实验须知") {
		t.Fatalf("cross-class search leaked data: %d %s", filtered.Code, filtered.Body.String())
	}
	body, contentType := makeUpload("new.md", []byte("# A-only lesson\nSafe text"))
	if response := call(http.MethodPost, "/api/materials", body, contentType, aStudent, aCSRF); response.Code != http.StatusForbidden {
		t.Fatalf("student upload: %d %s", response.Code, response.Body.String())
	}
	body, contentType = makeUpload("forged-role.txt", []byte("still a student"))
	forged := httptest.NewRequest(http.MethodPost, "/api/materials", body)
	forged.Header.Set("Content-Type", contentType)
	forged.Header.Set("X-CSRF-Token", aCSRF)
	forged.Header.Set("X-Role", "teacher")
	forged.Header.Set("Authorization", "Bearer "+aStudent)
	forgedResponse := httptest.NewRecorder()
	mux.ServeHTTP(forgedResponse, forged)
	if forgedResponse.Code != http.StatusForbidden {
		t.Fatalf("student forged teacher role: %d %s", forgedResponse.Code, forgedResponse.Body.String())
	}
	body, contentType = makeUpload("new.md", []byte("# A-only lesson\nSafe text"))
	if response := call(http.MethodPost, "/api/materials", body, contentType, teacher, ""); response.Code != http.StatusForbidden {
		t.Fatalf("missing CSRF: %d %s", response.Code, response.Body.String())
	}
	body, contentType = makeUpload("new.md", []byte("# A-only lesson\nSafe text"))
	created := call(http.MethodPost, "/api/materials", body, contentType, teacher, teacherCSRF)
	var inserted struct {
		ID int64 `json:"id"`
	}
	if created.Code != http.StatusCreated || json.Unmarshal(created.Body.Bytes(), &inserted) != nil || inserted.ID <= 0 {
		t.Fatalf("valid upload: %d %s", created.Code, created.Body.String())
	}
	if len(list(aStudent)) != 2 || len(list(bStudent)) != 1 {
		t.Fatal("uploaded material appeared outside A class or not in A list")
	}
	uploadPath := "/api/materials/" + strconv.FormatInt(inserted.ID, 10)
	detail := call(http.MethodGet, uploadPath, nil, "", aStudent, "")
	if detail.Code != http.StatusOK || !strings.Contains(detail.Body.String(), "A-only lesson") {
		t.Fatalf("same-class detail: %d %s", detail.Code, detail.Body.String())
	}
	download := call(http.MethodGet, uploadPath+"/file", nil, "", aStudent, "")
	if download.Code != http.StatusOK || download.Body.String() != "# A-only lesson\nSafe text" {
		t.Fatalf("same-class download: %d %s", download.Code, download.Body.String())
	}
	if call(http.MethodGet, uploadPath, nil, "", bStudent, "").Code != http.StatusNotFound {
		t.Fatal("B student read A upload")
	}
	for _, rejected := range []struct {
		name string
		body []byte
		want int
	}{
		{"empty.txt", nil, http.StatusBadRequest},
		{"invalid.txt", []byte{0xff}, http.StatusBadRequest},
		{"unsupported.pdf", []byte("content"), http.StatusBadRequest},
		{"large.txt", bytes.Repeat([]byte("x"), 100000), http.StatusRequestEntityTooLarge},
	} {
		body, contentType := makeUpload(rejected.name, rejected.body)
		response := call(http.MethodPost, "/api/materials", body, contentType, teacher, teacherCSRF)
		if response.Code != rejected.want {
			t.Errorf("%s: got %d want %d", rejected.name, response.Code, rejected.want)
		}
		if rejected.name == "large.txt" && body.Len() == 0 {
			t.Error("oversized request was fully read before rejection")
		}
	}
	body, contentType = makeUpload("wrong.txt", []byte("not allowed"))
	if response := call(http.MethodPost, "/api/materials?class_id=2", body, contentType, teacher, teacherCSRF); response.Code != http.StatusForbidden {
		t.Fatalf("class override: %d %s", response.Code, response.Body.String())
	}
	if len(list(aStudent)) != 2 || len(list(bStudent)) != 1 {
		t.Fatal("rejected uploads left visible materials")
	}
	files, err := os.ReadDir(cfg.UploadDir)
	if err != nil || len(files) != 3 {
		t.Fatalf("rejected uploads left files: %v count=%d", err, len(files))
	}
	var knowledgeCount int
	if err := database.QueryRowContext(ctx, "SELECT COUNT(*) FROM knowledge_entries").Scan(&knowledgeCount); err != nil || knowledgeCount != 3 {
		t.Fatalf("knowledge entries after uploads: %d %v", knowledgeCount, err)
	}
	if _, err := database.ExecContext(ctx, `CREATE TRIGGER fail_knowledge BEFORE INSERT ON knowledge_entries FOR EACH ROW SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='injected failure'`); err != nil {
		t.Fatal(err)
	}
	body, contentType = makeUpload("db-failure.txt", []byte("must roll back"))
	if response := call(http.MethodPost, "/api/materials", body, contentType, teacher, teacherCSRF); response.Code != http.StatusServiceUnavailable {
		t.Fatalf("injected database failure: %d %s", response.Code, response.Body.String())
	}
	if _, err := database.ExecContext(ctx, "DROP TRIGGER fail_knowledge"); err != nil {
		t.Fatal(err)
	}
	service.cfg.UploadDir = filepath.Join(cfg.UploadDir, "nonexistent")
	body, contentType = makeUpload("disk-failure.txt", []byte("must not write SQL"))
	if response := call(http.MethodPost, "/api/materials", body, contentType, teacher, teacherCSRF); response.Code != http.StatusServiceUnavailable {
		t.Fatalf("injected disk failure: %d %s", response.Code, response.Body.String())
	}
	service.cfg.UploadDir = cfg.UploadDir
	if len(list(aStudent)) != 2 {
		t.Fatal("injected failures left visible materials")
	}
	files, err = os.ReadDir(cfg.UploadDir)
	if err != nil || len(files) != 3 {
		t.Fatalf("injected failures left files: %v count=%d", err, len(files))
	}
	if err := database.QueryRowContext(ctx, "SELECT COUNT(*) FROM knowledge_entries").Scan(&knowledgeCount); err != nil || knowledgeCount != 3 {
		t.Fatalf("injected failures left knowledge: %d %v", knowledgeCount, err)
	}
	orphan := filepath.Join(cfg.UploadDir, strings.Repeat("e", 64))
	if err := os.WriteFile(orphan, []byte("orphan"), 0o600); err != nil {
		t.Fatal(err)
	}
	old := time.Now().Add(-2 * time.Hour)
	if err := os.Chtimes(orphan, old, old); err != nil {
		t.Fatal(err)
	}
	var uploadedKey string
	if err := database.QueryRowContext(ctx, "SELECT storage_key FROM materials WHERE id = ?", inserted.ID).Scan(&uploadedKey); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(filepath.Join(cfg.UploadDir, uploadedKey), old, old); err != nil {
		t.Fatal(err)
	}
	if err := CleanupOrphans(ctx, database, cfg.UploadDir, time.Hour); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(orphan); !os.IsNotExist(err) {
		t.Fatal("orphan file was not removed")
	}
	if len(list(aStudent)) != 2 {
		t.Fatal("cleanup changed committed materials")
	}
	if _, err := os.Stat(filepath.Join(cfg.UploadDir, uploadedKey)); err != nil {
		t.Fatalf("cleanup removed a committed file: %v", err)
	}
	if _, err := database.ExecContext(ctx, "DELETE FROM materials WHERE class_id = (SELECT id FROM classes WHERE name = 'B')"); err != nil {
		t.Fatal(err)
	}
	if got := list(bStudent); len(got) != 0 {
		t.Fatalf("empty B class list still shows demo entries: %+v", got)
	}
}
