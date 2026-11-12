CREATE TABLE route_tracks (
    id uuid PRIMARY KEY,
    owner_id uuid NOT NULL REFERENCES users(id),
    name text NOT NULL CHECK(octet_length(name) BETWEEN 1 AND 200),
    segments jsonb NOT NULL CHECK(jsonb_typeof(segments)='array' AND jsonb_array_length(segments) BETWEEN 1 AND 1000 AND octet_length(segments::text)<=2097152),
    distance_meters double precision NOT NULL CHECK(distance_meters BETWEEN 0 AND 10000000),
    duration_seconds double precision NOT NULL CHECK(duration_seconds BETWEEN 1 AND 604800),
    version bigint NOT NULL CHECK(version>0),
    created_at timestamptz NOT NULL
);
CREATE INDEX route_tracks_owner ON route_tracks(owner_id,id);

CREATE TABLE community_routes (
    id uuid PRIMARY KEY,
    owner_id uuid NOT NULL REFERENCES users(id),
    title text NOT NULL CHECK(octet_length(title) BETWEEN 1 AND 200),
    city text NOT NULL CHECK(octet_length(city)<=160),
    description text NOT NULL CHECK(octet_length(description)<=4000),
    surface text NOT NULL CHECK(surface IN ('asphalt','mixed','unpaved','unknown')),
    kind text NOT NULL CHECK(kind IN ('planned','recorded')),
    published boolean NOT NULL,
    hidden boolean NOT NULL DEFAULT false,
    version bigint NOT NULL CHECK(version>0),
    points jsonb NOT NULL CHECK(jsonb_typeof(points)='array' AND jsonb_array_length(points)<=10),
    segments jsonb NOT NULL CHECK(jsonb_typeof(segments)='array' AND jsonb_array_length(segments) BETWEEN 1 AND 1000 AND octet_length(segments::text)<=4194304),
    distance_meters double precision NOT NULL CHECK(distance_meters BETWEEN 0 AND 10000000),
    duration_seconds double precision NOT NULL CHECK(duration_seconds BETWEEN 0 AND 604800),
    source_track_id uuid REFERENCES route_tracks(id) ON DELETE SET NULL,
    created_at timestamptz NOT NULL,
    updated_at timestamptz NOT NULL,
    CHECK(kind='recorded' OR (jsonb_array_length(points)>=2 AND source_track_id IS NULL))
);
CREATE INDEX community_routes_public ON community_routes(id) WHERE published AND NOT hidden;
CREATE INDEX community_routes_city ON community_routes(lower(city),id) WHERE published AND NOT hidden;
CREATE INDEX community_routes_owner ON community_routes(owner_id,id);

CREATE TABLE route_collections (
    id uuid PRIMARY KEY,
    owner_id uuid NOT NULL REFERENCES users(id),
    name text NOT NULL CHECK(octet_length(name) BETWEEN 1 AND 100),
    version bigint NOT NULL CHECK(version>0),
    created_at timestamptz NOT NULL,
    updated_at timestamptz NOT NULL
);
CREATE INDEX route_collections_owner ON route_collections(owner_id,id);
CREATE TABLE route_collection_items (
    collection_id uuid NOT NULL REFERENCES route_collections(id) ON DELETE CASCADE,
    route_id uuid NOT NULL REFERENCES community_routes(id) ON DELETE CASCADE,
    created_at timestamptz NOT NULL,
    PRIMARY KEY(collection_id,route_id)
);
CREATE INDEX route_collection_items_route ON route_collection_items(route_id);

CREATE TABLE route_conditions (
    id uuid PRIMARY KEY,
    route_id uuid NOT NULL REFERENCES community_routes(id) ON DELETE CASCADE,
    author_id uuid NOT NULL REFERENCES users(id),
    kind text NOT NULL CHECK(kind IN ('gravel','closure','roadworks','damage','other')),
    description text NOT NULL CHECK(octet_length(description) BETWEEN 1 AND 2000),
    observed_at timestamptz NOT NULL,
    resolved boolean NOT NULL,
    hidden boolean NOT NULL DEFAULT false,
    version bigint NOT NULL CHECK(version>0),
    created_at timestamptz NOT NULL,
    updated_at timestamptz NOT NULL
);
CREATE INDEX route_conditions_route ON route_conditions(route_id,id);
CREATE INDEX route_conditions_author ON route_conditions(author_id,created_at);

ALTER TABLE moderation_hidden DROP CONSTRAINT moderation_hidden_target_type_check;
ALTER TABLE moderation_hidden ADD CONSTRAINT moderation_hidden_target_type_check
    CHECK(target_type IN ('user','ride','message','club','bike','bike_photo','club_image','avatar','ride_photo','ride_video','announcement','route','route_condition'));
ALTER TABLE trust_reports DROP CONSTRAINT trust_reports_target_type_check;
ALTER TABLE trust_reports ADD CONSTRAINT trust_reports_target_type_check
    CHECK(target_type IN ('user','ride','message','club','bike','bike_photo','club_image','avatar','ride_photo','ride_video','announcement','route','route_condition'));

-- Aggregate counters contain no report text and prevent deletion bypassing daily limits.
CREATE TABLE route_daily_limits (
    owner_id uuid NOT NULL REFERENCES users(id),
    day date NOT NULL,
    conditions_created integer NOT NULL CHECK(conditions_created BETWEEN 0 AND 20),
    PRIMARY KEY(owner_id,day)
);
