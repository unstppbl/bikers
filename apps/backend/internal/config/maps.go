package config

import (
	"errors"
	"net/url"
	"path"
	"strings"
)

// Map services are operator-selected internal origins, never request parameters.
func ValidateMaps(photon, valhalla, archive string) error {
	if (photon == "") != (valhalla == "") {
		return errors.New("PHOTON_BASE_URL and VALHALLA_BASE_URL must be configured together")
	}
	for _, raw := range []string{photon, valhalla} {
		if raw == "" {
			continue
		}
		u, err := url.Parse(raw)
		if err != nil || u.Hostname() == "" || (u.Scheme != "http" && u.Scheme != "https") || u.User != nil || u.RawQuery != "" || u.ForceQuery || u.Fragment != "" || (u.Path != "" && u.Path != "/") {
			return errors.New("map service URLs must be absolute HTTP(S) origins")
		}
	}
	if archive == "" {
		return nil
	}
	u, err := url.Parse(archive)
	if err != nil || u.Scheme != "" || u.Host != "" || u.User != nil || u.RawQuery != "" || u.ForceQuery || u.Fragment != "" || u.RawPath != "" || !strings.HasPrefix(u.Path, "/map-data/") || !strings.HasSuffix(u.Path, ".pmtiles") || path.Clean(u.Path) != u.Path || len(strings.Split(u.Path, "/")) != 4 || len(u.Path) > 240 {
		return errors.New("MAPS_ARCHIVE_URL must be a versioned same-origin /map-data/{version}/{name}.pmtiles path")
	}
	return nil
}
