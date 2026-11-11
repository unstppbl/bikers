package postgres

import (
	"bikers.kz/backend/internal/database"
	"bikers.kz/backend/internal/models"
)

// Target maps only the typed reported object and its persisted access context.
// Authorization and visibility decisions remain in the Trust manager.
func (t *trustTransaction) Target(kind, id, viewer string, content bool) (models.TrustTargetRecord, error) {
	out := models.TrustTargetRecord{Target: models.TrustTarget{Type: kind, ID: id}, Current: true}
	v := &out.Target
	var err error
	switch kind {
	case "user":
		err = t.tx.QueryRow(t.ctx, `SELECT id::text,display_name,status FROM users WHERE id=$1`, id).Scan(&v.AuthorID, &v.DisplayName, &out.State)
		out.OwnerID = v.AuthorID
	case "ride":
		err = t.tx.QueryRow(t.ctx, `SELECT organizer_id::text,city,to_char(starts_at AT TIME ZONE timezone,'YYYY-MM-DD'),description,status,COALESCE((SELECT state FROM participation WHERE ride_id=r.id AND user_id=NULLIF($2,'')::uuid),'') FROM rides r WHERE id=$1`, id, viewer).Scan(&v.AuthorID, &v.City, &v.Date, &v.Text, &v.Status, &out.Participation)
		out.OwnerID = v.AuthorID
	case "club":
		err = t.tx.QueryRow(t.ctx, `SELECT owner_id::text,name,city,description,status FROM clubs WHERE id=$1`, id).Scan(&v.AuthorID, &v.DisplayName, &v.City, &v.Text, &v.Status)
		out.OwnerID = v.AuthorID
	case "bike":
		err = t.tx.QueryRow(t.ctx, `SELECT owner_id::text,make||' '||model||' '||year::text,visibility FROM bikes WHERE id=$1`, id).Scan(&v.AuthorID, &v.Text, &out.Visibility)
		out.OwnerID = v.AuthorID
	case "route":
		err = t.tx.QueryRow(t.ctx, `SELECT r.owner_id::text,r.title,r.city,r.description,u.status,
 CASE WHEN r.published AND NOT r.hidden THEN 'public' ELSE 'private' END,
 NOT EXISTS(SELECT 1 FROM moderation_hidden h WHERE h.target_type='user' AND h.target_id=r.owner_id)
 FROM community_routes r JOIN users u ON u.id=r.owner_id WHERE r.id=$1`, id).Scan(&v.AuthorID, &v.DisplayName, &v.City, &v.Text, &out.AuthorStatus, &out.Visibility, &out.Current)
		out.OwnerID = v.AuthorID
	case "route_condition":
		err = t.tx.QueryRow(t.ctx, `SELECT c.author_id::text,c.route_id::text,c.description,u.display_name,r.owner_id::text,u.status,
 CASE WHEN r.published AND NOT r.hidden AND NOT c.hidden AND owner.status='active' THEN 'public' ELSE 'private' END,
 owner.status='active'
 AND NOT EXISTS(SELECT 1 FROM moderation_hidden h WHERE h.target_type='user' AND h.target_id IN (c.author_id,r.owner_id))
 AND NOT EXISTS(SELECT 1 FROM user_blocks b WHERE (b.blocker_id=NULLIF($2,'')::uuid AND b.blocked_id=r.owner_id) OR (b.blocker_id=r.owner_id AND b.blocked_id=NULLIF($2,'')::uuid))
 FROM route_conditions c JOIN community_routes r ON r.id=c.route_id JOIN users u ON u.id=c.author_id JOIN users owner ON owner.id=r.owner_id WHERE c.id=$1`, id, viewer).Scan(&v.AuthorID, &v.ScopeID, &v.Text, &v.DisplayName, &out.OwnerID, &out.AuthorStatus, &out.Visibility, &out.Current)
	case "message":
		err = t.tx.QueryRow(t.ctx, `SELECT m.sender_id::text,m.ride_id::text,m.body,u.display_name,r.organizer_id::text,COALESCE(p.state,'') FROM messages m JOIN users u ON u.id=m.sender_id JOIN rides r ON r.id=m.ride_id LEFT JOIN participation p ON p.ride_id=r.id AND p.user_id=NULLIF($2,'')::uuid WHERE m.id=$1`, id, viewer).Scan(&v.AuthorID, &v.ScopeID, &v.Text, &v.DisplayName, &out.OwnerID, &out.Participation)
	case "announcement":
		err = t.tx.QueryRow(t.ctx, `SELECT a.owner_id::text,a.ride_id::text,a.body,u.display_name,r.organizer_id::text,COALESCE(p.state,'') FROM announcements a JOIN users u ON u.id=a.owner_id JOIN rides r ON r.id=a.ride_id LEFT JOIN participation p ON p.ride_id=r.id AND p.user_id=NULLIF($2,'')::uuid WHERE a.id=$1`, id, viewer).Scan(&v.AuthorID, &v.ScopeID, &v.Text, &v.DisplayName, &out.OwnerID, &out.Participation)
	case "bike_photo":
		err = t.tx.QueryRow(t.ctx, `SELECT p.owner_id::text,p.bike_id::text,b.owner_id::text,b.visibility,p.state,CASE WHEN $2 THEN p.content ELSE NULL END FROM bike_photos p JOIN bikes b ON b.id=p.bike_id WHERE p.id=$1`, id, content).Scan(&v.AuthorID, &v.ScopeID, &out.OwnerID, &out.Visibility, &out.State, &out.Content)
		v.MediaURL = "/api/v1/bikes/" + v.ScopeID + "/photos/" + id + "/content"
	case "club_image":
		err = t.tx.QueryRow(t.ctx, `SELECT p.owner_id::text,p.club_id::text,c.owner_id::text,p.state,COALESCE(c.current_image_id=p.id,false),COALESCE((SELECT state FROM club_memberships WHERE club_id=c.id AND user_id=NULLIF($3,'')::uuid),''),CASE WHEN $2 THEN p.content ELSE NULL END FROM club_images p JOIN clubs c ON c.id=p.club_id WHERE p.id=$1`, id, content, viewer).Scan(&v.AuthorID, &v.ScopeID, &out.OwnerID, &out.State, &out.Current, &out.Membership, &out.Content)
		v.MediaURL = "/api/v1/clubs/" + v.ScopeID + "/image/" + id + "/content"
	case "avatar":
		err = t.tx.QueryRow(t.ctx, `SELECT a.owner_id::text,a.state,COALESCE(u.current_avatar_id=a.id,false),u.status,(SELECT count(*) FROM participation p JOIN participation v ON v.ride_id=p.ride_id WHERE p.user_id=a.owner_id AND p.state='approved' AND v.user_id=NULLIF($2,'')::uuid AND v.state='approved'),(SELECT count(*) FROM club_memberships p JOIN club_memberships v ON v.club_id=p.club_id WHERE p.user_id=a.owner_id AND p.state='approved' AND v.user_id=NULLIF($2,'')::uuid AND v.state='approved'),CASE WHEN $3 THEN a.content ELSE NULL END FROM avatars a JOIN users u ON u.id=a.owner_id WHERE a.id=$1`, id, viewer, content).Scan(&v.AuthorID, &out.State, &out.Current, &out.AuthorStatus, &out.SharedRides, &out.SharedClubs, &out.Content)
		out.OwnerID = v.AuthorID
	case "ride_photo":
		err = t.tx.QueryRow(t.ctx, `SELECT a.owner_id::text,a.ride_id::text,a.state,r.organizer_id::text,COALESCE(p.state,''),CASE WHEN $3 THEN a.content ELSE NULL END FROM ride_photos a JOIN rides r ON r.id=a.ride_id LEFT JOIN participation p ON p.ride_id=r.id AND p.user_id=NULLIF($2,'')::uuid WHERE a.id=$1`, id, viewer, content).Scan(&v.AuthorID, &v.ScopeID, &out.State, &out.OwnerID, &out.Participation, &out.Content)
		v.MediaURL = "/api/v1/rides/" + v.ScopeID + "/album/photos/" + id + "/content"
	case "ride_video":
		err = t.tx.QueryRow(t.ctx, `SELECT a.owner_id::text,a.ride_id::text,a.title,a.url,r.organizer_id::text,COALESCE(p.state,'') FROM ride_video_links a JOIN rides r ON r.id=a.ride_id LEFT JOIN participation p ON p.ride_id=r.id AND p.user_id=NULLIF($2,'')::uuid WHERE a.id=$1`, id, viewer).Scan(&v.AuthorID, &v.ScopeID, &v.Text, &v.MediaURL, &out.OwnerID, &out.Participation)
	default:
		return out, database.ErrNotFound
	}
	return out, repositoryError(err)
}
