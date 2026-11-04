package httpapi

import (
	"bikers.kz/backend/internal/models"
	"errors"
	"fmt"
	"html/template"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

var entityPreviewTemplate = template.Must(template.New("entity-preview").Parse(`<!doctype html><html><head><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1"><title>{{.Title}} · BIKERS.KZ</title><meta name="description" content="{{.Description}}"><meta property="og:title" content="{{.Title}} · BIKERS.KZ"><meta property="og:description" content="{{.Description}}"></head><body><div id="root"><h1>{{.Title}}</h1><p>{{.Description}}</p></div>{{.Assets}}</body></html>`))

// Public previews deliberately request the anonymous manager view. A private
// bike receives only the generic SPA shell, equally for owners and strangers.
func (s *Server) publicEntityHTML(w http.ResponseWriter, r *http.Request, prefix, id string) bool {
	var value any
	var err error
	if prefix == "/clubs/" {
		value, err = s.managers.Clubs.View(r.Context(), id, nil)
	} else {
		value, err = s.managers.Garage.View(r.Context(), id, nil)
	}
	if err != nil {
		var fault *models.Fault
		if errors.As(err, &fault) && fault.Code == "not_found" {
			if prefix == "/bikes/" {
				return false
			}
			http.NotFound(w, r)
			return true
		}
		http.Error(w, "Unavailable", 503)
		return true
	}
	var title, description string
	switch v := value.(type) {
	case models.PublicBike:
		title = v.Make + " " + v.Model
		description = fmt.Sprintf("%s · %d", title, v.Year)
	case models.ClubSummary:
		title = v.Name
		description = v.City + " · " + v.Description
	default:
		http.Error(w, "Unavailable", 503)
		return true
	}
	var assets strings.Builder
	if index, err := os.ReadFile(filepath.Join(s.settings.WebDistDir, "index.html")); err == nil {
		for _, tag := range buildAssetTags(string(index)) {
			assets.WriteString(tag)
		}
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-cache")
	_ = entityPreviewTemplate.Execute(w, struct {
		Title, Description string
		Assets             template.HTML
	}{title, description, template.HTML(assets.String())})
	return true
}
