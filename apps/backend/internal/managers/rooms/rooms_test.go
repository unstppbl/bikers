package rooms

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	"bikers.kz/backend/internal/database"
	"bikers.kz/backend/internal/models"
	"github.com/google/uuid"
)

type roomRepository struct {
	ride         models.Ride
	access       models.RoomAccess
	messages     []models.Message
	next         int64
	locked       bool
	commitErr    error
	messageLimit int
	readAfter    int64
	appended     int
	readUser     string
}

func (r *roomRepository) RoomAccess(context.Context, string, string) (models.RoomAccess, error) {
	return r.access, nil
}
func (r *roomRepository) Messages(_ context.Context, _ string, user string, after int64, limit int) (models.MessagePage, error) {
	r.messageLimit, r.readAfter = limit, after
	r.readUser = user
	return models.MessagePage{NextSequence: after}, nil
}
func (r *roomRepository) WithRide(_ context.Context, ride string, decide database.RideDecision) error {
	if ride != r.ride.ID {
		return database.ErrNotFound
	}
	r.locked = true
	defer func() { r.locked = false }()
	tx := &roomTransaction{repo: r, messages: append([]models.Message{}, r.messages...), next: r.next}
	if err := decide(tx); err != nil {
		return err
	}
	if r.commitErr != nil {
		return r.commitErr
	}
	r.messages, r.next = tx.messages, tx.next
	return nil
}

type roomTransaction struct {
	repo     *roomRepository
	messages []models.Message
	next     int64
}

func (tx *roomTransaction) Ride() models.Ride                            { return tx.repo.ride }
func (tx *roomTransaction) RoomAccess(string) (models.RoomAccess, error) { return tx.repo.access, nil }
func (tx *roomTransaction) Message(sender, command string) (models.Message, error) {
	for _, message := range tx.messages {
		if message.SenderID == sender && message.ClientMessageID == command {
			return message, nil
		}
	}
	return models.Message{}, database.ErrNotFound
}
func (tx *roomTransaction) AppendMessage(message models.Message) (models.Message, error) {
	if message.Sequence != 0 {
		panic("manager allocated a persistence sequence")
	}
	tx.next++
	message.Sequence = tx.next
	tx.messages = append(tx.messages, message)
	tx.repo.appended++
	return message, nil
}
func (*roomTransaction) Participation(string) (string, error) {
	panic("unexpected participation lookup")
}
func (*roomTransaction) ApprovedCount() (int, error) { panic("unexpected capacity query") }
func (*roomTransaction) SetParticipation(string, string, time.Time) error {
	panic("unexpected participation write")
}
func (*roomTransaction) UpdateRide(models.Ride) error               { panic("unexpected plan write") }
func (*roomTransaction) Blocked(string, string) (bool, error)       { return false, nil }
func (*roomTransaction) AccountStatus(string) (string, error)       { return "active", nil }
func (*roomTransaction) ContentHidden(string, string) (bool, error) { return false, nil }
func (*roomTransaction) ParticipationUsers() ([]models.Participant, error) {
	panic("unexpected participant listing")
}
func (*roomTransaction) Notify(models.NotificationIntent) error {
	panic("unexpected notification write")
}
func (*roomTransaction) Announcement(string) (models.Announcement, error) {
	panic("unexpected announcement lookup")
}
func (*roomTransaction) Announcements(int64, int) ([]models.Announcement, error) {
	panic("unexpected announcement listing")
}
func (*roomTransaction) AppendAnnouncement(models.Announcement) (models.Announcement, error) {
	panic("unexpected announcement append")
}

type roomEvents struct {
	t      *testing.T
	repo   *roomRepository
	events []models.Event
}

func (e *roomEvents) Publish(ride, kind string) {
	e.t.Helper()
	if e.repo.locked || e.repo.commitErr != nil {
		e.t.Fatal("message published before successful commit")
	}
	e.events = append(e.events, models.Event{RideID: ride, Type: kind})
}
func roomFixture(t *testing.T) (*Manager, *roomRepository, *roomEvents, models.User) {
	t.Helper()
	repo := &roomRepository{ride: models.Ride{RideSummary: models.RideSummary{ID: uuid.NewString(), Status: "published"}}, access: models.RoomAccess{ParticipationState: "approved", UserStatus: "active"}}
	events := &roomEvents{t: t, repo: repo}
	actor := models.User{ID: uuid.NewString(), Profile: models.Profile{DisplayName: "Rider"}}
	return New(repo, events, func() time.Time { return time.Date(2026, 11, 3, 10, 0, 0, 0, time.UTC) }), repo, events, actor
}
func requireRoomFault(t *testing.T, err error, code string) {
	t.Helper()
	var fault *models.Fault
	if !errors.As(err, &fault) || fault.Code != code {
		t.Fatalf("got %v; want %s", err, code)
	}
}

func TestMessagingUsesCommittedSequencesAndStableRetryIdentity(t *testing.T) {
	manager, repo, events, actor := roomFixture(t)
	input := models.MessageInput{ClientMessageID: uuid.NewString(), Body: " First message "}
	first, err := manager.Send(context.Background(), repo.ride.ID, actor, input)
	if err != nil || !first.Created || first.Message.Sequence != 1 || first.Message.Body != "First message" || first.Message.SenderName != "Rider" || len(events.events) != 1 {
		t.Fatal("new committed message contract changed")
	}
	second, err := manager.Send(context.Background(), repo.ride.ID, actor, input)
	if err != nil || second.Created || second.Message.ID != first.Message.ID || repo.appended != 1 || len(events.events) != 1 {
		t.Fatal("retry duplicated or rebroadcast message")
	}
	input.Body = "Changed text"
	_, err = manager.Send(context.Background(), repo.ride.ID, actor, input)
	requireRoomFault(t, err, "message_id_conflict")
	if len(repo.messages) != 1 {
		t.Fatal("conflicting retry altered history")
	}
}

func TestMessagePreservesAcceptedClientUUIDWireValue(t *testing.T) {
	manager, repo, _, actor := roomFixture(t)
	commandID := strings.ToUpper(uuid.NewString())
	result, err := manager.Send(context.Background(), repo.ride.ID, actor, models.MessageInput{ClientMessageID: commandID, Body: "Preserve the accepted representation"})
	if err != nil || result.Message.ClientMessageID != commandID {
		t.Fatal("validated client UUID was silently rewritten")
	}
}

func TestCancelledRoomIsReadOnlyButAcknowledgesCommittedRetry(t *testing.T) {
	manager, repo, _, actor := roomFixture(t)
	input := models.MessageInput{ClientMessageID: uuid.NewString(), Body: "Committed before cancellation"}
	first, err := manager.Send(context.Background(), repo.ride.ID, actor, input)
	if err != nil {
		t.Fatal(err)
	}
	repo.ride.Status = "cancelled"
	retry, err := manager.Send(context.Background(), repo.ride.ID, actor, input)
	if err != nil || retry.Created || retry.Message.ID != first.Message.ID {
		t.Fatal("cancelled room refused committed retry identity")
	}
	_, err = manager.Send(context.Background(), repo.ride.ID, actor, models.MessageInput{ClientMessageID: uuid.NewString(), Body: "New text"})
	requireRoomFault(t, err, "ride_cancelled")
	page, err := manager.Messages(context.Background(), repo.ride.ID, actor.ID, 7)
	if err != nil || repo.readAfter != 7 || repo.messageLimit != 100 || page.Messages == nil || page.NextSequence != 7 {
		t.Fatal("cancelled room history or page policy changed")
	}
}

func TestAccessIsRecheckedInsideSendBeforeIdempotentLookup(t *testing.T) {
	for name, access := range map[string]models.RoomAccess{"pending": {ParticipationState: "requested", UserStatus: "active"}, "removed": {ParticipationState: "removed", UserStatus: "active"}, "suspended": {ParticipationState: "approved", UserStatus: "suspended"}} {
		t.Run(name, func(t *testing.T) {
			manager, repo, events, actor := roomFixture(t)
			repo.access = access
			_, err := manager.Send(context.Background(), repo.ride.ID, actor, models.MessageInput{ClientMessageID: uuid.NewString(), Body: "Text"})
			requireRoomFault(t, err, "participant_required")
			if repo.appended != 0 || len(events.events) != 0 {
				t.Fatal("unauthorized sender changed room")
			}
			_, err = manager.Messages(context.Background(), repo.ride.ID, actor.ID, 0)
			requireRoomFault(t, err, "participant_required")
		})
	}
}

func TestMessageValidationAndFailedCommit(t *testing.T) {
	manager, repo, events, actor := roomFixture(t)
	for _, input := range []models.MessageInput{{ClientMessageID: "invalid", Body: "Text"}, {ClientMessageID: uuid.NewString(), Body: " "}, {ClientMessageID: uuid.NewString(), Body: strings.Repeat("a", 8001)}} {
		_, err := manager.Send(context.Background(), repo.ride.ID, actor, input)
		requireRoomFault(t, err, "invalid_message")
	}
	_, err := manager.Messages(context.Background(), repo.ride.ID, actor.ID, -1)
	requireRoomFault(t, err, "invalid_cursor")
	repo.commitErr = errors.New("commit failed")
	_, err = manager.Send(context.Background(), repo.ride.ID, actor, models.MessageInput{ClientMessageID: uuid.NewString(), Body: "Uncommitted"})
	if err == nil || len(repo.messages) != 0 || len(events.events) != 0 {
		t.Fatal("failed commit was published or persisted")
	}
}

func TestEventBusIsBoundedRideScopedAndNonblocking(t *testing.T) {
	bus := NewEventBus()
	a := bus.Subscribe("a")
	b := bus.Subscribe("b")
	for range 100 {
		bus.Publish("a", "message.created")
	}
	if len(a.Events) != 8 || len(b.Events) != 0 {
		t.Fatal("event queue is not bounded or ride scoped")
	}
	bus.WakeAll()
	if event := <-b.Events; event.Type != "" || event.RideID != "" {
		t.Fatal("wake should be an empty access-check hint")
	}
	bus.Unsubscribe(a)
	for len(a.Events) > 0 {
		<-a.Events
	}
	bus.Publish("a", "ride.updated")
	if len(a.Events) != 0 {
		t.Fatal("unsubscribed listener received events")
	}
}

func TestEventBusConcurrentLifecycle(t *testing.T) {
	bus := NewEventBus()
	var group sync.WaitGroup
	for range 20 {
		group.Add(1)
		go func() {
			defer group.Done()
			for range 100 {
				subscription := bus.Subscribe("ride")
				bus.Publish("ride", "ride.updated")
				bus.WakeAll()
				bus.Unsubscribe(subscription)
			}
		}()
	}
	done := make(chan struct{})
	go func() { group.Wait(); close(done) }()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("subscriber lifecycle blocked publication")
	}
}

func TestGlobalTrustHintDisplacesAnOlderQueuedHint(t *testing.T) {
	bus := NewEventBus()
	sub := bus.Subscribe("ride")
	defer bus.Unsubscribe(sub)
	for i := 0; i < subscriberQueueSize+4; i++ {
		bus.Publish("ride", "message.created")
	}
	bus.WakeAll()
	bus.PublishAll("trust.changed")
	found := false
	for i := 0; i < subscriberQueueSize; i++ {
		if event := <-sub.Events; event.Type == "trust.changed" {
			found = true
		}
	}
	if !found {
		t.Fatal("full room queue dropped access invalidation")
	}
}
