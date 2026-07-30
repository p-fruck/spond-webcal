package syncworker

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"

	"code.p-fruck.eu/spond-webcal/internal/api"
	"code.p-fruck.eu/spond-webcal/internal/db"
	"code.p-fruck.eu/spond-webcal/internal/spond"
)

type UserTokenReader interface {
	ActiveUsersWithToken(ctx context.Context) ([]db.UserTokenRecord, error)
}

type EventCacheWriter interface {
	UpsertUserEvents(ctx context.Context, userID uint, events []api.Event, syncedAt time.Time) error
}

type EventsWorker struct {
	userTokens   UserTokenReader
	cache        EventCacheWriter
	spondBaseURL string
	interval     time.Duration
	timeout      time.Duration
	logger       *log.Logger
}

func NewEventsWorker(
	userTokens UserTokenReader,
	cache EventCacheWriter,
	spondBaseURL string,
	interval time.Duration,
	timeout time.Duration,
	logger *log.Logger,
) (*EventsWorker, error) {
	if userTokens == nil {
		return nil, fmt.Errorf("user token reader is required")
	}

	if cache == nil {
		return nil, fmt.Errorf("event cache writer is required")
	}

	if interval <= 0 {
		return nil, fmt.Errorf("sync interval must be positive")
	}

	if timeout <= 0 {
		return nil, fmt.Errorf("sync timeout must be positive")
	}

	if logger == nil {
		logger = log.Default()
	}

	return &EventsWorker{
		userTokens:   userTokens,
		cache:        cache,
		spondBaseURL: spondBaseURL,
		interval:     interval,
		timeout:      timeout,
		logger:       logger,
	}, nil
}

func (w *EventsWorker) Start(ctx context.Context) {
	go func() {
		w.runLoop(ctx)
	}()
}

func (w *EventsWorker) runLoop(ctx context.Context) {
	w.runOnceWithLog(ctx)

	ticker := time.NewTicker(w.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			w.runOnceWithLog(ctx)
		}
	}
}

func (w *EventsWorker) runOnceWithLog(ctx context.Context) {
	if err := w.RunOnce(ctx); err != nil {
		w.logger.Printf("events sync worker run failed: %v", err)
	}
}

func (w *EventsWorker) RunOnce(ctx context.Context) error {
	users, err := w.userTokens.ActiveUsersWithToken(ctx)
	if err != nil {
		return fmt.Errorf("load active users with token: %w", err)
	}

	for _, user := range users {
		if ctx.Err() != nil {
			return ctx.Err()
		}

		if err := w.syncUser(ctx, user); err != nil {
			w.logger.Printf("events sync failed for user %d (%s): %v", user.UserID, user.ProfileID, err)
		}
	}

	return nil
}

func (w *EventsWorker) syncUser(parent context.Context, user db.UserTokenRecord) error {
	token, err := chooseTokenForSync(user, time.Now().UTC())
	if err != nil {
		return err
	}

	client, err := spond.New(w.spondBaseURL)
	if err != nil {
		return fmt.Errorf("create spond client: %w", err)
	}

	client.SetToken(token)

	ctx, cancel := context.WithTimeout(parent, w.timeout)
	defer cancel()

	events, err := client.FetchEvents(ctx, 200)
	if err != nil {
		return fmt.Errorf("fetch events: %w", err)
	}

	if err := w.cache.UpsertUserEvents(ctx, user.UserID, events, time.Now().UTC()); err != nil {
		return fmt.Errorf("cache events: %w", err)
	}

	return nil
}

func chooseTokenForSync(user db.UserTokenRecord, now time.Time) (string, error) {
	const expirySkew = 30 * time.Second

	accessToken := strings.TrimSpace(user.SpondToken)
	refreshToken := strings.TrimSpace(user.RefreshToken)

	accessValid := accessToken != "" && (user.TokenExpires == nil || user.TokenExpires.After(now.Add(expirySkew)))
	if accessValid {
		return accessToken, nil
	}

	refreshValid := refreshToken != "" && (user.RefreshTokenExpires == nil || user.RefreshTokenExpires.After(now.Add(expirySkew)))
	if refreshValid {
		return refreshToken, nil
	}

	if accessToken == "" && refreshToken == "" {
		return "", fmt.Errorf("no usable spond token is stored")
	}

	if user.TokenExpires != nil && user.TokenExpires.Before(now.Add(expirySkew)) {
		if user.RefreshTokenExpires != nil && user.RefreshTokenExpires.Before(now.Add(expirySkew)) {
			return "", fmt.Errorf("access and refresh tokens are expired")
		}

		if refreshToken == "" {
			return "", fmt.Errorf("access token expired and no refresh token stored")
		}
	}

	if accessToken != "" {
		return accessToken, nil
	}

	return "", fmt.Errorf("no usable spond token is available")
}
