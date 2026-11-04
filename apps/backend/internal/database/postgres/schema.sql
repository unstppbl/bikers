CREATE TABLE users (
    id uuid PRIMARY KEY,
    phone text NOT NULL UNIQUE,
    status text NOT NULL DEFAULT 'active' CHECK (status IN ('active','suspended','deleted')),
    display_name text NOT NULL DEFAULT '',
    city text NOT NULL DEFAULT '',
    languages text[] NOT NULL DEFAULT '{}',
    interests text NOT NULL DEFAULT '',
    locale text NOT NULL DEFAULT 'en' CHECK (locale IN ('en','ru','kk')),
    onboarding_complete boolean NOT NULL DEFAULT false,
    created_at timestamptz NOT NULL
);
CREATE TABLE otp_challenges (
    id uuid PRIMARY KEY,
    phone text NOT NULL,
    source_key text NOT NULL,
    provider_id text NOT NULL DEFAULT '',
    state text NOT NULL CHECK (state IN ('sending','pending','failed','uncertain','consumed')),
    attempts integer NOT NULL DEFAULT 0,
    created_at timestamptz NOT NULL,
    expires_at timestamptz NOT NULL,
    resend_after timestamptz NOT NULL,
    consumed_at timestamptz
);
CREATE INDEX otp_phone_created ON otp_challenges(phone,created_at);
CREATE INDEX otp_source_created ON otp_challenges(source_key,created_at);
CREATE TABLE sessions (
    token_digest bytea PRIMARY KEY,
    user_id uuid NOT NULL REFERENCES users(id),
    csrf_token text NOT NULL,
    created_at timestamptz NOT NULL,
    last_seen_at timestamptz NOT NULL,
    expires_at timestamptz NOT NULL,
    revoked_at timestamptz
);
CREATE INDEX sessions_by_user ON sessions(user_id);
CREATE TABLE rides (
    id uuid PRIMARY KEY,
    organizer_id uuid NOT NULL REFERENCES users(id),
    city text NOT NULL,
    description text NOT NULL,
    title text NOT NULL,
    starts_at timestamptz NOT NULL,
    timezone text NOT NULL,
    meeting_point text NOT NULL,
    destination text NOT NULL DEFAULT '',
    route_url text NOT NULL DEFAULT '',
    expectations text NOT NULL DEFAULT '',
    capacity integer NOT NULL CHECK (capacity BETWEEN 1 AND 1000),
    status text NOT NULL DEFAULT 'published' CHECK (status IN ('published','cancelled')),
    plan_version bigint NOT NULL DEFAULT 1,
    message_sequence bigint NOT NULL DEFAULT 0,
    created_at timestamptz NOT NULL
);
CREATE INDEX ride_discovery ON rides(starts_at,id);
CREATE TABLE participation (
    ride_id uuid NOT NULL REFERENCES rides(id),
    user_id uuid NOT NULL REFERENCES users(id),
    state text NOT NULL CHECK (state IN ('requested','approved','rejected','left','removed')),
    updated_at timestamptz NOT NULL,
    PRIMARY KEY(ride_id,user_id)
);
CREATE TABLE messages (
    id uuid PRIMARY KEY,
    ride_id uuid NOT NULL REFERENCES rides(id),
    sender_id uuid NOT NULL REFERENCES users(id),
    sequence bigint NOT NULL,
    client_message_id uuid NOT NULL,
    body text NOT NULL,
    created_at timestamptz NOT NULL,
    UNIQUE(ride_id,sequence),
    UNIQUE(ride_id,sender_id,client_message_id)
);
