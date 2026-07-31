package main

import (
	"log"
	"io/fs"

	"github.com/pocketbase/pocketbase"
	"github.com/pocketbase/pocketbase/apis"
	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/plugins/migratecmd"

	routeapi "pubquiz-website2.0/internal/api"

	_ "pubquiz-website2.0/migrations"
)

func main() {
	app := pocketbase.New()
	migratecmd.MustRegister(app, app.RootCmd, migratecmd.Config{})

	app.OnBeforeServe().Add(func(e *core.ServeEvent) error {
		assetFS, err := fs.Sub(webFiles, "web/public")
		if err != nil {
			return err
		}
		e.Router.GET("/assets/*", apis.StaticDirectoryHandler(assetFS, false))

		publicHandlers := routeapi.BuildPublicHandlers(app, renderPage, renderRegisterPanel, renderTemplate)
		adminHandlers := routeapi.BuildAdminHandlers(app, renderPage, renderTemplate)
		routeapi.RegisterPublicRoutes(e.Router, publicHandlers)
		routeapi.RegisterAdminRoutes(e.Router, adminHandlers)

		return nil
	})

	if err := app.Start(); err != nil {
		log.Fatal(err)
	}
}
