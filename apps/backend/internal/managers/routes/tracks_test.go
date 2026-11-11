package routes

import (
	"math"
	"testing"
	"time"

	"bikers.kz/backend/internal/models"
	"github.com/google/uuid"
)

func TestGPSRecordingPreservesPausesWithoutJoiningGaps(t *testing.T) {
	in := models.TrackInput{Name: " Mountain ride ", Segments: [][][2]float64{{{76.89, 43.24}, {76.90, 43.24}}, {{80, 50}, {80.01, 50}}}, DurationSeconds: 120}
	out, meters, err := validateTrack(in)
	if err != nil {
		t.Fatal(err)
	}
	if out.Name != "Mountain ride" || len(out.Segments) != 2 || meters < 1500 || meters > 1700 {
		t.Fatalf("gaps were joined or incorrect normalization: %+v distance=%f", out, meters)
	}
	if across := models.TrackDistanceMeters([][][2]float64{{in.Segments[0][1], in.Segments[1][0]}}); across < 100000 {
		t.Fatal("test fixture must have a large signal gap")
	}
}
func TestGPSRecordingRejectsUnboundedOrInvalidPayloads(t *testing.T) {
	good := models.TrackInput{Name: "Ride", Segments: [][][2]float64{{{76.89, 43.24}, {76.90, 43.24}}}, DurationSeconds: 1}
	cases := []models.TrackInput{{Name: "x", DurationSeconds: 1}, {Name: "x", Segments: [][][2]float64{{{76.89, 43.24}}}, DurationSeconds: 1}, {Name: "x", Segments: [][][2]float64{{}}, DurationSeconds: 1}, {Name: "x", Segments: [][][2]float64{{{math.NaN(), 43}, {76, 43}}}, DurationSeconds: 1}, {Name: "x", Segments: [][][2]float64{{{43, 76}, {44, 76}}}, DurationSeconds: 1}}
	for _, duration := range []float64{0, -1, math.NaN(), math.Inf(1), 604801} {
		v := good
		v.DurationSeconds = duration
		cases = append(cases, v)
	}
	v := good
	v.Segments = [][][2]float64{make([][2]float64, MaximumTrackPoints+1)}
	cases = append(cases, v)
	for i, in := range cases {
		if _, _, err := validateTrack(in); err == nil {
			t.Fatalf("invalid recording %d accepted", i)
		}
	}
}
func TestConditionExpiryAndResolutionUseObservationTime(t *testing.T) {
	at := time.Date(2026, 11, 11, 9, 0, 0, 0, time.UTC)
	c := models.RouteCondition{RouteConditionInput: models.RouteConditionInput{ObservedAt: at.Add(-30 * 24 * time.Hour)}}
	if got := conditionState(c, at); got.State != "expired" || !got.ExpiresAt.Equal(at) {
		t.Fatal("exact expiry boundary", got)
	}
	c.ObservedAt = at.Add(-time.Hour)
	if conditionState(c, at).State != "recent" {
		t.Fatal("fresh report expired")
	}
	c.Resolved = true
	if conditionState(c, at).State != "resolved" {
		t.Fatal("resolved state ignored")
	}
	c.ObservedAt = at.Add(time.Second)
	if _, err := validateCondition(c.RouteConditionInput, at); err == nil {
		t.Fatal("future report accepted")
	}
}
func TestCursorBindsActorFiltersAndPageSize(t *testing.T) {
	id := uuid.NewString()
	b := binding(ListInput{Scope: "public", City: "Almaty", Limit: 20})
	raw := encode(id, b)
	if got, err := decode(raw, b); err != nil || got != id {
		t.Fatal(got, err)
	}
	for _, other := range []string{binding(ListInput{Scope: "mine", City: "Almaty", Limit: 20}), binding(ListInput{Scope: "public", City: "Astana", Limit: 20}), binding(ListInput{Scope: "public", City: "Almaty", Limit: 100})} {
		if _, err := decode(raw, other); err == nil {
			t.Fatal("cursor crossed filters")
		}
	}
}
