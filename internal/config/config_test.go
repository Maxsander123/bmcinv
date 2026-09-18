package config

import (
	"os"
	"testing"
)

func TestGetScanConfig_Defaults(t *testing.T) {
	// When AppConfig is nil, fallback defaults are returned
	AppConfig = nil
	cfg := GetScanConfig()

	if cfg.Workers != 10 {
		t.Errorf("Workers = %d, want 10", cfg.Workers)
	}
	if cfg.TimeoutSecs != 30 {
		t.Errorf("TimeoutSecs = %d, want 30", cfg.TimeoutSecs)
	}
	if cfg.RetryAttempts != 2 {
		t.Errorf("RetryAttempts = %d, want 2", cfg.RetryAttempts)
	}
	if !cfg.TLSSkipVerify {
		t.Error("TLSSkipVerify should default to true")
	}
}

func TestGetCredential_NotInitialized(t *testing.T) {
	AppConfig = nil
	_, err := GetCredential(BMCTypeIDRAC)
	if err == nil {
		t.Error("expected error when AppConfig is nil")
	}
}

func TestGetCredential_MissingBMCType(t *testing.T) {
	AppConfig = &Config{
		Credentials: map[string]Credential{
			"idrac": {Username: "root", Password: "calvin"},
		},
	}
	_, err := GetCredential(BMCTypeILO)
	if err == nil {
		t.Error("expected error for unconfigured BMC type")
	}
}

func TestGetCredential_EmptyUsername(t *testing.T) {
	AppConfig = &Config{
		Credentials: map[string]Credential{
			"idrac": {Username: "", Password: "secret"},
		},
	}
	_, err := GetCredential(BMCTypeIDRAC)
	if err == nil {
		t.Error("expected error for empty username")
	}
}

func TestGetCredential_Success(t *testing.T) {
	AppConfig = &Config{
		Credentials: map[string]Credential{
			"idrac": {Username: "root", Password: "calvin"},
		},
	}
	cred, err := GetCredential(BMCTypeIDRAC)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cred.Username != "root" || cred.Password != "calvin" {
		t.Errorf("got %+v, want root/calvin", cred)
	}
}

func TestApplyEnvPasswords(t *testing.T) {
	AppConfig = &Config{
		Credentials: map[string]Credential{
			"idrac": {Username: "root", Password: "old"},
		},
	}

	t.Setenv("BMCINV_IDRAC_PASSWORD", "newpassword")
	applyEnvPasswords()

	cred := AppConfig.Credentials["idrac"]
	if cred.Password != "newpassword" {
		t.Errorf("password = %q, want %q", cred.Password, "newpassword")
	}
}

func TestApplyEnvPasswords_Username(t *testing.T) {
	AppConfig = &Config{
		Credentials: map[string]Credential{
			"ilo": {Username: "Administrator", Password: ""},
		},
	}

	t.Setenv("BMCINV_ILO_USERNAME", "admin2")
	t.Setenv("BMCINV_ILO_PASSWORD", "secret")
	applyEnvPasswords()

	cred := AppConfig.Credentials["ilo"]
	if cred.Username != "admin2" {
		t.Errorf("username = %q, want admin2", cred.Username)
	}
	if cred.Password != "secret" {
		t.Errorf("password = %q, want secret", cred.Password)
	}
}

func TestApplyEnvPasswords_NilConfig(t *testing.T) {
	AppConfig = nil
	// Should not panic
	applyEnvPasswords()
}

func TestDatabasePath_Default(t *testing.T) {
	AppConfig = nil
	path := DatabasePath()
	if path == "" {
		t.Error("DatabasePath should not be empty")
	}
}

func TestBMCTypeConstants(t *testing.T) {
	cases := []struct {
		got  BMCType
		want string
	}{
		{BMCTypeIDRAC, "idrac"},
		{BMCTypeILO, "ilo"},
		{BMCTypeIPMI, "ipmi"},
		{BMCTypeSupermicro, "supermicro"},
		{BMCTypeUnknown, "unknown"},
	}
	for _, tc := range cases {
		if string(tc.got) != tc.want {
			t.Errorf("BMCType constant = %q, want %q", tc.got, tc.want)
		}
	}
}

func TestGetCredential_Supermicro(t *testing.T) {
	AppConfig = &Config{
		Credentials: map[string]Credential{
			"supermicro": {Username: "ADMIN", Password: "ADMIN"},
		},
	}
	cred, err := GetCredential(BMCTypeSupermicro)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cred.Username != "ADMIN" {
		t.Errorf("username = %q, want ADMIN", cred.Username)
	}
}

// TestMain resets AppConfig after each test
func TestMain(m *testing.M) {
	code := m.Run()
	AppConfig = nil
	os.Exit(code)
}
