package migrations

import (
	"database/sql"
	"errors"

	"github.com/pocketbase/dbx"
	"github.com/pocketbase/pocketbase/daos"
	m "github.com/pocketbase/pocketbase/migrations"
	"github.com/pocketbase/pocketbase/models"
)

func init() {
	m.Register(func(db dbx.Builder) error {
		dao := daos.New(db)

		_, err := dao.FindCollectionByNameOrId("quiz_admins")
		if err == nil {
			return nil
		}
		if !errors.Is(err, sql.ErrNoRows) {
			return err
		}

		collection := &models.Collection{
			Name: "quiz_admins",
			Type: models.CollectionTypeAuth,
		}
		collection.MarkAsNew()
		if err := collection.SetOptions(models.CollectionAuthOptions{
			AllowEmailAuth:    true,
			RequireEmail:      true,
			AllowUsernameAuth: true,
			MinPasswordLength: 8,
		}); err != nil {
			return err
		}

		return dao.SaveCollection(collection)
	}, func(db dbx.Builder) error {
		dao := daos.New(db)

		collection, err := dao.FindCollectionByNameOrId("quiz_admins")
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return nil
			}
			return err
		}

		return dao.DeleteCollection(collection)
	})
}
