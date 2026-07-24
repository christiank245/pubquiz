package main

import (
	"bytes"
	"database/sql"
	"embed"
	"errors"
	"fmt"
	"html/template"
	"io/fs"
	"log"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v4"
	"github.com/labstack/echo/v5"
	"github.com/pocketbase/dbx"
	"github.com/pocketbase/pocketbase"
	"github.com/pocketbase/pocketbase/apis"
	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/daos"
	"github.com/pocketbase/pocketbase/models"
	"github.com/pocketbase/pocketbase/plugins/migratecmd"
	"github.com/pocketbase/pocketbase/tools/security"

	_ "pubquiz-website2.0/migrations"
)

//go:embed web/templates web/public
var webFiles embed.FS

var tpl = template.Must(template.New("site").Funcs(template.FuncMap{
	"add1": func(i int) int { return i + 1 },
	"contains": func(values []string, target string) bool {
		for _, value := range values {
			if value == target {
				return true
			}
		}
		return false
	},
}).ParseFS(
	webFiles,
	"web/templates/*.html",
	"web/templates/components/*.html",
	"web/templates/pages/*.html",
))

const (
	quizAutoCloseBeforeStart = time.Hour
	maxTeamSize              = 10
	adminSessionCookieName   = "pubquiz_admin_session"
)

type QuizCard struct {
	ID             string
	When           time.Time
	WhenLabel      string
	LocationName   string
	MapsURL        string
	Capacity       int
	SeatsRemaining int
}

type RegisterPanelData struct {
	Quiz            QuizCard
	Error           string
	Success         string
	Email           string
	TeamName        string
	TeamSize        string
	SeatsLeft       int
	ShowSuccess     bool
	ShowSplitPrompt bool
	ShowMergePrompt bool
	SplitSizesLabel string
	SplitTeamCount  int
	SplitSizes      []int
	SplitTeamNames  []string
}

type UnregisterViewData struct {
	RegistrationID string
	TeamName       string
	TeamSize       int
	Email          string
	Error          string
	Success        string
	ShowConfirm    bool
}

type AdminMergeViewData struct {
	Events                  []QuizCard
	SelectedQuizID          string
	EligibleRegistrations   []AdminMergeRegistrationRow
	SelectedRegistrationIDs []string
	TeamName                string
	Error                   string
	Success                 string
	MergedRecordID          string
	MergedQuizID            string
	MergedTeamName          string
	MergedTeamSize          int
	MergedEmail             string
	MergedFromIDs           []string
}

type AdminMergeRegistrationRow struct {
	ID       string
	TeamName string
	TeamSize int
	Email    string
}

type AdminLoginViewData struct {
	Email    string
	Redirect string
	Error    string
}

type PageData struct {
	Title               string
	PageTemplate        string
	IsQuizAdmin         bool
	ShowQuizAdminLogout bool
	SetupError          string
	Quizzes             []QuizCard
	Quiz                QuizCard
	Register            RegisterPanelData
	Unregister          UnregisterViewData
	Admin               AdminPageData
	AdminMerge          AdminMergeViewData
	AdminLogin          AdminLoginViewData
}

type MergeRegistrationsRequest struct {
	RegistrationIDs []string `json:"registration_ids"`
	TeamName        string   `json:"team_name"`
	Email           string   `json:"email"`
	WillingToMerge  *bool    `json:"willing_to_merge"`
}

type mergeRequestError struct {
	message string
}

func (e mergeRequestError) Error() string {
	return e.message
}

func main() {
	app := pocketbase.New()
	migratecmd.MustRegister(app, app.RootCmd, migratecmd.Config{})

	app.OnBeforeServe().Add(func(e *core.ServeEvent) error {
		assetFS, err := fs.Sub(webFiles, "web/public")
		if err != nil {
			return err
		}

		e.Router.GET("/assets/*", apis.StaticDirectoryHandler(assetFS, false))

		requireAdminSession := func(next echo.HandlerFunc) echo.HandlerFunc {
			return func(c echo.Context) error {
				if _, err := currentAdminFromSession(app, c); err != nil {
					loginTarget := "/admin/login?redirect=" + url.QueryEscape(c.Request().URL.RequestURI())
					return c.Redirect(http.StatusSeeOther, loginTarget)
				}
				return next(c)
			}
		}

		e.Router.AddRoute(echo.Route{
			Method: http.MethodGet,
			Path:   "/",
			Handler: func(c echo.Context) error {
				quizzes, err := loadUpcomingQuizzes(app)
				if err != nil {
					log.Printf("home load failed: %v", err)
					return renderPage(c, PageData{
						Title:        "Pub Quiz Dates",
						PageTemplate: "home",
						SetupError:   "The database collections are not ready yet. Import pb_schema.json in the PocketBase Admin UI, then add locations and quiz dates.",
					})
				}
				return renderPage(c, PageData{
					Title:        "Pub Quiz Dates",
					PageTemplate: "home",
					Quizzes:      quizzes,
				})
			},
		})

		e.Router.AddRoute(echo.Route{
			Method: http.MethodGet,
			Path:   "/quiz",
			Handler: func(c echo.Context) error {
				quizID := strings.TrimSpace(c.QueryParam("id"))
				if quizID == "" {
					return echo.NewHTTPError(http.StatusBadRequest, "quiz id is required")
				}

				quiz, seatsLeft, err := loadQuiz(app, quizID)
				if err != nil {
					return echo.NewHTTPError(http.StatusBadRequest, "failed to load quiz")
				}

				return renderPage(c, PageData{
					Title:        "Quiz Registration",
					PageTemplate: "quiz",
					Quiz:         quiz,
					Register: RegisterPanelData{
						Quiz:      quiz,
						TeamSize:  "1",
						SeatsLeft: seatsLeft,
					},
				})
			},
		})

		e.Router.AddRoute(echo.Route{
			Method: http.MethodPost,
			Path:   "/register",
			Handler: func(c echo.Context) error {
				quizID := strings.TrimSpace(c.FormValue("quiz_id"))
				email := strings.TrimSpace(c.FormValue("email"))
				teamName := strings.TrimSpace(c.FormValue("team_name"))
				teamSizeRaw := strings.TrimSpace(c.FormValue("team_size"))
				confirmSplit := strings.EqualFold(strings.TrimSpace(c.FormValue("confirm_split")), "true")
				confirmMerge := strings.TrimSpace(c.FormValue("confirm_merge"))
				isHTMX := strings.EqualFold(c.Request().Header.Get("HX-Request"), "true")

				quiz, seatsLeft, err := loadQuiz(app, quizID)
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
					return renderRegisterPanel(c, panel, isHTMX)
				}

				teamSize, err := strconv.Atoi(teamSizeRaw)
				if err != nil || teamSize < 1 {
					panel.Error = "Team size must be a number greater than 0."
					return renderRegisterPanel(c, panel, isHTMX)
				}

				if teamSize > seatsLeft {
					panel.Error = fmt.Sprintf("Only %d seat(s) left for this quiz.", seatsLeft)
					return renderRegisterPanel(c, panel, isHTMX)
				}

				mergeConsent := false
				if teamSize < 4 {
					if confirmMerge == "" {
						panel.ShowMergePrompt = true
						return renderRegisterPanel(c, panel, isHTMX)
					}
					mergeConsent, err = parseMergeConsent(confirmMerge)
					if err != nil {
						panel.Error = "Please choose whether your team can be merged with another small team."
						return renderRegisterPanel(c, panel, isHTMX)
					}
				}

				registrationSizes := []int{teamSize}
				if teamSize > maxTeamSize {
					registrationSizes = splitTeamSizes(teamSize, maxTeamSize)
					if !confirmSplit {
						panel.ShowSplitPrompt = true
						panel.SplitTeamCount = len(registrationSizes)
						panel.SplitSizes = registrationSizes
						panel.SplitSizesLabel = joinIntSizes(registrationSizes)
						panel.SplitTeamNames = defaultSplitTeamNames(teamName, len(registrationSizes))
						return renderRegisterPanel(c, panel, isHTMX)
					}
				}

				registrationTeamNames := []string{teamName}
				if len(registrationSizes) > 1 {
					registrationTeamNames = submittedSplitTeamNames(c, len(registrationSizes))
					if hasEmptyValues(registrationTeamNames) {
						panel.Error = "Please provide a name for every split team."
						panel.ShowSplitPrompt = true
						panel.SplitTeamCount = len(registrationSizes)
						panel.SplitSizes = registrationSizes
						panel.SplitSizesLabel = joinIntSizes(registrationSizes)
						panel.SplitTeamNames = registrationTeamNames
						return renderRegisterPanel(c, panel, isHTMX)
					}
				}

				col, err := app.Dao().FindCollectionByNameOrId("registrations")
				if err != nil {
					return echo.NewHTTPError(http.StatusInternalServerError, "registrations collection is missing")
				}

				if err := saveRegistrations(app.Dao(), col, quizID, email, registrationSizes, registrationTeamNames, mergeConsent); err != nil {
					panel.Error = "Failed to save registration."
					return renderRegisterPanel(c, panel, isHTMX)
				}

				remainingSeats := seatsLeft - teamSize
				if remainingSeats <= 0 {
					quizRec, err := app.Dao().FindRecordById("quiz_dates", quizID)
					if err != nil {
						return echo.NewHTTPError(http.StatusInternalServerError, "failed to refresh quiz status")
					}
					if quizRec.GetBool("is_open") {
						if err := closeQuiz(app, quizRec); err != nil {
							return echo.NewHTTPError(http.StatusInternalServerError, "failed to close full quiz")
						}
					}
				}

				panel.ShowSuccess = true
				if len(registrationSizes) > 1 {
					panel.Success = fmt.Sprintf(
						"Registration successful. Your group was split into %d teams (%s). See you at the quiz!",
						len(registrationSizes),
						joinIntSizes(registrationSizes),
					)
				} else {
					panel.Success = "Registration successful. See you at the quiz!"
				}
				panel.Email = ""
				panel.TeamName = ""
				panel.TeamSize = "1"
				panel.SeatsLeft = max(remainingSeats, 0)

				return renderRegisterPanel(c, panel, isHTMX)
			},
		})

		e.Router.AddRoute(echo.Route{
			Method: http.MethodGet,
			Path:   "/unregister",
			Handler: func(c echo.Context) error {
				registrationID := strings.TrimSpace(c.QueryParam("id"))
				if registrationID == "" {
					return echo.NewHTTPError(http.StatusBadRequest, "registration id is required")
				}

				unregisterData, err := loadRegistrationForUnregister(app, registrationID)
				if err != nil {
					return echo.NewHTTPError(http.StatusNotFound, "registration not found")
				}
				unregisterData.ShowConfirm = true

				return renderPage(c, PageData{
					Title:        "Unregister Team",
					PageTemplate: "unregister",
					Unregister:   unregisterData,
				})
			},
		})

		e.Router.AddRoute(echo.Route{
			Method: http.MethodPost,
			Path:   "/unregister",
			Handler: func(c echo.Context) error {
				registrationID := strings.TrimSpace(c.FormValue("registration_id"))
				if registrationID == "" {
					return echo.NewHTTPError(http.StatusBadRequest, "registration id is required")
				}

				unregisterData, err := loadRegistrationForUnregister(app, registrationID)
				if err != nil {
					return renderPage(c, PageData{
						Title:        "Unregister Team",
						PageTemplate: "unregister",
						Unregister: UnregisterViewData{
							RegistrationID: registrationID,
							Error:          "Registration not found or already removed.",
						},
					})
				}

				record, err := app.Dao().FindRecordById("registrations", registrationID)
				if err != nil {
					return echo.NewHTTPError(http.StatusInternalServerError, "failed to load registration")
				}
				if err := app.Dao().DeleteRecord(record); err != nil {
					return echo.NewHTTPError(http.StatusInternalServerError, "failed to unregister")
				}

				unregisterData.Success = "You have been successfully unregistered."
				unregisterData.ShowConfirm = false

				return renderPage(c, PageData{
					Title:        "Unregister Team",
					PageTemplate: "unregister",
					Unregister:   unregisterData,
				})
			},
		})

		e.Router.AddRoute(echo.Route{
			Method:      http.MethodPost,
			Path:        "/api/admin/registrations/merge",
			Middlewares: []echo.MiddlewareFunc{apis.RequireAdminAuth()},
			Handler: func(c echo.Context) error {
				var payload MergeRegistrationsRequest
				if err := c.Bind(&payload); err != nil {
					return echo.NewHTTPError(http.StatusBadRequest, "invalid request body")
				}

				registrationIDs, err := normalizeRegistrationIDs(payload.RegistrationIDs)
				if err != nil {
					return echo.NewHTTPError(http.StatusBadRequest, err.Error())
				}

				col, err := app.Dao().FindCollectionByNameOrId("registrations")
				if err != nil {
					return echo.NewHTTPError(http.StatusInternalServerError, "registrations collection is missing")
				}

				mergedRecord, err := mergeRegistrations(
					app.Dao(),
					col,
					registrationIDs,
					payload.TeamName,
					payload.Email,
					payload.WillingToMerge,
				)
				if err != nil {
					var reqErr mergeRequestError
					if errors.As(err, &reqErr) {
						return echo.NewHTTPError(http.StatusBadRequest, reqErr.Error())
					}
					return echo.NewHTTPError(http.StatusInternalServerError, "failed to merge registrations")
				}

				return c.JSON(http.StatusOK, map[string]any{
					"message":                "Registrations merged successfully.",
					"merged_registration_id": mergedRecord.Id,
					"quiz_id":                mergedRecord.GetString("quiz"),
					"team_name":              mergedRecord.GetString("team_name"),
					"team_size":              mergedRecord.GetInt("team_size"),
					"email":                  mergedRecord.GetString("email"),
					"willing_to_merge":       mergedRecord.GetBool("willing_to_merge"),
					"merged_from_ids":        registrationIDs,
				})
			},
		})

		e.Router.AddRoute(echo.Route{
			Method: http.MethodGet,
			Path:   "/admin/login",
			Handler: func(c echo.Context) error {
				redirectTo := strings.TrimSpace(c.QueryParam("redirect"))
				if redirectTo == "" {
					redirectTo = "/admin/registrations/merge"
				}

				return renderPage(c, PageData{
					Title:        "Admin Login",
					PageTemplate: "admin_login",
					AdminLogin: AdminLoginViewData{
						Redirect: redirectTo,
					},
				})
			},
		})

		e.Router.AddRoute(echo.Route{
			Method: http.MethodPost,
			Path:   "/admin/login",
			Handler: func(c echo.Context) error {
				email := strings.TrimSpace(c.FormValue("email"))
				password := c.FormValue("password")
				redirectTo := sanitizeAdminRedirect(c.FormValue("redirect"))

				loginData := AdminLoginViewData{
					Email:    email,
					Redirect: redirectTo,
				}

				if email == "" || password == "" {
					loginData.Error = "Email and password are required."
					return renderPage(c, PageData{
						Title:        "Admin Login",
						PageTemplate: "admin_login",
						AdminLogin:   loginData,
					})
				}

				admin, err := app.Dao().FindAdminByEmail(email)
				if err != nil || !admin.ValidatePassword(password) {
					loginData.Error = "Invalid admin credentials."
					return renderPage(c, PageData{
						Title:        "Admin Login",
						PageTemplate: "admin_login",
						AdminLogin:   loginData,
					})
				}

				sessionToken, sessionDuration, err := buildAdminSessionToken(app, admin)
				if err != nil {
					return echo.NewHTTPError(http.StatusInternalServerError, "failed to create admin session")
				}

				c.SetCookie(&http.Cookie{
					Name:     adminSessionCookieName,
					Value:    sessionToken,
					Path:     "/",
					HttpOnly: true,
					SameSite: http.SameSiteLaxMode,
					Secure:   c.Scheme() == "https",
					MaxAge:   int(sessionDuration.Seconds()),
				})

				return c.Redirect(http.StatusSeeOther, redirectTo)
			},
		})

		e.Router.AddRoute(echo.Route{
			Method:      http.MethodPost,
			Path:        "/admin/logout",
			Middlewares: []echo.MiddlewareFunc{requireAdminSession},
			Handler: func(c echo.Context) error {
				c.SetCookie(&http.Cookie{
					Name:     adminSessionCookieName,
					Value:    "",
					Path:     "/",
					HttpOnly: true,
					SameSite: http.SameSiteLaxMode,
					Secure:   c.Scheme() == "https",
					MaxAge:   -1,
				})
				return c.Redirect(http.StatusSeeOther, "/admin/login")
			},
		})

		e.Router.AddRoute(echo.Route{
			Method:      http.MethodGet,
			Path:        "/admin/registrations/merge",
			Middlewares: []echo.MiddlewareFunc{requireAdminSession},
			Handler: func(c echo.Context) error {
				selectedQuizID := strings.TrimSpace(c.QueryParam("quiz_id"))

				events, err := loadUpcomingQuizzes(app)
				if err != nil {
					return echo.NewHTTPError(http.StatusInternalServerError, "failed to load upcoming quizzes")
				}

				mergeData := AdminMergeViewData{
					Events:         events,
					SelectedQuizID: selectedQuizID,
				}

				if selectedQuizID != "" {
					if !quizIDExists(events, selectedQuizID) {
						mergeData.Error = "Selected quiz is not available."
					} else {
						rows, err := loadMergeEligibleRegistrations(app, selectedQuizID)
						if err != nil {
							return echo.NewHTTPError(http.StatusInternalServerError, "failed to load merge candidates")
						}
						mergeData.EligibleRegistrations = rows
					}
				}

				return renderPage(c, PageData{
					Title:        "Admin: Merge Registrations",
					PageTemplate: "admin_merge",
					AdminMerge:   mergeData,
				})
			},
		})

		e.Router.AddRoute(echo.Route{
			Method:      http.MethodPost,
			Path:        "/admin/registrations/merge",
			Middlewares: []echo.MiddlewareFunc{requireAdminSession},
			Handler: func(c echo.Context) error {
				selectedQuizID := strings.TrimSpace(c.FormValue("quiz_id"))
				teamName := strings.TrimSpace(c.FormValue("team_name"))

				events, err := loadUpcomingQuizzes(app)
				if err != nil {
					return echo.NewHTTPError(http.StatusInternalServerError, "failed to load upcoming quizzes")
				}

				mergeData := AdminMergeViewData{
					Events:         events,
					SelectedQuizID: selectedQuizID,
					TeamName:       teamName,
				}

				if selectedQuizID == "" {
					mergeData.Error = "Please select a quiz event first."
					return renderPage(c, PageData{
						Title:        "Admin: Merge Registrations",
						PageTemplate: "admin_merge",
						AdminMerge:   mergeData,
					})
				}
				if !quizIDExists(events, selectedQuizID) {
					mergeData.Error = "Selected quiz is not available."
					return renderPage(c, PageData{
						Title:        "Admin: Merge Registrations",
						PageTemplate: "admin_merge",
						AdminMerge:   mergeData,
					})
				}

				eligibleRows, err := loadMergeEligibleRegistrations(app, selectedQuizID)
				if err != nil {
					return echo.NewHTTPError(http.StatusInternalServerError, "failed to load merge candidates")
				}
				mergeData.EligibleRegistrations = eligibleRows

				if err := c.Request().ParseForm(); err != nil {
					return echo.NewHTTPError(http.StatusBadRequest, "invalid form payload")
				}
				mergeData.SelectedRegistrationIDs = c.Request().Form["registration_ids"]

				registrationIDs, err := normalizeRegistrationIDs(mergeData.SelectedRegistrationIDs)
				if err != nil {
					mergeData.Error = err.Error()
					return renderPage(c, PageData{
						Title:        "Admin: Merge Registrations",
						PageTemplate: "admin_merge",
						AdminMerge:   mergeData,
					})
				}

				eligibleIDs := make(map[string]struct{}, len(eligibleRows))
				for _, row := range eligibleRows {
					eligibleIDs[row.ID] = struct{}{}
				}
				for _, registrationID := range registrationIDs {
					if _, ok := eligibleIDs[registrationID]; !ok {
						mergeData.Error = "Please select only teams marked as merge candidates for this event."
						return renderPage(c, PageData{
							Title:        "Admin: Merge Registrations",
							PageTemplate: "admin_merge",
							AdminMerge:   mergeData,
						})
					}
				}

				formData := AdminMergeViewData{
					SelectedQuizID: selectedQuizID,
					TeamName:       teamName,
				}

				col, err := app.Dao().FindCollectionByNameOrId("registrations")
				if err != nil {
					return echo.NewHTTPError(http.StatusInternalServerError, "registrations collection is missing")
				}

				mergedRecord, err := mergeRegistrations(
					app.Dao(),
					col,
					registrationIDs,
					formData.TeamName,
					"",
					nil,
				)
				if err != nil {
					var reqErr mergeRequestError
					if errors.As(err, &reqErr) {
						mergeData.Error = reqErr.Error()
						return renderPage(c, PageData{
							Title:        "Admin: Merge Registrations",
							PageTemplate: "admin_merge",
							AdminMerge:   mergeData,
						})
					}
					return echo.NewHTTPError(http.StatusInternalServerError, "failed to merge registrations")
				}

				updatedEligibleRows, err := loadMergeEligibleRegistrations(app, selectedQuizID)
				if err != nil {
					return echo.NewHTTPError(http.StatusInternalServerError, "failed to refresh merge candidates")
				}

				return renderPage(c, PageData{
					Title:        "Admin: Merge Registrations",
					PageTemplate: "admin_merge",
					AdminMerge: AdminMergeViewData{
						Events:                events,
						SelectedQuizID:        selectedQuizID,
						EligibleRegistrations: updatedEligibleRows,
						TeamName:              "",
						Success:               "Registrations merged successfully.",
						MergedRecordID:        mergedRecord.Id,
						MergedQuizID:          mergedRecord.GetString("quiz"),
						MergedTeamName:        mergedRecord.GetString("team_name"),
						MergedTeamSize:        mergedRecord.GetInt("team_size"),
						MergedEmail:           mergedRecord.GetString("email"),
						MergedFromIDs:         registrationIDs,
					},
				})
			},
		})

		registerQuizAdminRoutes(e.Router, app)

		return nil
	})

	if err := app.Start(); err != nil {
		log.Fatal(err)
	}
}

func renderPage(c echo.Context, data PageData) error {
	var buf bytes.Buffer
	if err := tpl.ExecuteTemplate(&buf, "layout_start", data); err != nil {
		return err
	}
	if err := tpl.ExecuteTemplate(&buf, data.PageTemplate, data); err != nil {
		return err
	}
	if err := tpl.ExecuteTemplate(&buf, "layout_end", data); err != nil {
		return err
	}
	return c.HTMLBlob(http.StatusOK, buf.Bytes())
}

func renderRegisterPanel(c echo.Context, data RegisterPanelData, htmx bool) error {
	var buf bytes.Buffer
	if htmx {
		if err := tpl.ExecuteTemplate(&buf, "register_panel", data); err != nil {
			return err
		}
		return c.HTMLBlob(http.StatusOK, buf.Bytes())
	}
	pageData := PageData{
		Title:        "Quiz Registration",
		PageTemplate: "quiz",
		Quiz:         data.Quiz,
		Register:     data,
	}
	if err := tpl.ExecuteTemplate(&buf, "layout_start", pageData); err != nil {
		return err
	}
	if err := tpl.ExecuteTemplate(&buf, "quiz", pageData); err != nil {
		return err
	}
	if err := tpl.ExecuteTemplate(&buf, "layout_end", pageData); err != nil {
		return err
	}
	return c.HTMLBlob(http.StatusOK, buf.Bytes())
}

func loadUpcomingQuizzes(app core.App) ([]QuizCard, error) {
	recs, err := app.Dao().FindRecordsByFilter(
		"quiz_dates",
		"is_open = true",
		"scheduled_at",
		0,
		0,
	)
	if err != nil {
		return nil, err
	}

	now := time.Now()
	quizzes := make([]QuizCard, 0, len(recs))

	for _, r := range recs {
		when, err := parseDateTime(r.GetString("scheduled_at"))
		if err != nil {
			return nil, err
		}
		locID := r.GetString("location")
		loc, err := app.Dao().FindRecordById("locations", locID)
		if err != nil {
			return nil, err
		}

		capacity := loc.GetInt("capacity")
		registered, err := getRegisteredSeats(app, r.Id)
		if err != nil {
			return nil, err
		}

		if shouldAutoCloseQuiz(when, capacity, registered, now) {
			if r.GetBool("is_open") {
				if err := closeQuiz(app, r); err != nil {
					return nil, err
				}
			}
			continue
		}

		quizzes = append(quizzes, QuizCard{
			ID:             r.Id,
			When:           when,
			WhenLabel:      when.Format("Mon, 02 Jan 2006 15:04"),
			LocationName:   loc.GetString("name"),
			MapsURL:        loc.GetString("maps_url"),
			Capacity:       capacity,
			SeatsRemaining: max(capacity-registered, 0),
		})
	}

	sort.Slice(quizzes, func(i, j int) bool {
		return quizzes[i].When.Before(quizzes[j].When)
	})

	return quizzes, nil
}

func loadQuiz(app core.App, quizID string) (QuizCard, int, error) {
	quizRec, err := app.Dao().FindRecordById("quiz_dates", quizID)
	if err != nil {
		return QuizCard{}, 0, err
	}
	when, err := parseDateTime(quizRec.GetString("scheduled_at"))
	if err != nil {
		return QuizCard{}, 0, err
	}

	loc, err := app.Dao().FindRecordById("locations", quizRec.GetString("location"))
	if err != nil {
		return QuizCard{}, 0, err
	}

	capacity := loc.GetInt("capacity")
	registered, err := getRegisteredSeats(app, quizID)
	if err != nil {
		return QuizCard{}, 0, err
	}

	now := time.Now()
	if shouldAutoCloseQuiz(when, capacity, registered, now) {
		if quizRec.GetBool("is_open") {
			if err := closeQuiz(app, quizRec); err != nil {
				return QuizCard{}, 0, err
			}
		}
		return QuizCard{}, 0, fmt.Errorf("quiz is closed")
	}

	if !quizRec.GetBool("is_open") {
		return QuizCard{}, 0, fmt.Errorf("quiz is closed")
	}

	return QuizCard{
		ID:             quizRec.Id,
		When:           when,
		WhenLabel:      when.Format("Mon, 02 Jan 2006 15:04"),
		LocationName:   loc.GetString("name"),
		MapsURL:        loc.GetString("maps_url"),
		Capacity:       capacity,
		SeatsRemaining: max(capacity-registered, 0),
	}, max(capacity-registered, 0), nil
}

func getRegisteredSeats(app core.App, quizID string) (int, error) {
	recs, err := app.Dao().FindRecordsByFilter(
		"registrations",
		"quiz = {:quizId}",
		"",
		0,
		0,
		dbx.Params{"quizId": quizID},
	)
	if err != nil {
		return 0, err
	}

	total := 0
	for _, r := range recs {
		total += r.GetInt("team_size")
	}
	return total, nil
}

func parseDateTime(v string) (time.Time, error) {
	layouts := []string{
		time.RFC3339,
		"2006-01-02 15:04:05.000Z",
		"2006-01-02 15:04:05Z",
		"2006-01-02 15:04:05",
	}
	for _, layout := range layouts {
		parsed, err := time.Parse(layout, v)
		if err == nil {
			return parsed, nil
		}
	}
	return time.Time{}, fmt.Errorf("unsupported date format: %s", v)
}

func shouldAutoCloseQuiz(when time.Time, capacity, registered int, now time.Time) bool {
	if registered >= capacity {
		return true
	}
	return !when.After(now.Add(quizAutoCloseBeforeStart))
}

func closeQuiz(app core.App, quizRec *models.Record) error {
	quizRec.Set("is_open", false)
	return app.Dao().SaveRecord(quizRec)
}

func splitTeamSizes(total, maxPerTeam int) []int {
	teams := (total + maxPerTeam - 1) / maxPerTeam
	base := total / teams
	remainder := total % teams

	sizes := make([]int, teams)
	for i := 0; i < teams; i++ {
		sizes[i] = base
		if i < remainder {
			sizes[i]++
		}
	}
	return sizes
}

func joinIntSizes(values []int) string {
	if len(values) == 0 {
		return ""
	}
	parts := make([]string, len(values))
	for i, v := range values {
		parts[i] = strconv.Itoa(v)
	}
	return strings.Join(parts, ", ")
}

func parseMergeConsent(raw string) (bool, error) {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "yes":
		return true, nil
	case "no":
		return false, nil
	default:
		return false, fmt.Errorf("invalid merge consent value: %s", raw)
	}
}

func normalizeRegistrationIDs(ids []string) ([]string, error) {
	seen := make(map[string]struct{}, len(ids))
	normalized := make([]string, 0, len(ids))

	for _, id := range ids {
		trimmed := strings.TrimSpace(id)
		if trimmed == "" {
			continue
		}
		if _, exists := seen[trimmed]; exists {
			continue
		}
		seen[trimmed] = struct{}{}
		normalized = append(normalized, trimmed)
	}

	if len(normalized) < 2 {
		return nil, mergeRequestError{message: "at least two unique registration ids are required"}
	}

	return normalized, nil
}

func parseRegistrationIDsInput(raw string) ([]string, error) {
	fields := strings.FieldsFunc(raw, func(r rune) bool {
		return r == ',' || r == '\n' || r == '\r' || r == '\t' || r == ' '
	})

	return normalizeRegistrationIDs(fields)
}

func quizIDExists(quizzes []QuizCard, quizID string) bool {
	for _, quiz := range quizzes {
		if quiz.ID == quizID {
			return true
		}
	}
	return false
}

func loadMergeEligibleRegistrations(app core.App, quizID string) ([]AdminMergeRegistrationRow, error) {
	records, err := app.Dao().FindRecordsByFilter(
		"registrations",
		"quiz = {:quizId} && willing_to_merge = true",
		"team_name",
		0,
		0,
		dbx.Params{"quizId": quizID},
	)
	if err != nil {
		return nil, err
	}

	rows := make([]AdminMergeRegistrationRow, 0, len(records))
	for _, record := range records {
		rows = append(rows, AdminMergeRegistrationRow{
			ID:       record.Id,
			TeamName: record.GetString("team_name"),
			TeamSize: record.GetInt("team_size"),
			Email:    record.GetString("email"),
		})
	}
	return rows, nil
}

func parseOptionalMergeBool(raw string) (*bool, error) {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "", "auto":
		return nil, nil
	case "true":
		value := true
		return &value, nil
	case "false":
		value := false
		return &value, nil
	default:
		return nil, mergeRequestError{message: "willing_to_merge must be auto, true, or false"}
	}
}

func sanitizeAdminRedirect(raw string) string {
	redirectTo := strings.TrimSpace(raw)
	if redirectTo == "" {
		return "/admin/registrations/merge"
	}
	if !strings.HasPrefix(redirectTo, "/") || strings.HasPrefix(redirectTo, "//") {
		return "/admin/registrations/merge"
	}
	return redirectTo
}

func buildAdminSessionToken(app core.App, admin *models.Admin) (string, time.Duration, error) {
	signingSecret := app.Settings().AdminAuthToken.Secret
	if strings.TrimSpace(signingSecret) == "" {
		return "", 0, fmt.Errorf("admin auth token secret is empty")
	}

	durationSeconds := app.Settings().AdminAuthToken.Duration
	if durationSeconds <= 0 {
		durationSeconds = int64((12 * time.Hour).Seconds())
	}

	token, err := security.NewJWT(jwt.MapClaims{
		"id":   admin.Id,
		"type": "admin_merge_session",
	}, signingSecret, durationSeconds)
	if err != nil {
		return "", 0, err
	}

	return token, time.Duration(durationSeconds) * time.Second, nil
}

func currentAdminFromSession(app core.App, c echo.Context) (*models.Admin, error) {
	sessionCookie, err := c.Cookie(adminSessionCookieName)
	if err != nil {
		return nil, err
	}

	signingSecret := app.Settings().AdminAuthToken.Secret
	if strings.TrimSpace(signingSecret) == "" {
		return nil, fmt.Errorf("admin auth token secret is empty")
	}

	claims, err := security.ParseJWT(sessionCookie.Value, signingSecret)
	if err != nil {
		return nil, err
	}

	adminID, ok := claims["id"].(string)
	if !ok || strings.TrimSpace(adminID) == "" {
		return nil, fmt.Errorf("admin session token is missing id")
	}

	return app.Dao().FindAdminById(adminID)
}

func saveRegistrations(
	dao *daos.Dao,
	col *models.Collection,
	quizID, email string,
	sizes []int,
	teamNames []string,
	mergeConsent bool,
) error {
	return dao.RunInTransaction(func(txDao *daos.Dao) error {
		for i, size := range sizes {
			rec := models.NewRecord(col)
			rec.Set("quiz", quizID)
			rec.Set("email", email)
			rec.Set("team_size", size)
			rec.Set("team_name", teamNames[i])
			rec.Set("willing_to_merge", mergeConsent)

			if err := txDao.SaveRecord(rec); err != nil {
				return err
			}
		}
		return nil
	})
}

func mergeRegistrations(
	dao *daos.Dao,
	col *models.Collection,
	registrationIDs []string,
	teamName, email string,
	willingToMerge *bool,
) (*models.Record, error) {
	_ = willingToMerge

	records := make([]*models.Record, 0, len(registrationIDs))
	for _, registrationID := range registrationIDs {
		rec, err := dao.FindRecordById("registrations", registrationID)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return nil, mergeRequestError{message: fmt.Sprintf("registration not found: %s", registrationID)}
			}
			return nil, err
		}
		records = append(records, rec)
	}

	mergedQuizID := records[0].GetString("quiz")
	mergedTeamSize := 0
	teamNames := make([]string, 0, len(records))
	mergedWillingToMerge := false

	for _, rec := range records {
		quizID := rec.GetString("quiz")
		if quizID != mergedQuizID {
			return nil, mergeRequestError{message: "all registrations must belong to the same quiz"}
		}

		mergedTeamSize += rec.GetInt("team_size")
		teamNames = append(teamNames, rec.GetString("team_name"))
	}

	mergedTeamName := strings.TrimSpace(teamName)
	if mergedTeamName == "" {
		mergedTeamName = strings.Join(teamNames, " + ")
		if mergedTeamName == "" {
			mergedTeamName = "Merged Team"
		}
	}

	mergedEmail := strings.TrimSpace(email)
	if mergedEmail == "" {
		mergedEmail = records[0].GetString("email")
	}

	var mergedRecord *models.Record
	if err := dao.RunInTransaction(func(txDao *daos.Dao) error {
		mergedRecord = models.NewRecord(col)
		mergedRecord.Set("quiz", mergedQuizID)
		mergedRecord.Set("email", mergedEmail)
		mergedRecord.Set("team_name", mergedTeamName)
		mergedRecord.Set("team_size", mergedTeamSize)
		mergedRecord.Set("willing_to_merge", mergedWillingToMerge)

		if err := txDao.SaveRecord(mergedRecord); err != nil {
			return err
		}

		for _, rec := range records {
			if err := txDao.DeleteRecord(rec); err != nil {
				return err
			}
		}

		return nil
	}); err != nil {
		return nil, err
	}

	return mergedRecord, nil
}

func defaultSplitTeamNames(base string, count int) []string {
	names := make([]string, count)
	for i := 0; i < count; i++ {
		names[i] = fmt.Sprintf("%s (%d/%d)", base, i+1, count)
	}
	return names
}

func submittedSplitTeamNames(c echo.Context, count int) []string {
	names := make([]string, count)
	for i := 0; i < count; i++ {
		names[i] = strings.TrimSpace(c.FormValue(fmt.Sprintf("split_team_name_%d", i)))
	}
	return names
}

func hasEmptyValues(values []string) bool {
	for _, value := range values {
		if value == "" {
			return true
		}
	}
	return false
}

func loadRegistrationForUnregister(app core.App, registrationID string) (UnregisterViewData, error) {
	rec, err := app.Dao().FindRecordById("registrations", registrationID)
	if err != nil {
		return UnregisterViewData{}, err
	}

	return UnregisterViewData{
		RegistrationID: rec.Id,
		TeamName:       rec.GetString("team_name"),
		TeamSize:       rec.GetInt("team_size"),
		Email:          rec.GetString("email"),
	}, nil
}
