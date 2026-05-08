CREATE TABLE organizations (
    organization_id          UUID PRIMARY KEY,
    organization_name        VARCHAR(255) NOT NULL,
    organization_description VARCHAR(4000)
);

CREATE TABLE organization_users (
    organization_id UUID NOT NULL REFERENCES organizations (organization_id) ON DELETE CASCADE,
    user_id         UUID NOT NULL REFERENCES users (user_id) ON DELETE CASCADE,
    PRIMARY KEY (organization_id, user_id)
);
