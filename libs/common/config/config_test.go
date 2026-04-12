package config

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

type serviceConfig struct {
	AppConfig `mapstructure:",squash"`

	DBSource            string        `mapstructure:"DB_SOURCE"`
	HTTPServerAddress   string        `mapstructure:"HTTP_SERVER_ADDRESS"`
	GRPCServerAddress   string        `mapstructure:"GRPC_SERVER_ADDRESS"`
	AccessTokenDuration time.Duration `mapstructure:"ACCESS_TOKEN_DURATION"`
}

func (c *serviceConfig) Base() *AppConfig {
	return &c.AppConfig
}

func TestLoadConfig_ReadsFromFile(t *testing.T) {
	cfg, err := LoadConfig[serviceConfig]("testdata", "app")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cfg.ServiceName != "users" {
		t.Errorf("ServiceName: expected %q, got %q", "users", cfg.ServiceName)
	}
	if cfg.DBSource != "postgres://localhost:5432/users" {
		t.Errorf("DBSource: expected postgres URL, got %q", cfg.DBSource)
	}
	if cfg.HTTPServerAddress != "0.0.0.0:8080" {
		t.Errorf("HTTPServerAddress: expected %q, got %q", "0.0.0.0:8080", cfg.HTTPServerAddress)
	}

	expectedAllowedOrigins := []string{"https://bar.always", "https://good.game"}
	require.Len(t, cfg.AllowedOrigins, 2)
	require.EqualValues(t, expectedAllowedOrigins, cfg.AllowedOrigins)
}

func TestLoadConfig_EnvironmentUnmarshalsAsTypedValue(t *testing.T) {
	cfg, err := LoadConfig[serviceConfig]("testdata", "app")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cfg.Environment != Development {
		t.Errorf("Environment: expected %q, got %q", Development, cfg.Environment)
	}
}

func TestLoadConfig_ChainsFieldIsNil(t *testing.T) {
	cfg, err := LoadConfig[serviceConfig]("testdata", "app")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cfg.Chains != nil {
		t.Error("expected Chains to be nil after LoadConfig. It should only be wired by Load")
	}
}

func TestLoadConfig_EnvVarOverridesFile(t *testing.T) {
	t.Setenv("SERVICE_NAME", "overridden")

	cfg, err := LoadConfig[serviceConfig]("testdata", "app")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cfg.ServiceName != "overridden" {
		t.Errorf("expected env var to override file value, got %q", cfg.ServiceName)
	}
}

func TestLoadConfig_MissingFileIsNotAnError(t *testing.T) {
	// A missing config file is tolerated. Env vars alone are sufficient.
	t.Setenv("ENVIRONMENT", "staging")
	t.Setenv("SERVICE_NAME", "payments")

	cfg, err := LoadConfig[serviceConfig]("testdata", "nonexistent")
	if err != nil {
		t.Fatalf("expected missing file to be tolerated, got error: %v", err)
	}

	if cfg.ServiceName != "payments" {
		t.Errorf("expected SERVICE_NAME from env, got %q", cfg.ServiceName)
	}
}

func TestLoadConfig_IsolatedBetweenCalls(t *testing.T) {
	cfg1, err := LoadConfig[serviceConfig]("testdata", "app")
	if err != nil {
		t.Fatalf("first LoadConfig failed: %v", err)
	}

	cfg2, err := LoadConfig[serviceConfig]("testdata", "app")
	if err != nil {
		t.Fatalf("second LoadConfig failed: %v", err)
	}

	if cfg1.ServiceName != cfg2.ServiceName {
		t.Errorf("expected identical results from two calls, got %q and %q", cfg1.ServiceName, cfg2.ServiceName)
	}
}

func TestLoad_WiresChainsFromEnvironment(t *testing.T) {
	cfg, err := Load[*serviceConfig]("testdata", "app")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cfg.Chains == nil {
		t.Fatal("expected Chains to be wired by Load, got nil")
	}

	chain := cfg.Chains.Current()
	if chain.ID != 11155111 {
		t.Errorf("expected ETH Sepolia (11155111) for development, got %d", chain.ID)
	}
}

func TestLoad_RejectsInvalidEnvironment(t *testing.T) {
	t.Setenv("ENVIRONMENT", "invalid")

	_, err := Load[*serviceConfig]("testdata", "app")
	if err == nil {
		t.Fatal("expected error for invalid ENVIRONMENT, got nil")
	}
}

func TestLoad_PromotedFieldsAccessibleOnServiceStruct(t *testing.T) {
	cfg, err := Load[*serviceConfig]("testdata", "app")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cfg.ServiceName == "" {
		t.Error("ServiceName should be accessible directly via promotion")
	}
	if len(cfg.AllowedOrigins) == 0 {
		t.Error("AllowedOrigins should be accessible directly via promotion")
	}
	if cfg.Chains == nil {
		t.Error("Chains should be accessible directly via promotion")
	}
}

func TestLoad_IsMethodPromotedToServiceStruct(t *testing.T) {
	cfg, err := Load[*serviceConfig]("testdata", "app")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !cfg.Is(Development) {
		t.Errorf("expected cfg.Is(Development) to be true for environment %q", cfg.Environment)
	}
	if cfg.Is(Production) {
		t.Error("expected cfg.Is(Production) to be false")
	}
}

func TestLoad_ServiceSpecificFieldsStillUnmarshal(t *testing.T) {
	cfg, err := Load[*serviceConfig]("testdata", "app")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cfg.DBSource == "" {
		t.Error("expected DBSource to unmarshal from testdata/app.env")
	}
	if cfg.HTTPServerAddress == "" {
		t.Error("expected HTTPServerAddress to unmarshal from testdata/app.env")
	}
}
