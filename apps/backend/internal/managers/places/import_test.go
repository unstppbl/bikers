package places

import (
	"context"
	"crypto/sha256"
	"fmt"
	"strings"
	"testing"
	"time"
)

func TestImportRejectsEntireInvalidArtifactBeforeRepositoryAccess(t *testing.T) {
	m := New(nil, "", func() time.Time { return time.Date(2026, 11, 3, 0, 0, 0, 0, time.UTC) })
	good := `{"source_ref":"node/123","source_date":"2026-11-02","name":"Fuel","category":"fuel","lat":43.2,"lon":76.9}`
	for _, body := range []string{"", good + "\n" + good, good + "\n" + `{"source_ref":"node/456","name":"Bad","category":"fuel","lat":0,"lon":0}`, strings.Replace(good, "2026-11-02", "2026-11-04", 1), strings.Replace(good, "node/123", "node/0", 1), strings.Replace(good, "\"name\":", "\"unknown\":1,\"name\":", 1), good + " {}"} {
		digest := fmt.Sprintf("%x", sha256.Sum256([]byte(body)))
		if _, err := m.Import(context.Background(), strings.NewReader(body), digest); err == nil {
			t.Fatal("invalid import accepted", body)
		}
	}
	if _, err := m.Import(context.Background(), strings.NewReader(good), strings.Repeat("0", 64)); err == nil {
		t.Fatal("checksum mismatch accepted")
	}
	if _, err := m.Import(context.Background(), strings.NewReader(good), "bad"); err == nil {
		t.Fatal("missing valid checksum accepted")
	}
}
