package api

import (
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"

	"github.com/labstack/echo/v5"
	"github.com/pocketbase/pocketbase/core"
)

var renderPageFn func(c echo.Context, data PageData) error
var renderRegisterPanelFn func(c echo.Context, data RegisterPanelData, htmx bool) error
var renderTemplateFn func(c echo.Context, templateName string, data any) error

func SetPublicRenderers(
	pageRenderer func(c echo.Context, data PageData) error,
	panelRenderer func(c echo.Context, data RegisterPanelData, htmx bool) error,
) {
	renderPageFn = pageRenderer
	renderRegisterPanelFn = panelRenderer
}

func SetAdminTemplateRenderer(templateRenderer func(c echo.Context, templateName string, data any) error) {
	renderTemplateFn = templateRenderer
}

type publicHandlers struct {
	app core.App
}

func BuildPublicHandlers(app core.App) PublicHandlers {
	h := publicHandlers{app: app}
	return PublicHandlers{
		Home:           h.home,
		Quiz:           h.quiz,
		Register:       h.register,
		UnregisterGet:  h.unregisterGet,
		UnregisterPost: h.unregisterPost,
	}
}

func (h publicHandlers) home(c echo.Context) error {
	if renderPageFn == nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "page renderer is not configured")
	}
	if renderTemplateFn == nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "template renderer is not configured")
	}

	quizzes, err := LoadUpcomingQuizzes(h.app)
	if err != nil {
		log.Printf("home load failed: %v", err)
		page := PageData{
			Title:        "Pub Quiz Dates",
			PageTemplate: "home",
			SetupError:   "The database collections are not ready yet. Import pb_schema.json in the PocketBase Admin UI, then add locations and quiz dates.",
		}
		if isHTMXRequest(c) {
			return renderTemplateFn(c, "quiz_overview", page)
		}
		return renderPageFn(c, page)
	}
	page := PageData{
		Title:        "Pub Quiz Dates",
		PageTemplate: "home",
		Quizzes:      quizzes,
	}
	if isHTMXRequest(c) {
		return renderTemplateFn(c, "quiz_overview", page)
	}
	return renderPageFn(c, page)
}

func (h publicHandlers) quiz(c echo.Context) error {
	if renderPageFn == nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "page renderer is not configured")
	}

	quizID := strings.TrimSpace(c.QueryParam("id"))
	if quizID == "" {
		return echo.NewHTTPError(http.StatusBadRequest, "quiz id is required")
	}

	quiz, seatsLeft, err := LoadQuiz(h.app, quizID)
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "failed to load quiz")
	}

	if isHTMXRequest(c) {
		return renderTemplateFn(c, "quiz_registration_detail", PageData{
			Title: "Quiz Registration",
			Quiz:  quiz,
			Register: RegisterPanelData{
				Quiz:      quiz,
				TeamSize:  "1",
				SeatsLeft: seatsLeft,
			},
		})
	}

	quizzes, err := LoadUpcomingQuizzes(h.app)
	if err != nil {
		log.Printf("quiz load failed: %v", err)
		return renderPageFn(c, PageData{
			Title:        "Quiz Registration",
			PageTemplate: "quiz",
			SetupError:   "The database collections are not ready yet. Import pb_schema.json in the PocketBase Admin UI, then add locations and quiz dates.",
			Quiz:         quiz,
			Register: RegisterPanelData{
				Quiz:      quiz,
				TeamSize:  "1",
				SeatsLeft: seatsLeft,
			},
		})
	}

	return renderPageFn(c, PageData{
		Title:        "Quiz Registration",
		PageTemplate: "quiz",
		Quizzes:      quizzes,
		Quiz:         quiz,
		Register: RegisterPanelData{
			Quiz:      quiz,
			TeamSize:  "1",
			SeatsLeft: seatsLeft,
		},
	})
}

func (h publicHandlers) register(c echo.Context) error {
	if renderRegisterPanelFn == nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "register panel renderer is not configured")
	}

	quizID := strings.TrimSpace(c.FormValue("quiz_id"))
	email := strings.TrimSpace(c.FormValue("email"))
	teamName := strings.TrimSpace(c.FormValue("team_name"))
	teamSizeRaw := strings.TrimSpace(c.FormValue("team_size"))
	confirmSplit := strings.EqualFold(strings.TrimSpace(c.FormValue("confirm_split")), "true")
	confirmMerge := strings.TrimSpace(c.FormValue("confirm_merge"))
	isHTMX := isHTMXRequest(c)

	quiz, seatsLeft, err := LoadQuiz(h.app, quizID)
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "failed to load quiz")
	}

	panel := RegisterPanelData{
		Quiz:      quiz,
		Email:     email,
		TeamName:  teamName,
		TeamSize:  teamSizeRaw,
		SeatsLeft: seatsLeft,
	}

	if email == "" || teamName == "" || teamSizeRaw == "" {
		panel.Error = "Please fill all fields."
		return renderRegisterPanelFn(c, panel, isHTMX)
	}

	teamSize, err := strconv.Atoi(teamSizeRaw)
	if err != nil || teamSize < 1 {
		panel.Error = "Team size must be a number greater than 0."
		return renderRegisterPanelFn(c, panel, isHTMX)
	}

	if teamSize > seatsLeft {
		panel.Error = fmt.Sprintf("Only %d seat(s) left for this quiz.", seatsLeft)
		return renderRegisterPanelFn(c, panel, isHTMX)
	}

	mergeConsent := false
	if teamSize < 4 {
		if confirmMerge == "" {
			panel.ShowMergePrompt = true
			return renderRegisterPanelFn(c, panel, isHTMX)
		}
		mergeConsent, err = ParseMergeConsent(confirmMerge)
		if err != nil {
			panel.Error = "Please choose whether your team can be merged with another small team."
			return renderRegisterPanelFn(c, panel, isHTMX)
		}
	}

	registrationSizes := []int{teamSize}
	if teamSize > MaxTeamSize {
		registrationSizes = SplitTeamSizes(teamSize, MaxTeamSize)
		if !confirmSplit {
			panel.ShowSplitPrompt = true
			panel.SplitTeamCount = len(registrationSizes)
			panel.SplitSizes = registrationSizes
			panel.SplitSizesLabel = JoinIntSizes(registrationSizes)
			panel.SplitTeamNames = DefaultSplitTeamNames(teamName, len(registrationSizes))
			return renderRegisterPanelFn(c, panel, isHTMX)
		}
	}

	registrationTeamNames := []string{teamName}
	if len(registrationSizes) > 1 {
		registrationTeamNames = SubmittedSplitTeamNames(c, len(registrationSizes))
		if HasEmptyValues(registrationTeamNames) {
			panel.Error = "Please provide a name for every split team."
			panel.ShowSplitPrompt = true
			panel.SplitTeamCount = len(registrationSizes)
			panel.SplitSizes = registrationSizes
			panel.SplitSizesLabel = JoinIntSizes(registrationSizes)
			panel.SplitTeamNames = registrationTeamNames
			return renderRegisterPanelFn(c, panel, isHTMX)
		}
	}

	col, err := h.app.Dao().FindCollectionByNameOrId("registrations")
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "registrations collection is missing")
	}

	if err := SaveRegistrations(h.app.Dao(), col, quizID, email, registrationSizes, registrationTeamNames, mergeConsent); err != nil {
		panel.Error = "Failed to save registration."
		return renderRegisterPanelFn(c, panel, isHTMX)
	}

	remainingSeats := seatsLeft - teamSize
	if remainingSeats <= 0 {
		quizRec, err := h.app.Dao().FindRecordById("quiz_dates", quizID)
		if err != nil {
			return echo.NewHTTPError(http.StatusInternalServerError, "failed to refresh quiz status")
		}
		if quizRec.GetBool("is_open") {
			if err := CloseQuiz(h.app, quizRec); err != nil {
				return echo.NewHTTPError(http.StatusInternalServerError, "failed to close full quiz")
			}
		}
	}

	panel.ShowSuccess = true
	if len(registrationSizes) > 1 {
		panel.Success = fmt.Sprintf(
			"Registration successful. Your group was split into %d teams (%s). See you at the quiz!",
			len(registrationSizes),
			JoinIntSizes(registrationSizes),
		)
	} else {
		panel.Success = "Registration successful. See you at the quiz!"
	}
	panel.Email = ""
	panel.TeamName = ""
	panel.TeamSize = "1"
	panel.SeatsLeft = max(remainingSeats, 0)

	return renderRegisterPanelFn(c, panel, isHTMX)
}

func (h publicHandlers) unregisterGet(c echo.Context) error {
	if renderPageFn == nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "page renderer is not configured")
	}

	registrationID := strings.TrimSpace(c.QueryParam("id"))
	if registrationID == "" {
		return echo.NewHTTPError(http.StatusBadRequest, "registration id is required")
	}

	unregisterData, err := LoadRegistrationForUnregister(h.app, registrationID)
	if err != nil {
		return echo.NewHTTPError(http.StatusNotFound, "registration not found")
	}
	unregisterData.ShowConfirm = true

	return renderPageFn(c, PageData{
		Title:        "Unregister Team",
		PageTemplate: "unregister",
		Unregister:   unregisterData,
	})
}

func (h publicHandlers) unregisterPost(c echo.Context) error {
	if renderPageFn == nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "page renderer is not configured")
	}

	registrationID := strings.TrimSpace(c.FormValue("registration_id"))
	if registrationID == "" {
		return echo.NewHTTPError(http.StatusBadRequest, "registration id is required")
	}

	unregisterData, err := LoadRegistrationForUnregister(h.app, registrationID)
	if err != nil {
		return renderPageFn(c, PageData{
			Title:        "Unregister Team",
			PageTemplate: "unregister",
			Unregister: UnregisterViewData{
				RegistrationID: registrationID,
				Error:          "Registration not found or already removed.",
			},
		})
	}

	record, err := h.app.Dao().FindRecordById("registrations", registrationID)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to load registration")
	}
	if err := h.app.Dao().DeleteRecord(record); err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to unregister")
	}

	unregisterData.Success = "You have been successfully unregistered."
	unregisterData.ShowConfirm = false

	return renderPageFn(c, PageData{
		Title:        "Unregister Team",
		PageTemplate: "unregister",
		Unregister:   unregisterData,
	})
}
