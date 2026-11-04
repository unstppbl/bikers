CREATE TABLE bikes (
 id uuid PRIMARY KEY,
 owner_id uuid NOT NULL REFERENCES users(id),
 make text NOT NULL,
 model text NOT NULL,
 model_id text NOT NULL DEFAULT '',
 year integer NOT NULL,
 visibility text NOT NULL DEFAULT 'private' CHECK(visibility IN ('private','public')),
 version bigint NOT NULL DEFAULT 1,
 created_at timestamptz NOT NULL,
 updated_at timestamptz NOT NULL
);
CREATE INDEX bikes_owner_id ON bikes(owner_id,id);
CREATE TABLE maintenance_entries (
 id uuid PRIMARY KEY,
 bike_id uuid NOT NULL REFERENCES bikes(id) ON DELETE CASCADE,
 service_date date NOT NULL,
 mileage_km bigint NOT NULL CHECK(mileage_km>=0),
 work_performed text NOT NULL,
 version bigint NOT NULL DEFAULT 1,
 created_at timestamptz NOT NULL,
 updated_at timestamptz NOT NULL
);
CREATE INDEX maintenance_bike_date ON maintenance_entries(bike_id,service_date DESC,id DESC);
CREATE TABLE clubs (
 id uuid PRIMARY KEY,
 owner_id uuid NOT NULL REFERENCES users(id),
 name text NOT NULL,
 city text NOT NULL,
 description text NOT NULL,
 status text NOT NULL DEFAULT 'active' CHECK(status IN ('active','closed')),
 version bigint NOT NULL DEFAULT 1,
 created_at timestamptz NOT NULL,
 updated_at timestamptz NOT NULL
);
CREATE TABLE club_memberships (
 club_id uuid NOT NULL REFERENCES clubs(id) ON DELETE CASCADE,
 user_id uuid NOT NULL REFERENCES users(id),
 state text NOT NULL CHECK(state IN ('requested','approved','rejected','left','removed')),
 created_at timestamptz NOT NULL,
 updated_at timestamptz NOT NULL,
 PRIMARY KEY(club_id,user_id)
);
CREATE INDEX club_memberships_user ON club_memberships(user_id,club_id);
CREATE TABLE club_rides (
 club_id uuid NOT NULL REFERENCES clubs(id) ON DELETE CASCADE,
 ride_id uuid NOT NULL REFERENCES rides(id) ON DELETE CASCADE,
 linked_by uuid NOT NULL REFERENCES users(id),
 linked_at timestamptz NOT NULL,
 PRIMARY KEY(club_id,ride_id)
);
