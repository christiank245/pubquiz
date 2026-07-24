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
		dao := daos.New(db)

		registrations, err := dao.FindCollectionByNameOrId("registrations")
		if err != nil {
			return err
		}

		registrations.Schema.AddField(&schema.SchemaField{
			Id:       "registrations_willing_to_merge",
			Name:     "willing_to_merge",
			Type:     schema.FieldTypeBool,
			Required: false,
			Options:  &schema.BoolOptions{},
		})

		return dao.SaveCollection(registrations)
	}, func(db dbx.Builder) error {
		dao := daos.New(db)

		registrations, err := dao.FindCollectionByNameOrId("registrations")
		if err != nil {
			return err
		}

		field := registrations.Schema.GetFieldByName("willing_to_merge")
		if field == nil {
			return fmt.Errorf("registrations.willing_to_merge field not found")
		}

		registrations.Schema.RemoveField(field.Id)
		return dao.SaveCollection(registrations)
	})
}
