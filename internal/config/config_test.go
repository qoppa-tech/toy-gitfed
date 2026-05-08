package config

import "testing"

func TestValidateRequiresReposDirInDev(t *testing.T) {
	t.Setenv("ENV", "dev")
	t.Setenv("REPOS_DIR", "")

	cfg := Load()
	err := cfg.Validate()
	if err == nil {
		t.Fatal("expected validation error")
	}

	verr, ok := err.(*ValidationError)
	if !ok {
		t.Fatalf("expected ValidationError, got %T", err)
	}

	if len(verr.MissingVars) == 0 {
		t.Fatal("expected missing vars")
	}
}

func TestValidateRequiresInfraVarsInProd(t *testing.T) {
	t.Setenv("ENV", "prod")
	t.Setenv("REPOS_DIR", "/tmp/repos")
	t.Setenv("DB_HOST", "")
	t.Setenv("REDIS_HOST", "")
	t.Setenv("HTTP_ADDR", "")

	cfg := Load()
	err := cfg.Validate()
	if err == nil {
		t.Fatal("expected validation error")
	}

	verr := err.(*ValidationError)
	if len(verr.MissingVars) == 0 {
		t.Fatal("expected missing vars")
	}
}

func TestValidatePassesWithRequiredProdVars(t *testing.T) {
	t.Setenv("ENV", "prod")
	t.Setenv("DB_HOST", "localhost")
	t.Setenv("DB_PORT", "5432")
	t.Setenv("DB_USER", "postgres")
	t.Setenv("DB_PASSWORD", "postgres")
	t.Setenv("DB_NAME", "gitfed")
	t.Setenv("DB_SSLMODE", "disable")
	t.Setenv("REDIS_HOST", "localhost")
	t.Setenv("REDIS_PORT", "6379")
	t.Setenv("HTTP_ADDR", "0.0.0.0:8080")
	t.Setenv("REPOS_DIR", "/tmp/repos")
	t.Setenv("SHUTDOWN_TIMEOUT", "15s")
	t.Setenv("HEALTHCHECK_TIMEOUT", "2s")

	cfg := Load()
	if err := cfg.Validate(); err != nil {
		t.Fatalf("expected no validation error, got: %v", err)
	}
}

func TestValidateRejectsInvalidHealthcheckTimeout(t *testing.T) {
	t.Setenv("ENV", "prod")
	t.Setenv("DB_HOST", "localhost")
	t.Setenv("DB_PORT", "5432")
	t.Setenv("DB_USER", "postgres")
	t.Setenv("DB_PASSWORD", "postgres")
	t.Setenv("DB_NAME", "gitfed")
	t.Setenv("DB_SSLMODE", "disable")
	t.Setenv("REDIS_HOST", "localhost")
	t.Setenv("REDIS_PORT", "6379")
	t.Setenv("HTTP_ADDR", "0.0.0.0:8080")
	t.Setenv("REPOS_DIR", "/tmp/repos")
	t.Setenv("HEALTHCHECK_TIMEOUT", "-1s")

	cfg := Load()
	err := cfg.Validate()
	if err == nil {
		t.Fatal("expected validation error")
	}

	verr := err.(*ValidationError)
	if _, ok := verr.InvalidVars["HEALTHCHECK_TIMEOUT"]; !ok {
		t.Fatal("expected HEALTHCHECK_TIMEOUT invalid var")
	}
}

func TestValidateRejectsInvalidPorts(t *testing.T) {
	t.Setenv("ENV", "prod")
	t.Setenv("DB_HOST", "localhost")
	t.Setenv("DB_PORT", "99999")
	t.Setenv("DB_USER", "postgres")
	t.Setenv("DB_PASSWORD", "postgres")
	t.Setenv("DB_NAME", "gitfed")
	t.Setenv("DB_SSLMODE", "disable")
	t.Setenv("REDIS_HOST", "localhost")
	t.Setenv("REDIS_PORT", "0")
	t.Setenv("HTTP_ADDR", "0.0.0.0:8080")
	t.Setenv("REPOS_DIR", "/tmp/repos")

	cfg := Load()
	err := cfg.Validate()
	if err == nil {
		t.Fatal("expected validation error")
	}

	verr := err.(*ValidationError)
	if len(verr.InvalidVars) == 0 {
		t.Fatal("expected invalid vars")
	}
}

func TestValidateSeedRequiresSeedEnvVars(t *testing.T) {
	t.Setenv("SEED_ADMIN_NAME", "")
	t.Setenv("SEED_ADMIN_USERNAME", "")
	t.Setenv("SEED_ADMIN_EMAIL", "")
	t.Setenv("SEED_ADMIN_PASSWORD", "")

	cfg := Load()
	err := cfg.ValidateSeed()
	if err == nil {
		t.Fatal("expected validation error")
	}

	verr, ok := err.(*ValidationError)
	if !ok {
		t.Fatalf("expected ValidationError, got %T", err)
	}
	if len(verr.MissingVars) != 4 {
		t.Fatalf("expected 4 missing vars, got %d", len(verr.MissingVars))
	}
}

func TestValidateSeedPassesWithAllVars(t *testing.T) {
	t.Setenv("SEED_ADMIN_NAME", "Admin User")
	t.Setenv("SEED_ADMIN_USERNAME", "admin")
	t.Setenv("SEED_ADMIN_EMAIL", "admin@gitfed.local")
	t.Setenv("SEED_ADMIN_PASSWORD", "secret")

	cfg := Load()
	if err := cfg.ValidateSeed(); err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
}
