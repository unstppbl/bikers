package places

import (
	"bikers.kz/backend/internal/database"
	"bikers.kz/backend/internal/models"
	"bufio"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"github.com/google/uuid"
	"io"
	"regexp"
	"time"
)

const MaximumImportBytes = 32 << 20
const MaximumImportRows = 50_000

var osmReference = regexp.MustCompile(`^(node|way|relation)/[1-9][0-9]{0,18}$`)

// Import consumes a pinned local NDJSON artifact. All rows and its digest are
// validated before the repository starts a transaction; there is no network IO.
func (m *Manager) Import(ctx context.Context, reader io.Reader, wantSHA string) (models.PlaceImportResult, error) {
	var out models.PlaceImportResult
	want, err := hex.DecodeString(wantSHA)
	if err != nil || len(want) != sha256.Size {
		return out, models.Fail("invalid_import", "A SHA-256 digest is required")
	}
	b, err := io.ReadAll(io.LimitReader(reader, MaximumImportBytes+1))
	if err != nil {
		return out, err
	}
	if len(b) > MaximumImportBytes {
		return out, models.Fail("invalid_import", "Place import exceeds 32 MiB")
	}
	digest := sha256.Sum256(b)
	if !bytes.Equal(digest[:], want) {
		return out, models.Fail("invalid_import", "Place import checksum differs")
	}
	rows := []models.PlaceImportRecord{}
	refs := []string{}
	seen := map[string]bool{}
	scanner := bufio.NewScanner(bytes.NewReader(b))
	scanner.Buffer(make([]byte, 4096), 64<<10)
	line := 0
	for scanner.Scan() {
		line++
		if len(bytes.TrimSpace(scanner.Bytes())) == 0 {
			continue
		}
		if len(rows) >= MaximumImportRows {
			return out, models.Fail("invalid_import", "Place import exceeds 50000 rows")
		}
		var row models.PlaceImportRecord
		decoder := json.NewDecoder(bytes.NewReader(scanner.Bytes()))
		decoder.DisallowUnknownFields()
		if decoder.Decode(&row) != nil || decoder.Decode(new(any)) != io.EOF {
			return out, models.Fail("invalid_import", fmt.Sprintf("Invalid JSON at line %d", line))
		}
		row.PlaceInput, err = models.ValidatePlace(row.PlaceInput)
		if err != nil {
			return out, models.Fail("invalid_import", fmt.Sprintf("Invalid place at line %d", line))
		}
		date, e := time.Parse("2006-01-02", row.SourceDate)
		if !osmReference.MatchString(row.SourceRef) || seen[row.SourceRef] || e != nil || date.After(m.now().UTC().Truncate(24*time.Hour)) {
			return out, models.Fail("invalid_import", fmt.Sprintf("Invalid or duplicate OSM reference/date at line %d", line))
		}
		seen[row.SourceRef] = true
		rows = append(rows, row)
		refs = append(refs, row.SourceRef)
	}
	if scanner.Err() != nil {
		return out, models.Fail("invalid_import", "Place import contains an oversized or unreadable line")
	}
	if len(rows) == 0 {
		return out, models.Fail("invalid_import", "Place import has no records")
	}
	out.Total = len(rows)
	err = m.repo.WithPlaces(ctx, "", func(tx database.PlaceTransaction) error {
		existing, e := tx.PlacesBySources(refs)
		if e != nil {
			return e
		}
		changed := []models.Place{}
		at := m.now().UTC()
		for _, row := range rows {
			old, found := existing[row.SourceRef]
			if found && (old.Source != "osm" || old.Status != "published") {
				out.Preserved++
				continue
			}
			if found && old.SourceDate > row.SourceDate {
				out.Preserved++
				continue
			}
			if found && old.PlaceInput == row.PlaceInput && old.SourceDate == row.SourceDate {
				out.Unchanged++
				continue
			}
			p := models.Place{ID: uuid.NewString(), PlaceInput: row.PlaceInput, Source: "osm", SourceRef: row.SourceRef, SourceURL: "https://www.openstreetmap.org/" + row.SourceRef, SourceDate: row.SourceDate, UpdatedAt: at, Version: 1, Status: "published"}
			if found {
				p.ID = old.ID
				p.Version = old.Version + 1
				out.Updated++
			} else {
				out.Inserted++
			}
			changed = append(changed, p)
		}
		if len(changed) > 0 {
			return tx.SavePlaces(changed)
		}
		return nil
	})
	if err != nil {
		return models.PlaceImportResult{}, storageError(err)
	}
	return out, nil
}
