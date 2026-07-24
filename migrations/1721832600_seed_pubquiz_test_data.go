package migrations

import (
	"database/sql"
	"errors"
	"time"

	"github.com/pocketbase/dbx"
	"github.com/pocketbase/pocketbase/daos"
	m "github.com/pocketbase/pocketbase/migrations"
	"github.com/pocketbase/pocketbase/models"
)

func init() {
	m.Register(func(db dbx.Builder) error {
		dao := daos.New(db)

		copperMug, err := getOrCreateLocation(dao, "The Copper Mug", "https://maps.google.com/?q=Brussels+Grand+Place", 90)
		if err != nil {
			return err
		}
		hopLantern, err := getOrCreateLocation(dao, "The Hop & Lantern", "https://maps.google.com/?q=Antwerp+Central", 70)
		if err != nil {
			return err
		}
		quizCellar, err := getOrCreateLocation(dao, "Quiz Cellar", "https://maps.google.com/?q=Ghent+Korenmarkt", 50)
		if err != nil {
			return err
		}

		upcoming := []struct {
			locationID string
			when       time.Time
		}{
			{locationID: copperMug.Id, when: nextAt(3, 19, 30)},
			{locationID: hopLantern.Id, when: nextAt(7, 20, 0)},
			{locationID: quizCellar.Id, when: nextAt(12, 19, 45)},
		}

		for _, q := range upcoming {
			if err := createQuizDateIfMissing(dao, q.locationID, q.when); err != nil {
				return err
			}
		}

		return nil
	}, func(db dbx.Builder) error {
		dao := daos.New(db)

		for _, name := range []string{"The Copper Mug", "The Hop & Lantern", "Quiz Cellar"} {
			loc, err := dao.FindFirstRecordByData("locations", "name", name)
			if err != nil {
				if errors.Is(err, sql.ErrNoRows) {
					continue
				}
				return err
			}

			quizzes, err := dao.FindRecordsByFilter(
				"quiz_dates",
				"location = {:location}",
				"",
				0,
				0,
				dbx.Params{"location": loc.Id},
			)
			if err != nil {
				return err
			}

			for _, q := range quizzes {
				if err := dao.DeleteRecord(q); err != nil {
					return err
				}
			}

			if err := dao.DeleteRecord(loc); err != nil {
				return err
			}
		}

		return nil
	})
}

func getOrCreateLocation(dao *daos.Dao, name, mapsURL string, capacity int) (*models.Record, error) {
	found, err := dao.FindFirstRecordByData("locations", "name", name)
	if err == nil {
		return found, nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return nil, err
	}

	col, err := dao.FindCollectionByNameOrId("locations")
	if err != nil {
		return nil, err
	}

	rec := models.NewRecord(col)
	rec.Set("name", name)
	rec.Set("maps_url", mapsURL)
	rec.Set("capacity", capacity)

	if err := dao.SaveRecord(rec); err != nil {
		return nil, err
	}

	return rec, nil
}

func createQuizDateIfMissing(dao *daos.Dao, locationID string, when time.Time) error {
	col, err := dao.FindCollectionByNameOrId("quiz_dates")
	if err != nil {
		return err
	}

	scheduledAt := when.UTC().Format("2006-01-02 15:04:05.000Z")
	_, err = dao.FindFirstRecordByFilter(
		"quiz_dates",
		"location = {:location} && scheduled_at = {:scheduledAt}",
		dbx.Params{"location": locationID, "scheduledAt": scheduledAt},
	)
	if err == nil {
		return nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return err
	}

	rec := models.NewRecord(col)
	rec.Set("location", locationID)
	rec.Set("scheduled_at", scheduledAt)
	rec.Set("is_open", true)

	return dao.SaveRecord(rec)
}

func nextAt(daysAhead, hour, minute int) time.Time {
	now := time.Now().UTC()
	target := now.AddDate(0, 0, daysAhead)
	return time.Date(target.Year(), target.Month(), target.Day(), hour, minute, 0, 0, time.UTC)
}
