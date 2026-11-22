package httpapi

import (
	"encoding/xml"
	"mime"
	"net/http"
)

type gpxTrackPoint struct {
	Latitude  float64 `xml:"lat,attr"`
	Longitude float64 `xml:"lon,attr"`
}

type gpxTrackSegment struct {
	Points []gpxTrackPoint `xml:"trkpt"`
}

type gpxTrack struct {
	Name     string            `xml:"name"`
	Segments []gpxTrackSegment `xml:"trkseg"`
}

type gpxDocument struct {
	XMLName xml.Name `xml:"gpx"`
	NS      string   `xml:"xmlns,attr"`
	Version string   `xml:"version,attr"`
	Creator string   `xml:"creator,attr"`
	Track   gpxTrack `xml:"trk"`
}

func (s *Server) communityRouteGPX(w http.ResponseWriter, r *http.Request) {
	actor, ok := s.routeReader(w, r)
	if !ok {
		return
	}
	route, err := s.managers.Routes.ViewPublic(r.Context(), actor, r.PathValue("routeID"))
	if err != nil {
		failure(w, err)
		return
	}
	track := gpxTrack{Name: route.Title, Segments: make([]gpxTrackSegment, len(route.Segments))}
	for i, line := range route.Segments {
		segment := gpxTrackSegment{Points: make([]gpxTrackPoint, len(line))}
		for j, p := range line {
			segment.Points[j] = gpxTrackPoint{Latitude: p[1], Longitude: p[0]}
		}
		track.Segments[i] = segment
	}
	body, err := xml.Marshal(gpxDocument{NS: "http://www.topografix.com/GPX/1/1", Version: "1.1", Creator: "BIKERS.KZ", Track: track})
	if err != nil {
		internal(w)
		return
	}
	w.Header().Set("Content-Type", "application/gpx+xml; charset=utf-8")
	w.Header().Set("Content-Disposition", mime.FormatMediaType("attachment", map[string]string{"filename": "route-" + route.ID + ".gpx"}))
	_, _ = w.Write([]byte(xml.Header))
	_, _ = w.Write(body)
}
