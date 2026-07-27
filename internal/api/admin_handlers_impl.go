package api

import (
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"github.com/labstack/echo/v5"
	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/models"
	"github.com/pocketbase/pocketbase/tokens"
)

type adminHandlers struct {
	app core.App
}

func BuildAdminHandlers(app core.App) AdminHandlers {
	h := adminHandlers{app: app}
	return AdminHandlers{
		Index:                           h.index,
		LoginGet:                        h.loginGet,
		LoginPost:                       h.loginPost,
		LogoutPost:                      h.logoutPost,
		CollectionGet:                   h.collectionGet,
		CollectionCreate:                h.collectionCreate,
		CollectionUpdate:                h.collectionUpdate,
		RegistrationMergePost:           h.registrationMergePost,
		RegistrationMergeButtonPost:     h.registrationMergeButtonPost,
		RegistrationMergeModalPost:      h.registrationMergeModalPost,
		RegistrationMergeModalClosePost: h.registrationMergeModalClosePost,
		CollectionDelete:                h.collectionDelete,
	}
}

func (h adminHandlers) index(c echo.Context) error {
	if _, err := requireQuizAdmin(c, h.app); err != nil {
		return err
	}

	collections, err := managedCollections(h.app)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to load collections")
	}
	if len(collections) == 0 {
		return renderPageFn(c, PageData{
			Title:               "Quiz Admin",
			PageTemplate:        "admin_collection",
			IsQuizAdmin:         true,
			ShowQuizAdminLogout: true,
			Admin: AdminPageData{
				Error: "No non-system collections are available.",
			},
		})
	}

	return c.Redirect(http.StatusSeeOther, "/quiz-admin/collections/"+url.PathEscape(collections[0].Name))
}

func (h adminHandlers) loginGet(c echo.Context) error {
	if _, err := currentQuizAdmin(c, h.app); err == nil {
		return c.Redirect(http.StatusSeeOther, "/quiz-admin")
	}
	return renderPageFn(c, PageData{
		Title:        "Quiz Admin Login",
		PageTemplate: "admin_login",
		IsQuizAdmin:  true,
		Admin:        AdminPageData{},
	})
}

func (h adminHandlers) loginPost(c echo.Context) error {
	email := strings.TrimSpace(c.FormValue("email"))
	password := strings.TrimSpace(c.FormValue("password"))

	record, err := h.app.Dao().FindAuthRecordByEmail(quizAdminAuthCollection, email)
	if err != nil || !record.ValidatePassword(password) {
		return renderPageFn(c, PageData{
			Title:        "Quiz Admin Login",
			PageTemplate: "admin_login",
			IsQuizAdmin:  true,
			Admin: AdminPageData{
				Error: "Invalid email or password.",
			},
		})
	}

	token, err := tokens.NewRecordAuthToken(h.app, record)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to create auth token")
	}

	setQuizAdminCookie(c, token)
	return c.Redirect(http.StatusSeeOther, "/quiz-admin")
}

func (h adminHandlers) logoutPost(c echo.Context) error {
	clearQuizAdminCookie(c)
	return c.Redirect(http.StatusSeeOther, "/quiz-admin/login")
}

func (h adminHandlers) collectionGet(c echo.Context) error {
	if _, err := requireQuizAdmin(c, h.app); err != nil {
		return err
	}

	name := strings.TrimSpace(c.PathParam("name"))
	page, err := buildAdminPageData(h.app, name, c.QueryParam("success"), c.QueryParam("error"), c.QueryParam("new") == "1", c.QueryParam("edit"), AdminFormState{})
	if err != nil {
		return err
	}

	return renderPageFn(c, PageData{
		Title:               "Quiz Admin",
		PageTemplate:        "admin_collection",
		IsQuizAdmin:         true,
		ShowQuizAdminLogout: true,
		Admin:               page,
	})
}

func (h adminHandlers) collectionCreate(c echo.Context) error {
	if _, err := requireQuizAdmin(c, h.app); err != nil {
		return err
	}

	collection, err := managedCollectionByName(h.app, strings.TrimSpace(c.PathParam("name")))
	if err != nil {
		return err
	}
	if collection.IsView() {
		return redirectWithMessage(c, collection.Name, "", "View collections are read-only.")
	}

	tableFields, formFields := adminFields(collection)
	form := parseAdminForm(c, formFields)

	record := models.NewRecord(collection)
	if err := applyFormToRecord(record, formFields, form); err != nil {
		return renderAdminPageWithFormError(c, h.app, collection.Name, tableFields, formFields, true, false, "", form, err.Error())
	}
	if err := h.app.Dao().SaveRecord(record); err != nil {
		return renderAdminPageWithFormError(c, h.app, collection.Name, tableFields, formFields, true, false, "", form, err.Error())
	}

	return redirectWithMessage(c, collection.Name, "Entry created.", "")
}

func (h adminHandlers) collectionUpdate(c echo.Context) error {
	if _, err := requireQuizAdmin(c, h.app); err != nil {
		return err
	}

	collection, err := managedCollectionByName(h.app, strings.TrimSpace(c.PathParam("name")))
	if err != nil {
		return err
	}
	if collection.IsView() {
		return redirectWithMessage(c, collection.Name, "", "View collections are read-only.")
	}

	recordID := strings.TrimSpace(c.FormValue("record_id"))
	if recordID == "" {
		return echo.NewHTTPError(http.StatusBadRequest, "record id is required")
	}

	record, err := h.app.Dao().FindRecordById(collection.Name, recordID)
	if err != nil {
		return echo.NewHTTPError(http.StatusNotFound, "record not found")
	}

	tableFields, formFields := adminFields(collection)
	form := parseAdminForm(c, formFields)

	if err := applyFormToRecord(record, formFields, form); err != nil {
		return renderAdminPageWithFormError(c, h.app, collection.Name, tableFields, formFields, false, true, recordID, form, err.Error())
	}
	if err := h.app.Dao().SaveRecord(record); err != nil {
		return renderAdminPageWithFormError(c, h.app, collection.Name, tableFields, formFields, false, true, recordID, form, err.Error())
	}

	return redirectWithMessage(c, collection.Name, "Entry updated.", "")
}

func (h adminHandlers) registrationMergePost(c echo.Context) error {
	if _, err := requireQuizAdmin(c, h.app); err != nil {
		return err
	}

	registrationIDs, err := ParseRegistrationIDsInput(c.FormValue("registration_ids"))
	if err != nil {
		return redirectWithMessage(c, "registrations", "", err.Error())
	}

	col, err := h.app.Dao().FindCollectionByNameOrId("registrations")
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "registrations collection is missing")
	}

	mergedRecord, err := MergeRegistrations(
		h.app.Dao(),
		col,
		registrationIDs,
		strings.TrimSpace(c.FormValue("team_name")),
		strings.TrimSpace(c.FormValue("email")),
	)
	if err != nil {
		var reqErr MergeRequestError
		if errors.As(err, &reqErr) {
			return redirectWithMessage(c, "registrations", "", reqErr.Error())
		}
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to merge registrations")
	}

	return redirectWithMessage(
		c,
		"registrations",
		fmt.Sprintf("Entries merged successfully into \"%s\".", mergedRecord.GetString("team_name")),
		"",
	)
}

func (h adminHandlers) registrationMergeButtonPost(c echo.Context) error {
	if _, err := requireQuizAdmin(c, h.app); err != nil {
		return err
	}
	if err := c.Request().ParseForm(); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid form data")
	}

	selection, err := buildRegistrationMergeSelection(h.app, c.Request().Form["delete_ids"])
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to evaluate merge selection")
	}

	return renderAdminTemplate(c, "admin_merge_entries_button", AdminMergeButtonData{
		Enabled: selection.Error == "" && len(selection.Entries) >= 2,
	})
}

func (h adminHandlers) registrationMergeModalPost(c echo.Context) error {
	if _, err := requireQuizAdmin(c, h.app); err != nil {
		return err
	}
	if err := c.Request().ParseForm(); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid form data")
	}

	selection, err := buildRegistrationMergeSelection(h.app, c.Request().Form["delete_ids"])
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to build merge preview")
	}

	return renderAdminTemplate(c, "admin_merge_entries_modal", selection)
}

func (h adminHandlers) registrationMergeModalClosePost(c echo.Context) error {
	if _, err := requireQuizAdmin(c, h.app); err != nil {
		return err
	}
	return c.HTML(http.StatusOK, "")
}

func (h adminHandlers) collectionDelete(c echo.Context) error {
	if _, err := requireQuizAdmin(c, h.app); err != nil {
		return err
	}

	collection, err := managedCollectionByName(h.app, strings.TrimSpace(c.PathParam("name")))
	if err != nil {
		return err
	}
	if collection.IsView() {
		return redirectWithMessage(c, collection.Name, "", "View collections are read-only.")
	}

	if err := c.Request().ParseForm(); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid form data")
	}
	deleteIDs := c.Request().Form["delete_ids"]
	if len(deleteIDs) == 0 {
		return redirectWithMessage(c, collection.Name, "", "Select at least one entry to delete.")
	}

	for _, recordID := range deleteIDs {
		record, err := h.app.Dao().FindRecordById(collection.Name, recordID)
		if err != nil {
			return echo.NewHTTPError(http.StatusNotFound, "record not found")
		}
		if collection.Name == "quiz_dates" {
			if err := deleteRegistrationsForQuizDate(h.app, recordID); err != nil {
				return echo.NewHTTPError(http.StatusInternalServerError, "failed to delete linked registrations")
			}
		}
		if err := h.app.Dao().DeleteRecord(record); err != nil {
			return echo.NewHTTPError(http.StatusInternalServerError, "failed to delete record")
		}
	}

	return redirectWithMessage(c, collection.Name, fmt.Sprintf("Deleted %d entrie(s).", len(deleteIDs)), "")
}
