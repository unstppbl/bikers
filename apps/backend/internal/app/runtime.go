package app

import (
	"context"
	"errors"
	"log"
	"net"
	"net/http"
	"os"
	"time"

	"bikers.kz/backend/internal/clients/photon"
	"bikers.kz/backend/internal/clients/telegram"
	"bikers.kz/backend/internal/clients/valhalla"
	webpushclient "bikers.kz/backend/internal/clients/webpush"
	"bikers.kz/backend/internal/config"
	"bikers.kz/backend/internal/database"
	"bikers.kz/backend/internal/database/postgres"
	"bikers.kz/backend/internal/managers/accounts"
	"bikers.kz/backend/internal/managers/clubs"
	"bikers.kz/backend/internal/managers/garage"
	"bikers.kz/backend/internal/managers/geo"
	"bikers.kz/backend/internal/managers/media"
	"bikers.kz/backend/internal/managers/notifications"
	"bikers.kz/backend/internal/managers/photos"
	"bikers.kz/backend/internal/managers/places"
	"bikers.kz/backend/internal/managers/reminders"
	"bikers.kz/backend/internal/managers/rides"
	"bikers.kz/backend/internal/managers/rooms"
	"bikers.kz/backend/internal/managers/routes"
	"bikers.kz/backend/internal/managers/system"
	"bikers.kz/backend/internal/managers/trust"
	httpapi "bikers.kz/backend/internal/resources/http"
	"bikers.kz/backend/internal/workers"
	"github.com/jackc/pgx/v5/pgxpool"
	"sync"
)

type Options struct {
	Now         func() time.Time
	PushGateway notifications.Gateway
	Geocoder    geo.Geocoder
	Router      geo.Router
}
type Runtime struct {
	Handler       http.Handler
	System        *system.Manager
	Trust         *trust.Manager
	Notifications *notifications.Manager
	Reminders     *reminders.Manager
	Places        *places.Manager
	Routes        *routes.Manager
	settings      config.Config
	close         func()
}

// New composes explicitly supplied adapters, including the clock used by tests.
func New(cfg config.Config, repository database.Repository, gateway accounts.Gateway, options Options) (*Runtime, error) {
	if err := config.ValidateMaps(cfg.PhotonBaseURL, cfg.ValhallaBaseURL, cfg.MapsArchiveURL); err != nil {
		return nil, err
	}
	if err := config.ValidatePush(cfg.PushPublicKey, cfg.PushPrivateKey, cfg.PushSubject); err != nil {
		return nil, err
	}
	origin, err := config.ValidateBaseURL(cfg.BaseURL)
	if err != nil {
		return nil, err
	}
	proxies, err := config.ParseTrustedProxies(cfg.TrustedProxyCIDRs)
	if err != nil {
		return nil, err
	}
	now := options.Now
	if now == nil {
		now = func() time.Time { return time.Now().UTC() }
	}
	events := rooms.NewEventBus()
	geocoder, router := options.Geocoder, options.Router
	if geocoder == nil && cfg.PhotonBaseURL != "" {
		geocoder = photon.New(cfg.PhotonBaseURL, photon.Options{})
	}
	if router == nil && cfg.ValhallaBaseURL != "" {
		router = valhalla.New(cfg.ValhallaBaseURL, valhalla.Options{})
	}
	mapManager := geo.New(geocoder, router, cfg.MapsArchiveURL, now)
	a := accounts.New(repository, gateway, events, accounts.Options{Now: now, DailyBudget: cfg.OTPDailyBudget})
	r := rides.New(repository, events, now, mapManager)
	rm := rooms.New(repository, events, now)
	sy := system.New(repository, cfg.MediaDir)
	photoProcessing := photos.New(nil)
	g := garage.New(repository, now, photoProcessing)
	c := clubs.New(repository, now, photoProcessing)
	md := media.New(repository, now, photoProcessing)
	tr := trust.New(repository, events, now)
	push := options.PushGateway
	if push == nil && cfg.PushPublicKey != "" {
		push = webpushclient.New(webpushclient.Options{PublicKey: cfg.PushPublicKey, PrivateKey: cfg.PushPrivateKey, Subject: cfg.PushSubject})
	}
	nf := notifications.New(repository, push, cfg.PushPublicKey, now)
	reminderManager := reminders.New(repository, now)
	placeManager := places.New(repository, cfg.MapsArchiveURL, now)
	routeManager := routes.New(repository, mapManager, now)
	transport := httpapi.New(httpapi.Settings{Origin: origin, TrustedProxies: proxies, WebDistDir: cfg.WebDistDir}, httpapi.Managers{Accounts: a, Rides: r, Rooms: rm, System: sy, Events: events, Garage: g, Geo: mapManager, Clubs: c, Media: md, Trust: tr, Notifications: nf, Reminders: reminderManager, Places: placeManager, Routes: routeManager})
	return &Runtime{Handler: transport.Handler(), System: sy, Trust: tr, Notifications: nf, Reminders: reminderManager, Places: placeManager, Routes: routeManager, settings: cfg}, nil
}

// Open owns construction and cleanup of production dependencies.
func Open(ctx context.Context, cfg config.Config) (*Runtime, error) {
	db, err := pgxpool.New(ctx, cfg.DatabaseURL)
	if err != nil {
		return nil, errors.New("invalid database configuration")
	}
	runtime, err := New(cfg, postgres.New(db), telegram.New(cfg.TelegramToken, telegram.Options{}), Options{})
	if err != nil {
		db.Close()
		return nil, err
	}
	runtime.close = db.Close
	return runtime, nil
}
func (r *Runtime) Close() {
	if r.close != nil {
		r.close()
	}
}

func (r *Runtime) Run(ctx context.Context) error {
	ctx, cancelWorkers := context.WithCancel(ctx)
	var workerWG sync.WaitGroup
	workerWG.Add(2)
	go func() { defer workerWG.Done(); workers.RunPush(ctx, r.Notifications) }()
	go func() { defer workerWG.Done(); workers.RunReminders(ctx, r.Reminders) }()
	defer func() { cancelWorkers(); workerWG.Wait() }()
	if err := os.MkdirAll(r.settings.MediaDir, 0750); err != nil {
		return errors.New("persistent media directory unavailable")
	}
	h := http.Server{Addr: ":" + r.settings.Port, Handler: r.Handler, ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 60 * time.Second, WriteTimeout: 60 * time.Second, IdleTimeout: 60 * time.Second, MaxHeaderBytes: 16 << 10, BaseContext: func(net.Listener) context.Context { return ctx }}
	errs := make(chan error, 1)
	go func() {
		log.Printf("HTTP application listening on port %s", r.settings.Port)
		errs <- h.ListenAndServe()
	}()
	select {
	case err := <-errs:
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return err
	case <-ctx.Done():
		shutdown, done := context.WithTimeout(context.Background(), 10*time.Second)
		defer done()
		return h.Shutdown(shutdown)
	}
}
