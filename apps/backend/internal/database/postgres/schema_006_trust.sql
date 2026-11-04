CREATE TABLE trust_admins (
    user_id uuid PRIMARY KEY REFERENCES users(id),
    granted_at timestamptz NOT NULL
);
CREATE TABLE user_blocks (
    blocker_id uuid NOT NULL REFERENCES users(id),
    blocked_id uuid NOT NULL REFERENCES users(id),
    created_at timestamptz NOT NULL,
    PRIMARY KEY(blocker_id,blocked_id),
    CHECK(blocker_id<>blocked_id)
);
CREATE INDEX user_blocks_reverse ON user_blocks(blocked_id,blocker_id);
CREATE TABLE moderation_hidden (
    target_type text NOT NULL CHECK(target_type IN ('user','ride','message','club','bike','bike_photo','club_image','avatar','ride_photo','ride_video','announcement')),
    target_id uuid NOT NULL,
    hidden_at timestamptz NOT NULL,
    PRIMARY KEY(target_type,target_id)
);
CREATE TABLE community_badges (
    user_id uuid PRIMARY KEY REFERENCES users(id),
    assigned_at timestamptz NOT NULL
);
CREATE TABLE trust_reports (
    id uuid PRIMARY KEY,
    reporter_id uuid NOT NULL REFERENCES users(id),
    target_type text NOT NULL CHECK(target_type IN ('user','ride','message','club','bike','bike_photo','club_image','avatar','ride_photo','ride_video','announcement')),
    target_id uuid NOT NULL,
    reason text NOT NULL CHECK(reason IN ('spam','harassment','dangerous_content','other')),
    details text NOT NULL CHECK(octet_length(details)<=4000),
    status text NOT NULL CHECK(status IN ('open','reviewing','resolved','dismissed')),
    version bigint NOT NULL CHECK(version>0),
    snapshot jsonb NOT NULL,
    created_at timestamptz NOT NULL,
    updated_at timestamptz NOT NULL
);
CREATE UNIQUE INDEX trust_reports_open_target ON trust_reports(reporter_id,target_type,target_id) WHERE status IN ('open','reviewing');
CREATE INDEX trust_reports_queue ON trust_reports(status,created_at DESC,id DESC);
CREATE INDEX trust_reports_rate ON trust_reports(reporter_id,created_at);
CREATE TABLE trust_audit (
    id uuid PRIMARY KEY,
    actor_id uuid REFERENCES users(id),
    source text NOT NULL CHECK(source IN ('admin','cli')),
    action text NOT NULL,
    target_type text NOT NULL,
    target_id uuid NOT NULL,
    reason text NOT NULL CHECK(octet_length(reason) BETWEEN 1 AND 2000),
    client_action_id uuid,
    payload_hash text NOT NULL DEFAULT '',
    created_at timestamptz NOT NULL,
    UNIQUE(actor_id,client_action_id)
);
CREATE INDEX trust_audit_queue ON trust_audit(created_at DESC,id DESC);
