package media

import (
	"bikers.kz/backend/internal/models"
	"errors"
	"strings"
	"testing"
)

func TestVideoLinksRemainHTTPSMetadataWithoutCredentials(t *testing.T) {
	input, err := validateVideo(models.RideVideoInput{URL: " https://video.example/watch?v=private#t=10 ", Title: " Weekend ride "})
	if err != nil || input.URL != "https://video.example/watch?v=private#t=10" || input.Title != "Weekend ride" {
		t.Fatalf("valid link changed: %+v %v", input, err)
	}
	input, err = validateVideo(models.RideVideoInput{URL: "https://video.example/watch"})
	if err != nil || input.Title != "video.example" {
		t.Fatalf("optional title fallback: %+v %v", input, err)
	}
	for _, url := range []string{"http://video.example/watch", "javascript:alert(1)", "data:text/html,video", "/private/video", "https://user:password@video.example/watch", "https://localhost/watch", "https://a.localhost/watch", "https://camera.local/watch", "https://camera.internal/watch", "https://127.0.0.1/watch", "https://[::1]/watch", "https://video.example:70000/watch", "https://video.example/" + strings.Repeat("a", 2048)} {
		_, err := validateVideo(models.RideVideoInput{URL: url, Title: "Ride"})
		var fault *models.Fault
		if !errors.As(err, &fault) || fault.Code != "invalid_video_link" {
			t.Errorf("invalid link accepted: %q %v", url, err)
		}
	}
	_, err = validateVideo(models.RideVideoInput{URL: "https://video.example/watch", Title: strings.Repeat("x", 161)})
	if err == nil {
		t.Fatal("oversized title accepted")
	}
}
