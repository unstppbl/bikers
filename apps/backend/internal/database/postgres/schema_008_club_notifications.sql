ALTER TABLE notifications ALTER COLUMN ride_id DROP NOT NULL;
ALTER TABLE notifications ADD COLUMN club_id uuid REFERENCES clubs(id);

ALTER TABLE notifications DROP CONSTRAINT notifications_kind_check;
ALTER TABLE notifications ADD CONSTRAINT notifications_kind_check CHECK (
 kind IN ('ride.updated','ride.cancelled','ride.announcement',
          'participation.requested','participation.approved','participation.rejected','participation.removed',
          'club.membership.requested','club.membership.approved','club.membership.rejected','club.membership.removed')
);
ALTER TABLE notifications ADD CONSTRAINT notifications_target_scope CHECK (
 (ride_id IS NOT NULL AND club_id IS NULL
  AND kind IN ('ride.updated','ride.cancelled','ride.announcement',
               'participation.requested','participation.approved','participation.rejected','participation.removed'))
 OR
 (club_id IS NOT NULL AND ride_id IS NULL
  AND kind IN ('club.membership.requested','club.membership.approved','club.membership.rejected','club.membership.removed'))
);
