package main

import (
	"bytes"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/labstack/echo/v5"
	"github.com/pocketbase/dbx"
	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/models"
	"github.com/pocketbase/pocketbase/models/schema"
	"github.com/pocketbase/pocketbase/tokens"
)

const (
	quizAdminAuthCollection = "quiz_admins"
	quizAdminAuthCookieName = "quiz_admin_token"
)

type AdminPageData struct {
	Collections           []AdminCollectionNav
	ActiveCollection      string
	ActiveType            string
	TableFields           []AdminField
	FormFields            []AdminField
	Rows                  []AdminRow
	RegistrationMergeRows map[string]AdminRegistrationMergeRow
	RelationSelectOptions map[string][]AdminSelectOption
	Error                 string
	Success               string
	ShowCreateModal       bool
	ShowEditModal         bool
	EditRecordID          string
	Form                  AdminFormState
}

type AdminCollectionNav struct {
	Name string
	Type string
}

type AdminField struct {
	Name     string
	Type     string
	Editable bool
	Required bool
	Multiple bool
}

type AdminRow struct {
	ID     string
	Values []string
}

type AdminFormState struct {
	Text  map[string]string
	Bools map[string]bool
}

type AdminSelectOption struct {
	Value string
	Label string
}

type AdminRegistrationMergeRow struct {
	QuizID         string
	QuizLabel      string
	TeamName       string
	TeamSize       int
	Email          string
	WillingToMerge bool
}

type AdminMergeButtonData struct {
	Enabled bool
}

type AdminMergeModalEntry struct {
	ID       string
	TeamName string
	TeamSize int
	Email    string
}

type AdminMergeModalData struct {
	Show            bool
	Summary         string
	RegistrationIDs string
	Entries         []AdminMergeModalEntry
	Error           string
}

func registerQuizAdminRoutes(router *echo.Echo, app core.App) {
	router.GET("/quiz-admin", func(c echo.Context) error {
		if _, err := requireQuizAdmin(c, app); err != nil {
			return err
		}

		collections, err := managedCollections(app)
		if err != nil {
			return echo.NewHTTPError(http.StatusInternalServerError, "failed to load collections")
		}
		if len(collections) == 0 {
			return renderPage(c, PageData{
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
	})

	router.GET("/quiz-admin/login", func(c echo.Context) error {
		if _, err := currentQuizAdmin(c, app); err == nil {
			return c.Redirect(http.StatusSeeOther, "/quiz-admin")
		}
		return renderPage(c, PageData{
			Title:        "Quiz Admin Login",
			PageTemplate: "admin_login",
			IsQuizAdmin:  true,
		})
	})

	router.POST("/quiz-admin/login", func(c echo.Context) error {
		email := strings.TrimSpace(c.FormValue("email"))
		password := strings.TrimSpace(c.FormValue("password"))

		record, err := app.Dao().FindAuthRecordByEmail(quizAdminAuthCollection, email)
		if err != nil || !record.ValidatePassword(password) {
			return renderPage(c, PageData{
				Title:        "Quiz Admin Login",
				PageTemplate: "admin_login",
				IsQuizAdmin:  true,
				Admin: AdminPageData{
					Error: "Invalid email or password.",
				},
			})
		}

		token, err := tokens.NewRecordAuthToken(app, record)
		if err != nil {
			return echo.NewHTTPError(http.StatusInternalServerError, "failed to create auth token")
		}

		setQuizAdminCookie(c, token)
		return c.Redirect(http.StatusSeeOther, "/quiz-admin")
	})

	router.POST("/quiz-admin/logout", func(c echo.Context) error {
		clearQuizAdminCookie(c)
		return c.Redirect(http.StatusSeeOther, "/quiz-admin/login")
	})

	router.GET("/quiz-admin/collections/:name", func(c echo.Context) error {
		if _, err := requireQuizAdmin(c, app); err != nil {
			return err
		}

		name := strings.TrimSpace(c.PathParam("name"))
		page, err := buildAdminPageData(app, name, c.QueryParam("success"), c.QueryParam("error"), c.QueryParam("new") == "1", c.QueryParam("edit"), AdminFormState{})
		if err != nil {
			return err
		}

		return renderPage(c, PageData{
			Title:               "Quiz Admin",
			PageTemplate:        "admin_collection",
			IsQuizAdmin:         true,
			ShowQuizAdminLogout: true,
			Admin:               page,
		})
	})

	router.POST("/quiz-admin/collections/:name/create", func(c echo.Context) error {
		if _, err := requireQuizAdmin(c, app); err != nil {
			return err
		}

		collection, err := managedCollectionByName(app, strings.TrimSpace(c.PathParam("name")))
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
			return renderAdminPageWithFormError(c, app, collection.Name, tableFields, formFields, true, false, "", form, err.Error())
		}
		if err := app.Dao().SaveRecord(record); err != nil {
			return renderAdminPageWithFormError(c, app, collection.Name, tableFields, formFields, true, false, "", form, err.Error())
		}

		return redirectWithMessage(c, collection.Name, "Entry created.", "")
	})

	router.POST("/quiz-admin/collections/:name/update", func(c echo.Context) error {
		if _, err := requireQuizAdmin(c, app); err != nil {
			return err
		}

		collection, err := managedCollectionByName(app, strings.TrimSpace(c.PathParam("name")))
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

		record, err := app.Dao().FindRecordById(collection.Name, recordID)
		if err != nil {
			return echo.NewHTTPError(http.StatusNotFound, "record not found")
		}

		tableFields, formFields := adminFields(collection)
		form := parseAdminForm(c, formFields)

		if err := applyFormToRecord(record, formFields, form); err != nil {
			return renderAdminPageWithFormError(c, app, collection.Name, tableFields, formFields, false, true, recordID, form, err.Error())
		}
		if err := app.Dao().SaveRecord(record); err != nil {
			return renderAdminPageWithFormError(c, app, collection.Name, tableFields, formFields, false, true, recordID, form, err.Error())
		}

		return redirectWithMessage(c, collection.Name, "Entry updated.", "")
	})

	router.POST("/quiz-admin/collections/registrations/merge", func(c echo.Context) error {
		if _, err := requireQuizAdmin(c, app); err != nil {
			return err
		}

		registrationIDs, err := parseRegistrationIDsInput(c.FormValue("registration_ids"))
		if err != nil {
			return redirectWithMessage(c, "registrations", "", err.Error())
		}

		col, err := app.Dao().FindCollectionByNameOrId("registrations")
		if err != nil {
			return echo.NewHTTPError(http.StatusInternalServerError, "registrations collection is missing")
		}

		mergedRecord, err := mergeRegistrations(
			app.Dao(),
			col,
			registrationIDs,
			strings.TrimSpace(c.FormValue("team_name")),
			strings.TrimSpace(c.FormValue("email")),
		)
		if err != nil {
			var reqErr mergeRequestError
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
	})

	router.POST("/quiz-admin/collections/registrations/merge/button", func(c echo.Context) error {
		if _, err := requireQuizAdmin(c, app); err != nil {
			return err
		}
		if err := c.Request().ParseForm(); err != nil {
			return echo.NewHTTPError(http.StatusBadRequest, "invalid form data")
		}

		selection, err := buildRegistrationMergeSelection(app, c.Request().Form["delete_ids"])
		if err != nil {
			return echo.NewHTTPError(http.StatusInternalServerError, "failed to evaluate merge selection")
		}

		return renderAdminTemplate(c, "admin_merge_entries_button", AdminMergeButtonData{
			Enabled: selection.Error == "" && len(selection.Entries) >= 2,
		})
	})

	router.POST("/quiz-admin/collections/registrations/merge/modal", func(c echo.Context) error {
		if _, err := requireQuizAdmin(c, app); err != nil {
			return err
		}
		if err := c.Request().ParseForm(); err != nil {
			return echo.NewHTTPError(http.StatusBadRequest, "invalid form data")
		}

		selection, err := buildRegistrationMergeSelection(app, c.Request().Form["delete_ids"])
		if err != nil {
			return echo.NewHTTPError(http.StatusInternalServerError, "failed to build merge preview")
		}

		return renderAdminTemplate(c, "admin_merge_entries_modal", selection)
	})

	router.POST("/quiz-admin/collections/registrations/merge/modal/close", func(c echo.Context) error {
		if _, err := requireQuizAdmin(c, app); err != nil {
			return err
		}
		return c.HTML(http.StatusOK, "")
	})

	router.POST("/quiz-admin/collections/:name/delete", func(c echo.Context) error {
		if _, err := requireQuizAdmin(c, app); err != nil {
			return err
		}

		collection, err := managedCollectionByName(app, strings.TrimSpace(c.PathParam("name")))
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
			record, err := app.Dao().FindRecordById(collection.Name, recordID)
			if err != nil {
				return echo.NewHTTPError(http.StatusNotFound, "record not found")
			}
			if collection.Name == "quiz_dates" {
				if err := deleteRegistrationsForQuizDate(app, recordID); err != nil {
					return echo.NewHTTPError(http.StatusInternalServerError, "failed to delete linked registrations")
				}
			}
			if err := app.Dao().DeleteRecord(record); err != nil {
				return echo.NewHTTPError(http.StatusInternalServerError, "failed to delete record")
			}
		}

		return redirectWithMessage(c, collection.Name, fmt.Sprintf("Deleted %d entrie(s).", len(deleteIDs)), "")
	})
}

func requireQuizAdmin(c echo.Context, app core.App) (*models.Record, error) {
	record, err := currentQuizAdmin(c, app)
	if err != nil {
		clearQuizAdminCookie(c)
		return nil, c.Redirect(http.StatusSeeOther, "/quiz-admin/login")
	}
	return record, nil
}

func currentQuizAdmin(c echo.Context, app core.App) (*models.Record, error) {
	cookie, err := c.Cookie(quizAdminAuthCookieName)
	if err != nil {
		return nil, err
	}
	token := strings.TrimSpace(cookie.Value)
	if token == "" {
		return nil, fmt.Errorf("auth token is empty")
	}

	record, err := app.Dao().FindAuthRecordByToken(token, app.Settings().RecordAuthToken.Secret)
	if err != nil {
		return nil, err
	}
	if record.Collection().Name != quizAdminAuthCollection {
		return nil, fmt.Errorf("invalid auth collection")
	}
	return record, nil
}

func setQuizAdminCookie(c echo.Context, token string) {
	cookie := &http.Cookie{
		Name:     quizAdminAuthCookieName,
		Value:    token,
		Path:     "/quiz-admin",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	}
	c.SetCookie(cookie)
}

func clearQuizAdminCookie(c echo.Context) {
	c.SetCookie(&http.Cookie{
		Name:     quizAdminAuthCookieName,
		Value:    "",
		Path:     "/quiz-admin",
		MaxAge:   -1,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	})
}

func deleteRegistrationsForQuizDate(app core.App, quizDateID string) error {
	registrations, err := app.Dao().FindRecordsByFilter(
		"registrations",
		"quiz = {:quizId}",
		"",
		0,
		0,
		dbx.Params{"quizId": quizDateID},
	)
	if err != nil {
		return err
	}

	for _, registration := range registrations {
		if err := app.Dao().DeleteRecord(registration); err != nil {
			return err
		}
	}

	return nil
}

func managedCollections(app core.App) ([]*models.Collection, error) {
	all := []*models.Collection{}
	if err := app.Dao().CollectionQuery().OrderBy("name ASC").All(&all); err != nil {
		return nil, err
	}

	result := make([]*models.Collection, 0, len(all))
	for _, collection := range all {
		if collection.System || collection.IsAuth() {
			continue
		}
		result = append(result, collection)
	}
	return result, nil
}

func managedCollectionByName(app core.App, name string) (*models.Collection, error) {
	collection, err := app.Dao().FindCollectionByNameOrId(name)
	if err != nil {
		return nil, echo.NewHTTPError(http.StatusNotFound, "collection not found")
	}
	if collection.System || collection.IsAuth() {
		return nil, echo.NewHTTPError(http.StatusForbidden, "this collection type is not available in quiz admin")
	}
	return collection, nil
}

func buildAdminPageData(app core.App, collectionName, successMessage, errorMessage string, showCreate bool, editRecordID string, form AdminFormState) (AdminPageData, error) {
	collections, err := managedCollections(app)
	if err != nil {
		return AdminPageData{}, echo.NewHTTPError(http.StatusInternalServerError, "failed to load collections")
	}

	collection, err := managedCollectionByName(app, collectionName)
	if err != nil {
		return AdminPageData{}, err
	}

	tableFields, formFields := adminFields(collection)
	relationLabels, relationSelectOptions, err := relationDataForCollection(app, collection, tableFields)
	if err != nil {
		return AdminPageData{}, echo.NewHTTPError(http.StatusInternalServerError, "failed to load relation data")
	}

	records, err := loadAdminRecords(app, collection.Name)
	if err != nil {
		return AdminPageData{}, echo.NewHTTPError(http.StatusInternalServerError, "failed to load records")
	}

	rows := make([]AdminRow, 0, len(records))
	registrationMergeRows := map[string]AdminRegistrationMergeRow{}
	for _, record := range records {
		row := AdminRow{
			ID:     record.Id,
			Values: make([]string, 0, len(tableFields)),
		}
		for _, field := range tableFields {
			row.Values = append(row.Values, adminCellValue(record, field, relationLabels[field.Name]))
		}
		rows = append(rows, row)

		if collection.Name == "registrations" {
			quizID := strings.TrimSpace(record.GetString("quiz"))
			registrationMergeRows[record.Id] = AdminRegistrationMergeRow{
				QuizID:         quizID,
				QuizLabel:      relationLabelForID(quizID, relationLabels["quiz"]),
				TeamName:       strings.TrimSpace(record.GetString("team_name")),
				TeamSize:       record.GetInt("team_size"),
				Email:          strings.TrimSpace(record.GetString("email")),
				WillingToMerge: record.GetBool("willing_to_merge"),
			}
		}
	}

	if len(form.Text) == 0 && len(form.Bools) == 0 {
		form = defaultFormState(formFields)
		if editRecordID != "" {
			record, err := app.Dao().FindRecordById(collection.Name, editRecordID)
			if err != nil {
				return AdminPageData{}, echo.NewHTTPError(http.StatusNotFound, "record not found")
			}
			form = formStateFromRecord(record, formFields)
		}
	}

	return AdminPageData{
		Collections:           toCollectionNav(collections),
		ActiveCollection:      collection.Name,
		ActiveType:            collection.Type,
		TableFields:           tableFields,
		FormFields:            formFields,
		Rows:                  rows,
		RegistrationMergeRows: registrationMergeRows,
		RelationSelectOptions: relationSelectOptions,
		Error:                 errorMessage,
		Success:               successMessage,
		ShowCreateModal:       showCreate,
		ShowEditModal:         editRecordID != "",
		EditRecordID:          editRecordID,
		Form:                  form,
	}, nil
}

func loadAdminRecords(app core.App, collectionName string) ([]*models.Record, error) {
	switch collectionName {
	case "quiz_dates":
		records := []*models.Record{}
		err := app.Dao().
			RecordQuery(collectionName).
			OrderBy("scheduled_at ASC", "id ASC").
			All(&records)
		return records, err
	case "registrations":
		records := []*models.Record{}
		err := app.Dao().
			RecordQuery(collectionName).
			Select("registrations.*").
			LeftJoin("quiz_dates", dbx.NewExp("quiz_dates.id = registrations.quiz")).
			OrderBy("quiz_dates.scheduled_at ASC", "registrations.id ASC").
			All(&records)
		return records, err
	default:
		return app.Dao().FindRecordsByExpr(collectionName)
	}
}

func renderAdminPageWithFormError(
	c echo.Context,
	app core.App,
	collectionName string,
	tableFields []AdminField,
	formFields []AdminField,
	showCreate bool,
	showEdit bool,
	editRecordID string,
	form AdminFormState,
	errorMessage string,
) error {
	page, err := buildAdminPageData(app, collectionName, "", errorMessage, showCreate, editRecordID, form)
	if err != nil {
		return err
	}
	page.TableFields = tableFields
	page.FormFields = formFields
	page.ShowCreateModal = showCreate
	page.ShowEditModal = showEdit
	page.EditRecordID = editRecordID
	page.Form = form

	return renderPage(c, PageData{
		Title:               "Quiz Admin",
		PageTemplate:        "admin_collection",
		IsQuizAdmin:         true,
		ShowQuizAdminLogout: true,
		Admin:               page,
	})
}

func toCollectionNav(collections []*models.Collection) []AdminCollectionNav {
	result := make([]AdminCollectionNav, 0, len(collections))
	for _, collection := range collections {
		result = append(result, AdminCollectionNav{
			Name: collection.Name,
			Type: collection.Type,
		})
	}
	return result
}

func adminFields(collection *models.Collection) ([]AdminField, []AdminField) {
	tableFields := []AdminField{}
	formFields := []AdminField{}

	for _, field := range collection.Schema.Fields() {
		if err := field.InitOptions(); err != nil {
			continue
		}
		adminField := AdminField{
			Name:     field.Name,
			Type:     field.Type,
			Editable: !field.System,
			Required: field.Required,
		}
		if opt, ok := field.Options.(schema.MultiValuer); ok {
			adminField.Multiple = opt.IsMultiple()
		}
		tableFields = append(tableFields, adminField)
		if adminField.Editable {
			formFields = append(formFields, adminField)
		}
	}

	tableFields = append(
		tableFields,
		AdminField{Name: "created", Type: "date"},
		AdminField{Name: "updated", Type: "date"},
	)

	return tableFields, formFields
}

func adminCellValue(record *models.Record, field AdminField, relationLabels map[string]string) string {
	switch field.Name {
	case "id":
		return record.Id
	case "created", "updated":
		return formatGermanDateTimeLabel(record.GetString(field.Name))
	default:
		if field.Type == schema.FieldTypeRelation {
			return formatRelationValue(record.Get(field.Name), relationLabels, field.Multiple)
		}
		if field.Type == "date" {
			return formatGermanDateTimeLabel(record.GetString(field.Name))
		}
		return formatAdminValue(record.Get(field.Name))
	}
}

func parseAdminForm(c echo.Context, fields []AdminField) AdminFormState {
	result := defaultFormState(fields)
	for _, field := range fields {
		formKey := "field_" + field.Name
		if field.Type == "bool" {
			result.Bools[field.Name] = strings.EqualFold(strings.TrimSpace(c.FormValue(formKey)), "true")
			continue
		}
		result.Text[field.Name] = strings.TrimSpace(c.FormValue(formKey))
	}
	return result
}

func applyFormToRecord(record *models.Record, formFields []AdminField, form AdminFormState) error {
	for _, field := range formFields {
		if !field.Editable {
			continue
		}

		switch field.Type {
		case "bool":
			record.Set(field.Name, form.Bools[field.Name])
		case "select", "relation", "file":
			if field.Multiple {
				record.Set(field.Name, splitCSVValues(form.Text[field.Name]))
			} else {
				record.Set(field.Name, form.Text[field.Name])
			}
		case "number":
			numberText := strings.TrimSpace(form.Text[field.Name])
			if numberText == "" {
				record.Set(field.Name, "")
				continue
			}
			numberValue, err := strconv.ParseFloat(numberText, 64)
			if err != nil {
				return fmt.Errorf("%s must be a valid number", field.Name)
			}
			record.Set(field.Name, numberValue)
		case "date":
			normalizedDateTime, err := normalizeDateTimeInput(form.Text[field.Name])
			if err != nil {
				return fmt.Errorf("%s must be a valid date-time", field.Name)
			}
			record.Set(field.Name, normalizedDateTime)
		default:
			record.Set(field.Name, form.Text[field.Name])
		}
	}

	return nil
}

func defaultFormState(fields []AdminField) AdminFormState {
	result := AdminFormState{
		Text:  map[string]string{},
		Bools: map[string]bool{},
	}
	for _, field := range fields {
		if field.Type == "bool" {
			result.Bools[field.Name] = false
			continue
		}
		result.Text[field.Name] = ""
	}
	return result
}

func formStateFromRecord(record *models.Record, fields []AdminField) AdminFormState {
	result := defaultFormState(fields)
	for _, field := range fields {
		if field.Type == "bool" {
			result.Bools[field.Name] = record.GetBool(field.Name)
			continue
		}
		if field.Type == "date" {
			result.Text[field.Name] = formatDateTimeInputValue(record.GetString(field.Name))
			continue
		}
		if field.Multiple {
			result.Text[field.Name] = strings.Join(splitCSVValues(formatAdminValue(record.Get(field.Name))), ", ")
			continue
		}
		result.Text[field.Name] = formatAdminValue(record.Get(field.Name))
	}
	return result
}

func formatAdminValue(value any) string {
	switch typed := value.(type) {
	case nil:
		return ""
	case []string:
		return strings.Join(typed, ", ")
	case []any:
		values := make([]string, 0, len(typed))
		for _, item := range typed {
			values = append(values, fmt.Sprintf("%v", item))
		}
		return strings.Join(values, ", ")
	default:
		return fmt.Sprintf("%v", typed)
	}
}

func formatRelationValue(value any, relationLabels map[string]string, multiple bool) string {
	if multiple {
		ids := normalizeStringSlice(value)
		labels := make([]string, 0, len(ids))
		for _, id := range ids {
			labels = append(labels, relationLabelForID(id, relationLabels))
		}
		return strings.Join(labels, ", ")
	}

	id := strings.TrimSpace(formatAdminValue(value))
	if id == "" {
		return ""
	}
	return relationLabelForID(id, relationLabels)
}

func relationLabelForID(id string, relationLabels map[string]string) string {
	if relationLabels == nil {
		return id
	}
	if label, ok := relationLabels[id]; ok && label != "" {
		return label
	}
	return id
}

func normalizeStringSlice(value any) []string {
	switch typed := value.(type) {
	case []string:
		result := make([]string, 0, len(typed))
		for _, item := range typed {
			item = strings.TrimSpace(item)
			if item != "" {
				result = append(result, item)
			}
		}
		return result
	case []any:
		result := make([]string, 0, len(typed))
		for _, item := range typed {
			id := strings.TrimSpace(fmt.Sprintf("%v", item))
			if id != "" {
				result = append(result, id)
			}
		}
		return result
	default:
		raw := strings.TrimSpace(fmt.Sprintf("%v", value))
		if raw == "" || raw == "<nil>" {
			return nil
		}
		return []string{raw}
	}
}

func formatDateTimeInputValue(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}

	when, err := parseDateTime(raw)
	if err != nil {
		return raw
	}

	return when.Local().Format("2006-01-02T15:04")
}

func formatGermanDateTimeLabel(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}

	when, err := parseDateTime(raw)
	if err != nil {
		return raw
	}

	return when.Local().Format("02.01.2006 15:04")
}

func normalizeDateTimeInput(raw string) (string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", nil
	}

	when, err := time.ParseInLocation("2006-01-02T15:04", raw, time.Local)
	if err == nil {
		return when.UTC().Format(time.RFC3339), nil
	}

	when, err = time.ParseInLocation("2006-01-02T15:04:05", raw, time.Local)
	if err == nil {
		return when.UTC().Format(time.RFC3339), nil
	}

	when, err = parseDateTime(raw)
	if err != nil {
		return "", err
	}
	return when.UTC().Format(time.RFC3339), nil
}

func relationDataForCollection(
	app core.App,
	collection *models.Collection,
	fields []AdminField,
) (map[string]map[string]string, map[string][]AdminSelectOption, error) {
	labelMaps := map[string]map[string]string{}
	selectOptions := map[string][]AdminSelectOption{}

	for _, field := range fields {
		if field.Type != schema.FieldTypeRelation {
			continue
		}

		schemaField := collection.Schema.GetFieldByName(field.Name)
		if schemaField == nil {
			continue
		}
		if err := schemaField.InitOptions(); err != nil {
			return nil, nil, err
		}
		relationOptions, ok := schemaField.Options.(*schema.RelationOptions)
		if !ok {
			continue
		}

		relatedCollection, err := app.Dao().FindCollectionByNameOrId(relationOptions.CollectionId)
		if err != nil {
			return nil, nil, err
		}
		relatedRecords, err := app.Dao().FindRecordsByExpr(relatedCollection.Name)
		if err != nil {
			return nil, nil, err
		}

		fieldLabels := make(map[string]string, len(relatedRecords))
		options := make([]AdminSelectOption, 0, len(relatedRecords))
		for _, relatedRecord := range relatedRecords {
			label, err := relationRecordLabel(app, relatedCollection, relatedRecord)
			if err != nil {
				return nil, nil, err
			}
			fieldLabels[relatedRecord.Id] = label
			options = append(options, AdminSelectOption{
				Value: relatedRecord.Id,
				Label: label,
			})
		}

		sort.Slice(options, func(i, j int) bool {
			return strings.ToLower(options[i].Label) < strings.ToLower(options[j].Label)
		})

		labelMaps[field.Name] = fieldLabels
		selectOptions[field.Name] = options
	}

	return labelMaps, selectOptions, nil
}

func relationRecordLabel(app core.App, relatedCollection *models.Collection, record *models.Record) (string, error) {
	if relatedCollection.Name == "quiz_dates" {
		scheduledAtRaw := strings.TrimSpace(record.GetString("scheduled_at"))
		scheduledAtLabel := formatGermanDateTimeLabel(scheduledAtRaw)

		locationLabel := record.GetString("location")
		locationID := strings.TrimSpace(locationLabel)
		if locationID != "" {
			locationRecord, err := app.Dao().FindRecordById("locations", locationID)
			if err != nil {
				return "", err
			}

			locationName := strings.TrimSpace(locationRecord.GetString("name"))
			if locationName != "" {
				locationLabel = locationName
			}
		}

		if scheduledAtLabel != "" && locationLabel != "" {
			return scheduledAtLabel + " - " + locationLabel, nil
		}
		if scheduledAtLabel != "" {
			return scheduledAtLabel, nil
		}
		if locationLabel != "" {
			return locationLabel, nil
		}
		return record.Id, nil
	}

	for _, key := range []string{"name", "title", "team_name", "username", "email"} {
		value := strings.TrimSpace(record.GetString(key))
		if value != "" {
			return value, nil
		}
	}
	return record.Id, nil
}

func renderAdminTemplate(c echo.Context, templateName string, data any) error {
	var buf bytes.Buffer
	if err := tpl.ExecuteTemplate(&buf, templateName, data); err != nil {
		return err
	}
	return c.HTML(http.StatusOK, buf.String())
}

func buildRegistrationMergeSelection(app core.App, selectedIDs []string) (AdminMergeModalData, error) {
	normalizedIDs, err := normalizeRegistrationIDs(selectedIDs)
	if err != nil {
		return AdminMergeModalData{
			Show:  true,
			Error: err.Error(),
		}, nil
	}

	quizDatesCollection, err := app.Dao().FindCollectionByNameOrId("quiz_dates")
	if err != nil {
		return AdminMergeModalData{}, err
	}

	entries := make([]AdminMergeModalEntry, 0, len(normalizedIDs))
	quizLabels := map[string]string{}
	firstQuizID := ""
	firstQuizLabel := ""

	for _, registrationID := range normalizedIDs {
		record, err := app.Dao().FindRecordById("registrations", registrationID)
		if err != nil {
			return AdminMergeModalData{}, err
		}

		quizID := strings.TrimSpace(record.GetString("quiz"))
		if quizID == "" {
			return AdminMergeModalData{
				Show:  true,
				Error: "all selected registrations must have a quiz id",
			}, nil
		}
		if !record.GetBool("willing_to_merge") {
			return AdminMergeModalData{
				Show:  true,
				Error: "all selected registrations must have willing_to_merge set to true",
			}, nil
		}

		quizLabel, ok := quizLabels[quizID]
		if !ok {
			quizRecord, err := app.Dao().FindRecordById("quiz_dates", quizID)
			if err != nil {
				return AdminMergeModalData{}, err
			}
			quizLabel, err = relationRecordLabel(app, quizDatesCollection, quizRecord)
			if err != nil {
				return AdminMergeModalData{}, err
			}
			quizLabels[quizID] = quizLabel
		}

		if firstQuizID == "" {
			firstQuizID = quizID
			firstQuizLabel = quizLabel
		}
		if quizID != firstQuizID {
			return AdminMergeModalData{
				Show:  true,
				Error: "all selected registrations must belong to the same quiz",
			}, nil
		}

		entries = append(entries, AdminMergeModalEntry{
			ID:       registrationID,
			TeamName: strings.TrimSpace(record.GetString("team_name")),
			TeamSize: record.GetInt("team_size"),
			Email:    strings.TrimSpace(record.GetString("email")),
		})
	}

	return AdminMergeModalData{
		Show:            true,
		Summary:         fmt.Sprintf("%d entries from %s will be merged.", len(entries), firstQuizLabel),
		RegistrationIDs: strings.Join(normalizedIDs, ","),
		Entries:         entries,
	}, nil
}

func splitCSVValues(raw string) []string {
	raw = strings.ReplaceAll(raw, "\n", ",")
	parts := strings.Split(raw, ",")
	values := make([]string, 0, len(parts))
	for _, part := range parts {
		value := strings.TrimSpace(part)
		if value == "" {
			continue
		}
		values = append(values, value)
	}
	return values
}

func redirectWithMessage(c echo.Context, collectionName, successMessage, errorMessage string) error {
	redirectURL := "/quiz-admin/collections/" + url.PathEscape(collectionName)
	params := url.Values{}
	if successMessage != "" {
		params.Set("success", successMessage)
	}
	if errorMessage != "" {
		params.Set("error", errorMessage)
	}
	if encoded := params.Encode(); encoded != "" {
		redirectURL += "?" + encoded
	}
	return c.Redirect(http.StatusSeeOther, redirectURL)
}
