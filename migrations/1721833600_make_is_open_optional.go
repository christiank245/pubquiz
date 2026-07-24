package migrations

import (
	"fmt"

	"github.com/pocketbase/dbx"
	"github.com/pocketbase/pocketbase/daos"
	m "github.com/pocketbase/pocketbase/migrations"
)

func init() {
	m.Register(func(db dbx.Builder) error {
		dao := daos.New(db)

		quizDates, err := dao.FindCollectionByNameOrId("quiz_dates")
		if err != nil {
			return err
		}

		field := quizDates.Schema.GetFieldByName("is_open")
		if field == nil {
			return fmt.Errorf("quiz_dates.is_open field not found")
		}

		field.Required = false
		quizDates.Schema.AddField(field)

		return dao.SaveCollection(quizDates)
	}, func(db dbx.Builder) error {
		dao := daos.New(db)

		quizDates, err := dao.FindCollectionByNameOrId("quiz_dates")
		if err != nil {
			return err
		}

		field := quizDates.Schema.GetFieldByName("is_open")
		if field == nil {
			return fmt.Errorf("quiz_dates.is_open field not found")
		}

		field.Required = true
		quizDates.Schema.AddField(field)

		return dao.SaveCollection(quizDates)
	})
}
