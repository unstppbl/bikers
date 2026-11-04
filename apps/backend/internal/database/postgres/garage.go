package postgres

import (
	"context"

	"bikers.kz/backend/internal/database"
	"bikers.kz/backend/internal/models"
	"github.com/jackc/pgx/v5"
)

const bikeColumns = "id::text,owner_id::text,make,model,model_id,year,visibility,version,created_at,updated_at"
const maintenanceColumns = "id::text,bike_id::text,to_char(service_date,'YYYY-MM-DD'),mileage_km,work_performed,version,created_at,updated_at"

func scanBike(row pgx.Row) (models.Bike, error) {
	var bike models.Bike
	err := row.Scan(&bike.ID, &bike.OwnerID, &bike.Make, &bike.Model, &bike.ModelID, &bike.Year, &bike.Visibility, &bike.Version, &bike.CreatedAt, &bike.UpdatedAt)
	return bike, repositoryError(err)
}
func scanMaintenance(row pgx.Row) (models.MaintenanceEntry, error) {
	var entry models.MaintenanceEntry
	err := row.Scan(&entry.ID, &entry.BikeID, &entry.ServiceDate, &entry.MileageKM, &entry.WorkPerformed, &entry.Version, &entry.CreatedAt, &entry.UpdatedAt)
	return entry, repositoryError(err)
}

func (s *Store) Bikes(ctx context.Context, ownerID, after string, limit int) ([]models.Bike, error) {
	sql := "SELECT " + bikeColumns + " FROM bikes WHERE owner_id=$1 AND EXISTS(SELECT 1 FROM users WHERE users.id=bikes.owner_id AND users.status='active')"
	args := []any{ownerID}
	if after != "" {
		sql += " AND id>$2 ORDER BY id LIMIT $3"
		args = append(args, after, limit)
	} else {
		sql += " ORDER BY id LIMIT $2"
		args = append(args, limit)
	}
	rows, err := s.pool.Query(ctx, sql, args...)
	if err != nil {
		return nil, repositoryError(err)
	}
	defer rows.Close()
	out := []models.Bike{}
	for rows.Next() {
		bike, err := scanBike(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, bike)
	}
	if err = rows.Err(); err != nil {
		return nil, repositoryError(err)
	}
	return out, nil
}
func (s *Store) Bike(ctx context.Context, id string) (models.Bike, error) {
	return scanBike(s.pool.QueryRow(ctx, "SELECT "+bikeColumns+" FROM bikes WHERE id=$1", id))
}

func (s *Store) PublicBikes(ctx context.Context, query database.PublicBikeQuery) ([]models.Bike, error) {
	rows, err := s.pool.Query(ctx, `SELECT `+bikeColumns+` FROM bikes
 WHERE visibility='public'
   AND NOT EXISTS(SELECT 1 FROM moderation_hidden h WHERE h.target_type='bike' AND h.target_id=bikes.id)
   AND ($1='' OR strpos(lower(make||' '||model),lower($1))>0)
   AND ($2='' OR id>NULLIF($2,'')::uuid)
 ORDER BY id LIMIT $3`, query.Search, query.AfterID, query.Limit)
	if err != nil {
		return nil, repositoryError(err)
	}
	defer rows.Close()
	out := []models.Bike{}
	for rows.Next() {
		bike, err := scanBike(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, bike)
	}
	return out, repositoryError(rows.Err())
}

type garageOwnerTransaction struct {
	ctx   context.Context
	tx    pgx.Tx
	owner models.GarageOwner
}

func (t *garageOwnerTransaction) Owner() models.GarageOwner { return t.owner }
func (t *garageOwnerTransaction) CreateBike(bike models.Bike) (models.Bike, error) {
	return scanBike(t.tx.QueryRow(t.ctx, `INSERT INTO bikes(id,owner_id,make,model,model_id,year,visibility,version,created_at,updated_at) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10) RETURNING `+bikeColumns, bike.ID, t.owner.ID, bike.Make, bike.Model, bike.ModelID, bike.Year, bike.Visibility, bike.Version, bike.CreatedAt, bike.UpdatedAt))
}
func lockedGarageOwner(ctx context.Context, tx pgx.Tx, id string) (models.GarageOwner, error) {
	var owner models.GarageOwner
	err := tx.QueryRow(ctx, "SELECT id::text,status FROM users WHERE id=$1 FOR UPDATE", id).Scan(&owner.ID, &owner.Status)
	return owner, repositoryError(err)
}

func (s *Store) WithGarageOwner(ctx context.Context, ownerID string, decide database.GarageOwnerDecision) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return repositoryError(err)
	}
	defer tx.Rollback(ctx)
	if _, err = tx.Exec(ctx, "SELECT pg_advisory_xact_lock($1)", trustAdvisoryLock); err != nil {
		return repositoryError(err)
	}
	owner, err := lockedGarageOwner(ctx, tx, ownerID)
	if err != nil {
		return err
	}
	if err = decide(&garageOwnerTransaction{ctx: ctx, tx: tx, owner: owner}); err != nil {
		return err
	}
	return repositoryError(tx.Commit(ctx))
}

type bikeTransaction struct {
	garageOwnerTransaction
	bike models.Bike
}

func (t *bikeTransaction) Bike() models.Bike { return t.bike }
func (s *Store) WithBike(ctx context.Context, id string, decide database.BikeDecision) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return repositoryError(err)
	}
	defer tx.Rollback(ctx)
	if _, err = tx.Exec(ctx, "SELECT pg_advisory_xact_lock($1)", trustAdvisoryLock); err != nil {
		return repositoryError(err)
	}
	// Resolve the aggregate owner, then use a consistent owner -> bike order.
	// Owner identity is immutable through this repository's bike update operation.
	var ownerID string
	if err = tx.QueryRow(ctx, "SELECT owner_id::text FROM bikes WHERE id=$1", id).Scan(&ownerID); err != nil {
		return repositoryError(err)
	}
	owner, err := lockedGarageOwner(ctx, tx, ownerID)
	if err != nil {
		return err
	}
	bike, err := scanBike(tx.QueryRow(ctx, "SELECT "+bikeColumns+" FROM bikes WHERE id=$1 FOR UPDATE", id))
	if err != nil {
		return err
	}
	if bike.OwnerID != owner.ID {
		return database.ErrConflict
	}
	locked := &bikeTransaction{garageOwnerTransaction: garageOwnerTransaction{ctx: ctx, tx: tx, owner: owner}, bike: bike}
	if err = decide(locked); err != nil {
		return err
	}
	return repositoryError(tx.Commit(ctx))
}
func (t *bikeTransaction) UpdateBike(bike models.Bike) (models.Bike, error) {
	stored, err := scanBike(t.tx.QueryRow(t.ctx, `UPDATE bikes SET make=$2,model=$3,model_id=$4,year=$5,visibility=$6,version=$7,updated_at=$8 WHERE id=$1 RETURNING `+bikeColumns, t.bike.ID, bike.Make, bike.Model, bike.ModelID, bike.Year, bike.Visibility, bike.Version, bike.UpdatedAt))
	if err == nil {
		t.bike = stored
	}
	return stored, err
}
func (t *bikeTransaction) DeleteBike() error {
	_, err := t.tx.Exec(t.ctx, "DELETE FROM bikes WHERE id=$1", t.bike.ID)
	return repositoryError(err)
}

func (t *bikeTransaction) Maintenance(after models.MaintenanceCursor, limit int) ([]models.MaintenanceEntry, error) {
	sql := "SELECT " + maintenanceColumns + " FROM maintenance_entries WHERE bike_id=$1"
	args := []any{t.bike.ID}
	if after.ID != "" {
		sql += " AND (service_date,id)<($2::date,$3::uuid) ORDER BY service_date DESC,id DESC LIMIT $4"
		args = append(args, after.Date, after.ID, limit)
	} else {
		sql += " ORDER BY service_date DESC,id DESC LIMIT $2"
		args = append(args, limit)
	}
	rows, err := t.tx.Query(t.ctx, sql, args...)
	if err != nil {
		return nil, repositoryError(err)
	}
	defer rows.Close()
	out := []models.MaintenanceEntry{}
	for rows.Next() {
		entry, err := scanMaintenance(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, entry)
	}
	if err = rows.Err(); err != nil {
		return nil, repositoryError(err)
	}
	return out, nil
}
func (t *bikeTransaction) Entry(id string) (models.MaintenanceEntry, error) {
	return scanMaintenance(t.tx.QueryRow(t.ctx, "SELECT "+maintenanceColumns+" FROM maintenance_entries WHERE bike_id=$1 AND id=$2 FOR UPDATE", t.bike.ID, id))
}
func (t *bikeTransaction) CreateEntry(entry models.MaintenanceEntry) (models.MaintenanceEntry, error) {
	return scanMaintenance(t.tx.QueryRow(t.ctx, `INSERT INTO maintenance_entries(id,bike_id,service_date,mileage_km,work_performed,version,created_at,updated_at) VALUES($1,$2,$3::date,$4,$5,$6,$7,$8) RETURNING `+maintenanceColumns, entry.ID, t.bike.ID, entry.ServiceDate, entry.MileageKM, entry.WorkPerformed, entry.Version, entry.CreatedAt, entry.UpdatedAt))
}
func (t *bikeTransaction) UpdateEntry(entry models.MaintenanceEntry) (models.MaintenanceEntry, error) {
	return scanMaintenance(t.tx.QueryRow(t.ctx, `UPDATE maintenance_entries SET service_date=$3::date,mileage_km=$4,work_performed=$5,version=$6,updated_at=$7 WHERE bike_id=$1 AND id=$2 RETURNING `+maintenanceColumns, t.bike.ID, entry.ID, entry.ServiceDate, entry.MileageKM, entry.WorkPerformed, entry.Version, entry.UpdatedAt))
}
func (t *bikeTransaction) DeleteEntry(id string) error {
	_, err := t.tx.Exec(t.ctx, "DELETE FROM maintenance_entries WHERE bike_id=$1 AND id=$2", t.bike.ID, id)
	return repositoryError(err)
}

var _ database.GarageRepository = (*Store)(nil)
var _ database.GarageOwnerTransaction = (*garageOwnerTransaction)(nil)
var _ database.BikeTransaction = (*bikeTransaction)(nil)

func (t *bikeTransaction) ContentHidden(kind, id string) (bool, error) {
	return trustContentHidden(t.ctx, t.tx, kind, id)
}
