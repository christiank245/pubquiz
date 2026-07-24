package migrations

import (
	"fmt"

	"github.com/pocketbase/dbx"
	"github.com/pocketbase/pocketbase/daos"
	m "github.com/pocketbase/pocketbase/migrations"
	"github.com/pocketbase/pocketbase/models/schema"
)

func init() {
	m.Register(func(db dbx.Builder) error {
		return setRegistrationsQuizCascadeDelete(db, true)
	}, func(db dbx.Builder) error {
		return setRegistrationsQuizCascadeDelete(db, false)
	})
}

func setRegistrationsQuizCascadeDelete(db dbx.Builder, enabled bool) error {
	dao := daos.New(db)

	registrations, err := dao.FindCollectionByNameOrId("registrations")
	if err != nil {
		return err
	}

	field := registrations.Schema.GetFieldByName("quiz")
	if field == nil {
		return fmt.Errorf("registrations.quiz field not found")
	}
	if err := field.InitOptions(); err != nil {
		return err
	}

	options, ok := field.Options.(*schema.RelationOptions)
	if !ok {
		return fmt.Errorf("registrations.quiz is not a relation field")
	}
	options.CascadeDelete = enabled
	field.Options = options
	registrations.Schema.AddField(field)

	return dao.SaveCollection(registrations)
}
