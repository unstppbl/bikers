package httpapi

import (
	"errors"
	"html/template"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"bikers.kz/backend/internal/models"
	"github.com/google/uuid"
)

var previewTemplate = template.Must(template.New("preview").Parse(`<!doctype html>
<html lang="en"><head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width,initial-scale=1,viewport-fit=cover">
<title>{{.Title}} · BIKERS.KZ</title>
<link rel="canonical" href="{{.URL}}">
<meta name="description" content="{{.Description}}">
<meta property="og:site_name" content="BIKERS.KZ">
<meta property="og:type" content="website">
<meta property="og:url" content="{{.URL}}">
<meta property="og:title" content="{{.Title}}">
<meta property="og:description" content="{{.Description}}">
{{if .CoverURL}}<meta property="og:image" content="{{.CoverURL}}">
<meta property="og:image:width" content="{{.CoverWidth}}">
<meta property="og:image:height" content="{{.CoverHeight}}">{{end}}
</head><body><div id="root"><h1>{{.Title}}</h1><p>{{.Description}}</p></div>{{.Assets}}</body></html>`))
var assetTagPattern = regexp.MustCompile(`(?s)<script\b[^>]*>.*?</script>|<link\b[^>]*>`)

func buildAssetTags(index string) []string { return assetTagPattern.FindAllString(index, -1) }

type ridePreview struct {
	Title, Description, URL, CoverURL string
	CoverWidth, CoverHeight           int
	Assets                            template.HTML
}

func (s *Server) publicHTML(w http.ResponseWriter, r *http.Request) {
	// Moderation and cover replacements take effect on the next fetch.
	w.Header().Set("Cache-Control", "no-store")
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
	preview := ridePreview{
		Title:       "Ride in " + v.City,
		Description: v.Date + " · " + v.Description,
		URL:         s.origin.ResolveReference(&url.URL{Path: "/rides/" + v.ID}).String(),
	}
	if v.Status == "cancelled" {
		preview.Title = "Cancelled ride in " + v.City
	}
	if v.Cover != nil {
		preview.CoverURL = s.origin.ResolveReference(&url.URL{Path: v.Cover.URL}).String()
		preview.CoverWidth, preview.CoverHeight = v.Cover.Width, v.Cover.Height
	}
	// Trusted build assets are retained, while the HTML itself contains only public preview data.
	var assets strings.Builder
	if b, e := os.ReadFile(filepath.Join(s.settings.WebDistDir, "index.html")); e == nil {
		for _, tag := range buildAssetTags(string(b)) {
			assets.WriteString(tag)
		}
	}
	preview.Assets = template.HTML(assets.String())
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_ = previewTemplate.Execute(w, preview)
}
