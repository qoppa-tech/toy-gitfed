CREATE TABLE git_repository (
    id          UUID PRIMARY KEY,
    name        VARCHAR(100) NOT NULL,
    description VARCHAR(4000),
    is_private  BOOLEAN NOT NULL,
    is_deleted  BOOLEAN NOT NULL,
    created_at  TIMESTAMPTZ DEFAULT now() NOT NULL,
    updated_at  TIMESTAMPTZ DEFAULT now() NOT NULL,
    owner_id    UUID NOT NULL REFERENCES users (user_id),
    default_ref VARCHAR(255) NOT NULL,
    head        VARCHAR(255) NOT NULL
);

CREATE INDEX idx_git_repository_owner_id ON git_repository (owner_id);
