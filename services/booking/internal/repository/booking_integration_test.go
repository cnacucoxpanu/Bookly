package repository_test

import (
	"context"
	"errors"
	"fmt"
	"os"
	"sync"
	"testing"
	"time"

	"bookly/services/booking/internal/database"
	"bookly/services/booking/internal/model"
	"bookly/services/booking/internal/repository"
	"bookly/services/booking/migrations"

	"github.com/jackc/pgx/v5/pgxpool"
)

// These tests need a real Postgres, because the thing under test is how the
// database behaves under concurrent transactions. Run them with:
//
//	TEST_DATABASE_URL=postgres://user:pass@localhost:5432/booking go test ./internal/repository/
func setup(t *testing.T) (*pgxpool.Pool, *model.Space) {
	t.Helper()

	url := os.Getenv("TEST_DATABASE_URL")
	if url == "" {
		t.Skip("TEST_DATABASE_URL is not set")
	}

	ctx := context.Background()

	if err := database.Migrate(url, migrations.FS); err != nil {
		t.Fatal(err)
	}

	pool, err := database.Connect(ctx, url)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)

	// A fresh space per test keeps tests independent of each other and of
	// whatever is already in the database.
	space := &model.Space{Name: fmt.Sprintf("test-space-%d", time.Now().UnixNano()), Capacity: 1}
	if err := repository.NewSpaceRepository(pool).Create(ctx, space); err != nil {
		t.Fatal(err)
	}

	return pool, space
}

func TestConcurrentBookingsOfSameSlot(t *testing.T) {
	pool, space := setup(t)
	repo := repository.NewBookingRepository(pool)

	const workers = 20
	start := time.Now().Add(48 * time.Hour).Truncate(time.Minute)

	var wg sync.WaitGroup
	results := make(chan error, workers)
	ready := make(chan struct{})

	for i := 1; i <= workers; i++ {
		wg.Add(1)
		go func(userID int64) {
			defer wg.Done()
			<-ready

			results <- repo.Create(context.Background(), &model.Booking{
				SpaceID:   space.ID,
				UserID:    userID,
				StartTime: start,
				EndTime:   start.Add(time.Hour),
			})
		}(int64(i))
	}

	close(ready)
	wg.Wait()
	close(results)

	var booked, taken int
	for err := range results {
		switch {
		case err == nil:
			booked++
		case errors.Is(err, repository.ErrSlotTaken):
			taken++
		default:
			t.Errorf("unexpected error: %v", err)
		}
	}

	if booked != 1 || taken != workers-1 {
		t.Fatalf("booked=%d taken=%d, want exactly 1 and %d", booked, taken, workers-1)
	}
}

func TestConcurrentOverlappingBookingsNeverOverlap(t *testing.T) {
	pool, space := setup(t)
	repo := repository.NewBookingRepository(pool)

	const workers = 30
	base := time.Now().Add(72 * time.Hour).Truncate(time.Minute)

	var wg sync.WaitGroup
	ready := make(chan struct{})

	// Every booking lasts 2 hours and starts 20 minutes after the previous
	// one, so each overlaps with several others.
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			<-ready

			start := base.Add(time.Duration(i) * 20 * time.Minute)
			err := repo.Create(context.Background(), &model.Booking{
				SpaceID:   space.ID,
				UserID:    int64(i + 1),
				StartTime: start,
				EndTime:   start.Add(2 * time.Hour),
			})
			if err != nil && !errors.Is(err, repository.ErrSlotTaken) {
				t.Errorf("unexpected error: %v", err)
			}
		}(i)
	}

	close(ready)
	wg.Wait()

	var overlapping int
	err := pool.QueryRow(context.Background(), `
	SELECT COUNT(*)
	FROM bookings a
	JOIN bookings b
	  ON a.space_id = b.space_id
	 AND a.id < b.id
	 AND a.status = 'active'
	 AND b.status = 'active'
	 AND a.start_time < b.end_time
	 AND a.end_time > b.start_time
	WHERE a.space_id = $1
	`, space.ID).Scan(&overlapping)
	if err != nil {
		t.Fatal(err)
	}
	if overlapping != 0 {
		t.Fatalf("found %d pairs of overlapping active bookings", overlapping)
	}
}

func TestCancelledBookingFreesSlot(t *testing.T) {
	pool, space := setup(t)
	repo := repository.NewBookingRepository(pool)
	ctx := context.Background()

	start := time.Now().Add(96 * time.Hour).Truncate(time.Minute)
	first := &model.Booking{SpaceID: space.ID, UserID: 1, StartTime: start, EndTime: start.Add(time.Hour)}
	if err := repo.Create(ctx, first); err != nil {
		t.Fatal(err)
	}

	second := &model.Booking{SpaceID: space.ID, UserID: 2, StartTime: start, EndTime: start.Add(time.Hour)}
	if err := repo.Create(ctx, second); !errors.Is(err, repository.ErrSlotTaken) {
		t.Fatalf("got %v, want ErrSlotTaken", err)
	}

	if err := repo.Cancel(ctx, first.ID); err != nil {
		t.Fatal(err)
	}
	if err := repo.Cancel(ctx, first.ID); !errors.Is(err, repository.ErrBookingNotActive) {
		t.Fatalf("second cancel: got %v, want ErrBookingNotActive", err)
	}

	if err := repo.Create(ctx, second); err != nil {
		t.Fatalf("slot must be free after cancel: %v", err)
	}
}

// The exclusion constraint is the safety net: even a plain INSERT that skips
// the check in the repository cannot create an overlapping booking.
func TestDatabaseRejectsOverlapOnItsOwn(t *testing.T) {
	pool, space := setup(t)
	ctx := context.Background()

	start := time.Now().Add(120 * time.Hour).Truncate(time.Minute)
	insert := `
	INSERT INTO bookings (space_id, user_id, start_time, end_time)
	VALUES ($1, 1, $2, $3)
	`

	if _, err := pool.Exec(ctx, insert, space.ID, start, start.Add(time.Hour)); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, insert, space.ID, start.Add(30*time.Minute), start.Add(2*time.Hour)); err == nil {
		t.Fatal("overlapping insert must be rejected by the exclusion constraint")
	}
}
