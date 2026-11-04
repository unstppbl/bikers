package httpapi

import (
	"bikers.kz/backend/internal/models"
	"errors"
	"github.com/google/uuid"
	"html/template"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

var previewTemplate = template.Must(template.New("preview").Parse(`<!doctype html><html><head><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1"><title>BIKERS.KZ</title><meta name="description" content="{{.Description}}"><meta property="og:title" content="BIKERS.KZ"><meta property="og:description" content="{{.City}} · {{.Date}} · {{.Description}}"></head><body><div id="root"><p>{{.City}} · {{.Date}}</p><p>{{.Description}}</p></div>{{.Assets}}</body></html>`))
var assetTagPattern = regexp.MustCompile(`(?s)<script\b[^>]*>.*?</script>|<link\b[^>]*>`)

func buildAssetTags(index string) []string { return assetTagPattern.FindAllString(index, -1) }
func (s *Server) publicHTML(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimPrefix(r.URL.Path, "/rides/")
	if _, e := uuid.Parse(id); e != nil {
		http.NotFound(w, r)
		return
	}
	v, e := s.managers.Rides.Preview(r.Context(), id)
	var fault *models.Fault
	if errors.As(e, &fault) && fault.Code == "not_found" {
		http.NotFound(w, r)
		return
	}
	if e != nil {
		http.Error(w, "Unavailable", 503)
		return
	}
	// Trusted build assets are retained, while the HTML itself contains only public preview data.
	var assets strings.Builder
	if b, e := os.ReadFile(filepath.Join(s.settings.WebDistDir, "index.html")); e == nil {
		for _, tag := range buildAssetTags(string(b)) {
			assets.WriteString(tag)
		}
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-cache")
	_ = previewTemplate.Execute(w, struct {
		City, Date, Description string
		Assets                  template.HTML
	}{v.City, v.Date, v.Description, template.HTML(assets.String())})
}
