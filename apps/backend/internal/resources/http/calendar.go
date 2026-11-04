package httpapi

import (
	"mime"
	"net/http"
	"net/url"
	"strings"
	"time"

	"bikers.kz/backend/internal/models"
	ics "github.com/arran4/golang-ical"
)

func (s *Server) rideCalendar(w http.ResponseWriter, r *http.Request) {
	session := s.require(w, r, false)
	if session == nil {
		return
	}
	id, ok := idParam(w, r, "id")
	if !ok {
		return
	}
	ride, err := s.managers.Rides.Calendar(r.Context(), session.User, id)
	if err != nil {
		failure(w, err)
		return
	}
	link, err := url.JoinPath(s.origin.String(), "rides", ride.ID)
	if err != nil {
		internal(w)
		return
	}
	w.Header().Set("Content-Type", "text/calendar; charset=utf-8")
	w.Header().Set("Content-Disposition", mime.FormatMediaType("attachment", map[string]string{"filename": "ride-" + ride.ID + ".ics"}))
	w.Header().Set("Cache-Control", "no-store")
	_, _ = w.Write([]byte(serializeRideCalendar(ride, link, session.User.Profile.Locale, time.Now().UTC())))
}

// Export an event at the known departure time, without inventing a duration or
// end time. A stable UID and plan sequence let a later download identify the
// same event, including cancellation; downloaded files are not a live feed.
func serializeRideCalendar(ride models.Ride, link, locale string, at time.Time) string {
	calendar := ics.NewCalendar()
	calendar.SetProductId("-//BIKERS.KZ//Rides//EN")
	calendar.SetMethod(ics.MethodPublish)
	event := calendar.AddEvent(ride.ID + "@bikers.kz")
	event.SetDtStampTime(at)
	event.SetSequence(int(ride.PlanVersion))
	event.SetStartAt(ride.StartsAt.UTC())
	event.SetSummary(ride.Title)
	event.SetLocation(ride.MeetingPoint)
	event.SetClass(ics.ClassificationPrivate)
	event.SetURL(link)
	event.SetStatus(ics.ObjectStatusConfirmed)
	if ride.Status == "cancelled" {
		event.SetStatus(ics.ObjectStatusCancelled)
	}
	labels := map[string][3]string{
		"en": {"Destination", "Riding expectations", "Departure timezone"},
		"ru": {"Пункт назначения", "Требования к поездке", "Часовой пояс отправления"},
		"kk": {"Баратын жер", "Сапар талаптары", "Жолға шығу уақыт белдеуі"},
	}
	words, ok := labels[locale]
	if !ok {
		words = labels["en"]
	}
	parts := []string{ride.Description}
	if ride.Destination != "" {
		parts = append(parts, words[0]+": "+ride.Destination)
	}
	if ride.Expectations != "" {
		parts = append(parts, words[1]+": "+ride.Expectations)
	}
	parts = append(parts, words[2]+": "+ride.Timezone)
	if ride.RouteURL != "" {
		parts = append(parts, ride.RouteURL)
	}
	event.SetDescription(strings.Join(parts, "\n\n"))
	return calendar.Serialize(ics.WithNewLineWindows)
}
