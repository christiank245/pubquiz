package main

import (
	"bytes"
	"embed"
	"fmt"
	"html/template"
	"io/fs"
	"log"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/labstack/echo/v5"
	"github.com/pocketbase/dbx"
	"github.com/pocketbase/pocketbase"
	"github.com/pocketbase/pocketbase/apis"
	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/daos"
	"github.com/pocketbase/pocketbase/models"
	"github.com/pocketbase/pocketbase/plugins/migratecmd"

	_ "pubquiz-website2.0/migrations"
)

//go:embed web/templates web/public
var webFiles embed.FS

var tpl = template.Must(template.New("site").Funcs(template.FuncMap{
	"add1": func(i int) int { return i + 1 },
}).ParseFS(
	webFiles,
	"web/templates/*.html",
	"web/templates/components/*.html",
	"web/templates/pages/*.html",
))

const (
	quizAutoCloseBeforeStart = time.Hour
	maxTeamSize              = 10
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

type PageData struct {
	Title        string
	PageTemplate string
	SetupError   string
	Quizzes      []QuizCard
	Quiz         QuizCard
	Register     RegisterPanelData
	Unregister   UnregisterViewData
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

				if err := saveRegistrations(app.Dao(), col, quizID, email, registrationSizes, registrationTeamNames); err != nil {
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

func saveRegistrations(dao *daos.Dao, col *models.Collection, quizID, email string, sizes []int, teamNames []string) error {
	return dao.RunInTransaction(func(txDao *daos.Dao) error {
		for i, size := range sizes {
			rec := models.NewRecord(col)
			rec.Set("quiz", quizID)
			rec.Set("email", email)
			rec.Set("team_size", size)
			rec.Set("team_name", teamNames[i])

			if err := txDao.SaveRecord(rec); err != nil {
				return err
			}
		}
		return nil
	})
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
