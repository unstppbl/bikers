ALTER TABLE users ADD COLUMN country_code text NOT NULL DEFAULT 'KZ' CHECK(country_code IN ('KZ','KG','UZ','RU','ZZ'));
ALTER TABLE users ADD COLUMN currency_code text NOT NULL DEFAULT 'KZT' CHECK(currency_code IN ('KZT','KGS','UZS','RUB','USD','EUR'));

CREATE TABLE announcements (
 id uuid PRIMARY KEY,
 ride_id uuid NOT NULL REFERENCES rides(id),
 owner_id uuid NOT NULL REFERENCES users(id),
 sequence bigserial NOT NULL UNIQUE,
 client_announcement_id uuid NOT NULL,
 body text NOT NULL CHECK(octet_length(body) BETWEEN 1 AND 4000),
 created_at timestamptz NOT NULL,
 UNIQUE(ride_id,client_announcement_id)
);
CREATE INDEX announcements_ride_sequence ON announcements(ride_id,sequence DESC);

CREATE TABLE notifications (
 id uuid PRIMARY KEY,
 sequence bigserial NOT NULL UNIQUE,
 user_id uuid NOT NULL REFERENCES users(id),
 ride_id uuid NOT NULL REFERENCES rides(id),
 kind text NOT NULL CHECK(kind IN ('ride.updated','ride.cancelled','ride.announcement','participation.requested','participation.approved','participation.rejected','participation.removed')),
 event_id uuid NOT NULL,
 created_at timestamptz NOT NULL,
 read_at timestamptz,
 UNIQUE(user_id,event_id)
);
CREATE INDEX notifications_user_sequence ON notifications(user_id,sequence DESC);
CREATE INDEX notifications_user_unread ON notifications(user_id) WHERE read_at IS NULL;

CREATE TABLE push_subscriptions (
 id uuid PRIMARY KEY,
 user_id uuid NOT NULL REFERENCES users(id),
 session_digest bytea NOT NULL REFERENCES sessions(token_digest),
 endpoint text NOT NULL UNIQUE CHECK(octet_length(endpoint) BETWEEN 1 AND 4096),
 p256dh text NOT NULL,
 auth text NOT NULL,
 created_at timestamptz NOT NULL
);
CREATE INDEX push_subscriptions_user ON push_subscriptions(user_id);
CREATE TABLE push_deliveries (
 id uuid PRIMARY KEY,
 notification_id uuid NOT NULL REFERENCES notifications(id) ON DELETE CASCADE,
 subscription_id uuid NOT NULL REFERENCES push_subscriptions(id) ON DELETE CASCADE,
 state text NOT NULL DEFAULT 'pending' CHECK(state IN ('pending','sending','delivered','failed','skipped')),
 attempts integer NOT NULL DEFAULT 0 CHECK(attempts BETWEEN 0 AND 5),
 next_attempt timestamptz NOT NULL,
 lease_id uuid,
 lease_until timestamptz,
 UNIQUE(notification_id,subscription_id)
);
CREATE INDEX push_deliveries_due ON push_deliveries(next_attempt) WHERE state IN ('pending','sending');
