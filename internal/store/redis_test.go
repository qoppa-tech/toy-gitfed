package store

import "testing"

func TestRedisConfig_Addr(t *testing.T) {
	tests := []struct {
		name string
		cfg  RedisConfig
		want string
	}{
		{
			name: "default",
			cfg:  RedisConfig{Host: "localhost", Port: 6379},
			want: "localhost:6379",
		},
		{
			name: "custom",
			cfg:  RedisConfig{Host: "redis.example.com", Port: 6380},
			want: "redis.example.com:6380",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.cfg.Addr()
			if got != tt.want {
				t.Errorf("Addr() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestAccessKey(t *testing.T) {
	got := accessKey("abc123")
	want := "access:abc123"
	if got != want {
		t.Errorf("accessKey() = %q, want %q", got, want)
	}
}

func TestRefreshKey(t *testing.T) {
	got := refreshKey("abc123")
	want := "refresh:abc123"
	if got != want {
		t.Errorf("refreshKey() = %q, want %q", got, want)
	}
}

func TestTotpKey(t *testing.T) {
	got := totpKey("user-123")
	want := "totp:user-123"
	if got != want {
		t.Errorf("totpKey() = %q, want %q", got, want)
	}
}

func TestOauthStateKey(t *testing.T) {
	got := oauthStateKey("state-xyz")
	want := "oauth_state:state-xyz"
	if got != want {
		t.Errorf("oauthStateKey() = %q, want %q", got, want)
	}
}
