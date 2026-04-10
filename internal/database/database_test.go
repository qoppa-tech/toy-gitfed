package database

import "testing"

func TestConfig_DSN(t *testing.T) {
	tests := []struct {
		name string
		cfg  Config
		want string
	}{
		{
			name: "default sslmode",
			cfg: Config{
				Host:     "localhost",
				Port:     5432,
				User:     "postgres",
				Password: "secret",
				DBName:   "testdb",
			},
			want: "postgres://postgres:secret@localhost:5432/testdb?sslmode=disable",
		},
		{
			name: "explicit sslmode",
			cfg: Config{
				Host:     "db.example.com",
				Port:     5433,
				User:     "app",
				Password: "pass",
				DBName:   "prod",
				SSLMode:  "require",
			},
			want: "postgres://app:pass@db.example.com:5433/prod?sslmode=require",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.cfg.DSN()
			if got != tt.want {
				t.Errorf("DSN() = %q, want %q", got, tt.want)
			}
		})
	}
}
