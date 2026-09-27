package config

import (
	"fmt"
	"os"
	"strconv"
	"time"
)

type Config struct {
	ListenAddr            string
	SessionSecret         string
	DBHost                string
	DBPort                int
	DBName                string
	DBUser                string
	DBPassword            string
	UploadDir             string
	MaxUploadBytes        int64
	SessionTTL            time.Duration
	LoginFailureThreshold int
	SeedTeacherAPassword  string
	SeedStudentA1Password string
	SeedStudentB1Password string
}

func Load() (Config, error) {
	var cfg Config
	var err error

	if cfg.ListenAddr, err = required("LISTEN_ADDR"); err != nil {
		return Config{}, err
	}
	if cfg.SessionSecret, err = required("SESSION_SECRET"); err != nil {
		return Config{}, err
	}
	if cfg.DBHost, err = required("DB_HOST"); err != nil {
		return Config{}, err
	}
	if cfg.DBName, err = required("DB_NAME"); err != nil {
		return Config{}, err
	}
	if cfg.DBUser, err = required("DB_USER"); err != nil {
		return Config{}, err
	}
	if cfg.DBPassword, err = required("DB_PASSWORD"); err != nil {
		return Config{}, err
	}
	if cfg.UploadDir, err = required("UPLOAD_DIR"); err != nil {
		return Config{}, err
	}
	if cfg.SeedTeacherAPassword, err = required("SEED_TEACHER_A_PASSWORD"); err != nil {
		return Config{}, err
	}
	if cfg.SeedStudentA1Password, err = required("SEED_STUDENT_A1_PASSWORD"); err != nil {
		return Config{}, err
	}
	if cfg.SeedStudentB1Password, err = required("SEED_STUDENT_B1_PASSWORD"); err != nil {
		return Config{}, err
	}
	if cfg.DBPort, err = positiveInt("DB_PORT"); err != nil {
		return Config{}, err
	}
	if cfg.DBPort > 65535 {
		return Config{}, fmt.Errorf("invalid DB_PORT")
	}
	if cfg.MaxUploadBytes, err = positiveInt64("MAX_UPLOAD_BYTES"); err != nil {
		return Config{}, err
	}
	ttlSeconds, err := positiveInt("SESSION_TTL_SECONDS")
	if err != nil {
		return Config{}, err
	}
	if int64(ttlSeconds) > int64(^uint64(0)>>1)/int64(time.Second) {
		return Config{}, fmt.Errorf("invalid SESSION_TTL_SECONDS")
	}
	cfg.SessionTTL = time.Duration(ttlSeconds) * time.Second
	if cfg.LoginFailureThreshold, err = positiveInt("LOGIN_FAILURE_THRESHOLD"); err != nil {
		return Config{}, err
	}

	return cfg, nil
}

func required(name string) (string, error) {
	value := os.Getenv(name)
	if value == "" {
		return "", fmt.Errorf("missing %s", name)
	}
	return value, nil
}

func positiveInt(name string) (int, error) {
	value, err := required(name)
	if err != nil {
		return 0, err
	}
	parsed, err := strconv.Atoi(value)
	if err != nil || parsed <= 0 {
		return 0, fmt.Errorf("invalid %s", name)
	}
	return parsed, nil
}

func positiveInt64(name string) (int64, error) {
	value, err := required(name)
	if err != nil {
		return 0, err
	}
	parsed, err := strconv.ParseInt(value, 10, 64)
	if err != nil || parsed <= 0 {
		return 0, fmt.Errorf("invalid %s", name)
	}
	return parsed, nil
}
