package api

import "github.com/labstack/echo/v5"

type AdminHandlers struct {
	Index                           echo.HandlerFunc
	LoginGet                        echo.HandlerFunc
	LoginPost                       echo.HandlerFunc
	LogoutPost                      echo.HandlerFunc
	CollectionGet                   echo.HandlerFunc
	CollectionCreate                echo.HandlerFunc
	CollectionUpdate                echo.HandlerFunc
	RegistrationMergePost           echo.HandlerFunc
	RegistrationMergeButtonPost     echo.HandlerFunc
	RegistrationMergeModalPost      echo.HandlerFunc
	RegistrationMergeModalClosePost echo.HandlerFunc
	CollectionDelete                echo.HandlerFunc
}

func RegisterAdminRoutes(router *echo.Echo, handlers AdminHandlers) {
	router.GET("/quiz-admin", handlers.Index)
	router.GET("/quiz-admin/login", handlers.LoginGet)
	router.POST("/quiz-admin/login", handlers.LoginPost)
	router.POST("/quiz-admin/logout", handlers.LogoutPost)
	router.GET("/quiz-admin/collections/:name", handlers.CollectionGet)
	router.POST("/quiz-admin/collections/:name/create", handlers.CollectionCreate)
	router.POST("/quiz-admin/collections/:name/update", handlers.CollectionUpdate)
	router.POST("/quiz-admin/collections/registrations/merge", handlers.RegistrationMergePost)
	router.POST("/quiz-admin/collections/registrations/merge/button", handlers.RegistrationMergeButtonPost)
	router.POST("/quiz-admin/collections/registrations/merge/modal", handlers.RegistrationMergeModalPost)
	router.POST("/quiz-admin/collections/registrations/merge/modal/close", handlers.RegistrationMergeModalClosePost)
	router.POST("/quiz-admin/collections/:name/delete", handlers.CollectionDelete)
}
