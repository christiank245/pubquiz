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

type routeRenderers struct {
	page     func(c echo.Context, data PageData) error
	panel    func(c echo.Context, data RegisterPanelData, htmx bool) error
	template func(c echo.Context, templateName string, data any) error
}

type publicHandlers struct {
	app       core.App
	renderers routeRenderers
}

func BuildPublicHandlers(
	app core.App,
	pageRenderer func(c echo.Context, data PageData) error,
	panelRenderer func(c echo.Context, data RegisterPanelData, htmx bool) error,
	templateRenderer func(c echo.Context, templateName string, data any) error,
) PublicHandlers {
	h := publicHandlers{
		app: app,
		renderers: routeRenderers{
			page:     pageRenderer,
			panel:    panelRenderer,
			template: templateRenderer,
		},
	}
	return PublicHandlers{
		Home:           h.home,
		About:          h.about,
		Contact:        h.contact,
		Privacy:        h.privacy,
		Quiz:           h.quiz,
		Register:       h.register,
		UnregisterGet:  h.unregisterGet,
		UnregisterPost: h.unregisterPost,
	}
}

func (h publicHandlers) renderStaticPage(c echo.Context, title, templateName string) error {
	if h.renderers.page == nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "page renderer is not configured")
	}
	return h.renderers.page(c, PageData{
		Title:        title,
		PageTemplate: templateName,
	})
}

func (h publicHandlers) home(c echo.Context) error {
	if h.renderers.page == nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "page renderer is not configured")
	}
	if h.renderers.template == nil {
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
			return h.renderers.template(c, "quiz_overview", page)
		}
		return h.renderers.page(c, page)
	}
	page := PageData{
		Title:        "Pub Quiz Dates",
		PageTemplate: "home",
		Quizzes:      quizzes,
	}
	if isHTMXRequest(c) {
		return h.renderers.template(c, "quiz_overview", page)
	}
	return h.renderers.page(c, page)
}

func (h publicHandlers) about(c echo.Context) error {
	return h.renderStaticPage(c, "About Us", "about")
}

func (h publicHandlers) contact(c echo.Context) error {
	return h.renderStaticPage(c, "Contact Us", "contact")
}

func (h publicHandlers) privacy(c echo.Context) error {
	return h.renderStaticPage(c, "Privacy Policy", "privacy")
}

func (h publicHandlers) quiz(c echo.Context) error {
	if h.renderers.page == nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "page renderer is not configured")
	}
	if h.renderers.template == nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "template renderer is not configured")
	}

	quizID := strings.TrimSpace(c.QueryParam("id"))
	if quizID == "" {
		return h.renderers.page(c, PageData{
			Title:        "404 - Page not found",
			PageTemplate: "not_found",
			StatusCode:   http.StatusNotFound,
		})
	}

	quiz, seatsLeft, err := LoadQuiz(h.app, quizID)
	if err != nil {
		log.Printf("quiz lookup failed for %q: %v", quizID, err)
		return h.renderers.page(c, PageData{
			Title:        "404 - Page not found",
			PageTemplate: "not_found",
			StatusCode:   http.StatusNotFound,
		})
	}

	if isHTMXRequest(c) {
		return h.renderers.template(c, "quiz_registration_detail", PageData{
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
		return h.renderers.page(c, PageData{
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

	return h.renderers.page(c, PageData{
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
	if h.renderers.panel == nil {
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
		return h.renderers.panel(c, panel, isHTMX)
	}

	teamSize, err := strconv.Atoi(teamSizeRaw)
	if err != nil || teamSize < 1 {
		panel.Error = "Team size must be a number greater than 0."
		return h.renderers.panel(c, panel, isHTMX)
	}

	if teamSize > seatsLeft {
		panel.Error = fmt.Sprintf("Only %d seat(s) left for this quiz.", seatsLeft)
		return h.renderers.panel(c, panel, isHTMX)
	}

	mergeConsent := false
	if teamSize < 4 {
		if confirmMerge == "" {
			panel.ShowMergePrompt = true
			return h.renderers.panel(c, panel, isHTMX)
		}
		mergeConsent, err = ParseMergeConsent(confirmMerge)
		if err != nil {
			panel.Error = "Please choose whether your team can be merged with another small team."
			return h.renderers.panel(c, panel, isHTMX)
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
			return h.renderers.panel(c, panel, isHTMX)
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
			return h.renderers.panel(c, panel, isHTMX)
		}
	}

	col, err := h.app.Dao().FindCollectionByNameOrId("registrations")
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "registrations collection is missing")
	}

	if err := SaveRegistrations(h.app.Dao(), col, quizID, email, registrationSizes, registrationTeamNames, mergeConsent); err != nil {
		panel.Error = "Failed to save registration."
		return h.renderers.panel(c, panel, isHTMX)
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

	return h.renderers.panel(c, panel, isHTMX)
}

func (h publicHandlers) unregisterGet(c echo.Context) error {
	if h.renderers.page == nil {
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

	return h.renderers.page(c, PageData{
		Title:        "Unregister Team",
		PageTemplate: "unregister",
		Unregister:   unregisterData,
	})
}

func (h publicHandlers) unregisterPost(c echo.Context) error {
	if h.renderers.page == nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "page renderer is not configured")
	}

	registrationID := strings.TrimSpace(c.FormValue("registration_id"))
	if registrationID == "" {
		return echo.NewHTTPError(http.StatusBadRequest, "registration id is required")
	}

	unregisterData, err := LoadRegistrationForUnregister(h.app, registrationID)
	if err != nil {
		return h.renderers.page(c, PageData{
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

	return h.renderers.page(c, PageData{
		Title:        "Unregister Team",
		PageTemplate: "unregister",
		Unregister:   unregisterData,
	})
}
