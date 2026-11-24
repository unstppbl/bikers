CREATE TABLE ride_polls (
    id uuid PRIMARY KEY,
    organizer_id uuid NOT NULL REFERENCES users(id),
    city text NOT NULL CHECK (octet_length(city) BETWEEN 1 AND 160),
    description text NOT NULL CHECK (char_length(description) BETWEEN 1 AND 500 AND octet_length(description) <= 1000),
    selected_option_id uuid,
    created_at timestamptz NOT NULL
);
CREATE INDEX ride_polls_owner_created ON ride_polls(organizer_id, created_at);

CREATE TABLE ride_poll_options (
    poll_id uuid NOT NULL REFERENCES ride_polls(id) ON DELETE CASCADE,
    id uuid NOT NULL,
    starts_at timestamptz NOT NULL,
    PRIMARY KEY (poll_id, id),
    UNIQUE (poll_id, starts_at)
);
ALTER TABLE ride_polls ADD CONSTRAINT ride_poll_selected_option
    FOREIGN KEY (id, selected_option_id) REFERENCES ride_poll_options(poll_id, id);

CREATE TABLE ride_poll_votes (
    poll_id uuid NOT NULL,
    voter_hash bytea NOT NULL CHECK (octet_length(voter_hash) = 32),
    option_id uuid NOT NULL,
    PRIMARY KEY (poll_id, voter_hash, option_id),
    FOREIGN KEY (poll_id, option_id) REFERENCES ride_poll_options(poll_id, id) ON DELETE CASCADE
);
CREATE INDEX ride_poll_votes_option ON ride_poll_votes(poll_id, option_id);
