package config

import (
	"strings"
	"testing"
)

func TestMissingSecretsFailWithoutLeakingValues(t *testing.T) {
	valid := map[string]string{
		"LISTEN_ADDR":              ":8080",
		"SESSION_SECRET":           "test-session-secret-value",
		"DB_HOST":                  "db",
		"DB_PORT":                  "3306",
		"DB_NAME":                  "campusclaw",
		"DB_USER":                  "campusclaw",
		"DB_PASSWORD":              "test-database-password-value",
		"UPLOAD_DIR":               "/data/uploads",
		"MAX_UPLOAD_BYTES":         "10485760",
		"SESSION_TTL_SECONDS":      "28800",
		"LOGIN_FAILURE_THRESHOLD":  "5",
		"SEED_TEACHER_A_PASSWORD":  "test-teacher-password",
		"SEED_STUDENT_A1_PASSWORD": "test-student-a-password",
		"SEED_STUDENT_B1_PASSWORD": "test-student-b-password",
	}
	for name, value := range valid {
		t.Setenv(name, value)
	}

	if _, err := Load(); err != nil {
		t.Fatalf("valid configuration rejected: %v", err)
	}

	for _, name := range []string{"SESSION_SECRET", "DB_PASSWORD"} {
		t.Run(name, func(t *testing.T) {
			t.Setenv(name, "")
			_, err := Load()
			if err == nil || !strings.Contains(err.Error(), name) {
				t.Fatalf("missing %s should fail with its key name: %v", name, err)
			}
			if strings.Contains(err.Error(), valid["SESSION_SECRET"]) || strings.Contains(err.Error(), valid["DB_PASSWORD"]) {
				t.Fatalf("configuration error revealed a secret: %v", err)
			}
		})
	}
}
