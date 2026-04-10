CREATE TABLE sso_providers (
    sso_id       UUID         PRIMARY KEY,
    user_id      UUID         NOT NULL REFERENCES users (user_id) ON DELETE CASCADE,
    name         VARCHAR(255) NOT NULL,
    provider     VARCHAR(50)  NOT NULL CHECK (provider IN ('google', 'local', 'qoppatech')),
    username     VARCHAR(255),
    activated_at TIMESTAMPTZ,
    created_at   TIMESTAMPTZ  NOT NULL DEFAULT now()
);

CREATE INDEX idx_sso_user_id  ON sso_providers (user_id);
CREATE INDEX idx_sso_provider ON sso_providers (provider, username);
