package main

import (
	"log"

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
		assetFS, err := assetsSubFS()
		if err != nil {
			return err
		}

		e.Router.GET("/assets/*", apis.StaticDirectoryHandler(assetFS, false))

		routeapi.RegisterPublicRoutes(e.Router, routeapi.BuildPublicHandlers(app, renderPage, renderRegisterPanel, renderTemplate))
		routeapi.RegisterAdminRoutes(e.Router, routeapi.BuildAdminHandlers(app, renderPage, renderTemplate))

		return nil
	})

	if err := app.Start(); err != nil {
		log.Fatal(err)
	}
}
