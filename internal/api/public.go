package api

import "github.com/labstack/echo/v5"

type PublicHandlers struct {
	Home           echo.HandlerFunc
	About          echo.HandlerFunc
	Contact        echo.HandlerFunc
	Privacy        echo.HandlerFunc
	Quiz           echo.HandlerFunc
	Register       echo.HandlerFunc
	UnregisterGet  echo.HandlerFunc
	UnregisterPost echo.HandlerFunc
}

func RegisterPublicRoutes(router *echo.Echo, handlers PublicHandlers) {
	router.GET("/", handlers.Home)
	router.GET("/about", handlers.About)
	router.GET("/contact", handlers.Contact)
	router.GET("/privacy", handlers.Privacy)
	router.GET("/quiz", handlers.Quiz)
	router.POST("/register", handlers.Register)
	router.GET("/unregister", handlers.UnregisterGet)
	router.POST("/unregister", handlers.UnregisterPost)
}
