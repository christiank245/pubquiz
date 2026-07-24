package migrations

import (
	"database/sql"
	"errors"

	"github.com/pocketbase/dbx"
	"github.com/pocketbase/pocketbase/daos"
	m "github.com/pocketbase/pocketbase/migrations"
	"github.com/pocketbase/pocketbase/models"
	"github.com/pocketbase/pocketbase/models/schema"
	"github.com/pocketbase/pocketbase/tools/types"
)

func init() {
	m.Register(func(db dbx.Builder) error {
		dao := daos.New(db)

		locations := newBaseCollection("locations")
		locations.Schema = schema.NewSchema(
			&schema.SchemaField{
				Id:       "locations_name",
				Name:     "name",
				Type:     schema.FieldTypeText,
				Required: true,
				Options:  &schema.TextOptions{},
			},
			&schema.SchemaField{
				Id:       "locations_maps_url",
				Name:     "maps_url",
				Type:     schema.FieldTypeUrl,
				Required: true,
				Options:  &schema.UrlOptions{},
			},
			&schema.SchemaField{
				Id:       "locations_capacity",
				Name:     "capacity",
				Type:     schema.FieldTypeNumber,
				Required: true,
				Options: &schema.NumberOptions{
					NoDecimal: true,
					Min:       types.Pointer(1.0),
				},
			},
		)
		if err := dao.SaveCollection(locations); err != nil {
			return err
		}

		quizDates := newBaseCollection("quiz_dates")
		quizDates.Schema = schema.NewSchema(
			&schema.SchemaField{
				Id:       "quiz_dates_scheduled_at",
				Name:     "scheduled_at",
				Type:     schema.FieldTypeDate,
				Required: true,
				Options:  &schema.DateOptions{},
			},
			&schema.SchemaField{
				Id:       "quiz_dates_location",
				Name:     "location",
				Type:     schema.FieldTypeRelation,
				Required: true,
				Options: &schema.RelationOptions{
					CollectionId: locations.Id,
					MinSelect:    types.Pointer(1),
					MaxSelect:    types.Pointer(1),
				},
			},
			&schema.SchemaField{
				Id:       "quiz_dates_is_open",
				Name:     "is_open",
				Type:     schema.FieldTypeBool,
				Required: false,
				Options:  &schema.BoolOptions{},
			},
		)
		if err := dao.SaveCollection(quizDates); err != nil {
			return err
		}

		registrations := newBaseCollection("registrations")
		registrations.Schema = schema.NewSchema(
			&schema.SchemaField{
				Id:       "registrations_quiz",
				Name:     "quiz",
				Type:     schema.FieldTypeRelation,
				Required: true,
				Options: &schema.RelationOptions{
					CollectionId:  quizDates.Id,
					CascadeDelete: true,
					MinSelect:     types.Pointer(1),
					MaxSelect:     types.Pointer(1),
				},
			},
			&schema.SchemaField{
				Id:       "registrations_email",
				Name:     "email",
				Type:     schema.FieldTypeEmail,
				Required: true,
				Options:  &schema.EmailOptions{},
			},
			&schema.SchemaField{
				Id:       "registrations_team_name",
				Name:     "team_name",
				Type:     schema.FieldTypeText,
				Required: true,
				Options:  &schema.TextOptions{},
			},
			&schema.SchemaField{
				Id:       "registrations_team_size",
				Name:     "team_size",
				Type:     schema.FieldTypeNumber,
				Required: true,
				Options: &schema.NumberOptions{
					NoDecimal: true,
					Min:       types.Pointer(1.0),
				},
			},
		)
		if err := dao.SaveCollection(registrations); err != nil {
			return err
		}

		return nil
	}, func(db dbx.Builder) error {
		dao := daos.New(db)

		for _, name := range []string{"registrations", "quiz_dates", "locations"} {
			collection, err := dao.FindCollectionByNameOrId(name)
			if err != nil {
				if errors.Is(err, sql.ErrNoRows) {
					continue
				}
				return err
			}

			if err := dao.DeleteCollection(collection); err != nil {
				return err
			}
		}

		return nil
	})
}

func newBaseCollection(name string) *models.Collection {
	collection := &models.Collection{
		Name: name,
		Type: models.CollectionTypeBase,
	}
	collection.MarkAsNew()
	return collection
}
