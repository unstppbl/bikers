ALTER TABLE users ADD COLUMN push_ride_updates boolean NOT NULL DEFAULT true;
ALTER TABLE users ADD COLUMN push_participation boolean NOT NULL DEFAULT true;
ALTER TABLE users ADD COLUMN push_club_membership boolean NOT NULL DEFAULT true;
ALTER TABLE users ADD COLUMN push_ride_reminders boolean NOT NULL DEFAULT true;

CREATE TABLE ride_reminders (
 ride_id uuid NOT NULL REFERENCES rides(id),
 user_id uuid NOT NULL REFERENCES users(id),
 day_before boolean NOT NULL DEFAULT false,
 hour_before boolean NOT NULL DEFAULT false,
 day_before_enabled_at timestamptz,
 hour_before_enabled_at timestamptz,
 participation_version bigint NOT NULL,
 updated_at timestamptz NOT NULL,
 PRIMARY KEY(ride_id,user_id),
 CHECK(day_before = (day_before_enabled_at IS NOT NULL)),
 CHECK(hour_before = (hour_before_enabled_at IS NOT NULL))
);
CREATE INDEX ride_reminders_enabled ON ride_reminders(ride_id) WHERE day_before OR hour_before;
CREATE TABLE ride_reminder_events (
 ride_id uuid NOT NULL REFERENCES rides(id),
 user_id uuid NOT NULL REFERENCES users(id),
 starts_at timestamptz NOT NULL,
 lead_minutes integer NOT NULL CHECK(lead_minutes IN (60,1440)),
 processed_at timestamptz NOT NULL,
 PRIMARY KEY(ride_id,user_id,starts_at,lead_minutes)
);

ALTER TABLE notifications ADD COLUMN reminder_starts_at timestamptz;
ALTER TABLE notifications ADD COLUMN reminder_lead_minutes integer;
ALTER TABLE notifications DROP CONSTRAINT notifications_kind_check;
ALTER TABLE notifications ADD CONSTRAINT notifications_kind_check CHECK (
 kind IN ('ride.updated','ride.cancelled','ride.announcement','ride.reminder',
          'participation.requested','participation.approved','participation.rejected','participation.removed',
          'club.membership.requested','club.membership.approved','club.membership.rejected','club.membership.removed')
);
ALTER TABLE notifications DROP CONSTRAINT notifications_target_scope;
ALTER TABLE notifications ADD CONSTRAINT notifications_target_scope CHECK (
 (ride_id IS NOT NULL AND club_id IS NULL
  AND kind IN ('ride.updated','ride.cancelled','ride.announcement','ride.reminder',
               'participation.requested','participation.approved','participation.rejected','participation.removed'))
 OR
 (club_id IS NOT NULL AND ride_id IS NULL
  AND kind IN ('club.membership.requested','club.membership.approved','club.membership.rejected','club.membership.removed'))
);
ALTER TABLE notifications ADD CONSTRAINT notifications_reminder_scope CHECK (
 (kind='ride.reminder' AND reminder_starts_at IS NOT NULL AND reminder_lead_minutes IS NOT NULL AND reminder_lead_minutes IN (60,1440))
 OR
 (kind<>'ride.reminder' AND reminder_starts_at IS NULL AND reminder_lead_minutes IS NULL)
);
