package api

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/labstack/echo/v5"
	"github.com/pocketbase/dbx"
	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/models"
)

const (
	QuizAutoCloseBeforeStart = time.Hour
	MaxTeamSize              = 10
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

type PageData struct {
	Title               string
	PageTemplate        string
	StatusCode          int
	IsQuizAdmin         bool
	ShowQuizAdminLogout bool
	SetupError          string
	Quizzes             []QuizCard
	Quiz                QuizCard
	Register            RegisterPanelData
	Unregister          UnregisterViewData
	Admin               any
}

func LoadUpcomingQuizzes(app core.App) ([]QuizCard, error) {
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
		when, err := ParseDateTime(r.GetString("scheduled_at"))
		if err != nil {
			return nil, err
		}
		locID := r.GetString("location")
		loc, err := app.Dao().FindRecordById("locations", locID)
		if err != nil {
			return nil, err
		}

		capacity := loc.GetInt("capacity")
		registered, err := GetRegisteredSeats(app, r.Id)
		if err != nil {
			return nil, err
		}

		if ShouldAutoCloseQuiz(when, capacity, registered, now) {
			if r.GetBool("is_open") {
				if err := CloseQuiz(app, r); err != nil {
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

func LoadQuiz(app core.App, quizID string) (QuizCard, int, error) {
	quizRec, err := app.Dao().FindRecordById("quiz_dates", quizID)
	if err != nil {
		return QuizCard{}, 0, err
	}
	when, err := ParseDateTime(quizRec.GetString("scheduled_at"))
	if err != nil {
		return QuizCard{}, 0, err
	}

	loc, err := app.Dao().FindRecordById("locations", quizRec.GetString("location"))
	if err != nil {
		return QuizCard{}, 0, err
	}

	capacity := loc.GetInt("capacity")
	registered, err := GetRegisteredSeats(app, quizID)
	if err != nil {
		return QuizCard{}, 0, err
	}

	now := time.Now()
	if ShouldAutoCloseQuiz(when, capacity, registered, now) {
		if quizRec.GetBool("is_open") {
			if err := CloseQuiz(app, quizRec); err != nil {
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

func GetRegisteredSeats(app core.App, quizID string) (int, error) {
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

func ShouldAutoCloseQuiz(when time.Time, capacity, registered int, now time.Time) bool {
	if registered >= capacity {
		return true
	}
	return !when.After(now.Add(QuizAutoCloseBeforeStart))
}

func CloseQuiz(app core.App, quizRec *models.Record) error {
	quizRec.Set("is_open", false)
	return app.Dao().SaveRecord(quizRec)
}

func SplitTeamSizes(total, maxPerTeam int) []int {
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

func JoinIntSizes(values []int) string {
	if len(values) == 0 {
		return ""
	}
	parts := make([]string, len(values))
	for i, v := range values {
		parts[i] = strconv.Itoa(v)
	}
	return strings.Join(parts, ", ")
}

func ParseMergeConsent(raw string) (bool, error) {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "yes":
		return true, nil
	case "no":
		return false, nil
	default:
		return false, fmt.Errorf("invalid merge consent value: %s", raw)
	}
}

func DefaultSplitTeamNames(base string, count int) []string {
	names := make([]string, count)
	for i := 0; i < count; i++ {
		names[i] = fmt.Sprintf("%s (%d/%d)", base, i+1, count)
	}
	return names
}

func SubmittedSplitTeamNames(c echo.Context, count int) []string {
	names := make([]string, count)
	for i := 0; i < count; i++ {
		names[i] = strings.TrimSpace(c.FormValue(fmt.Sprintf("split_team_name_%d", i)))
	}
	return names
}

func HasEmptyValues(values []string) bool {
	for _, value := range values {
		if value == "" {
			return true
		}
	}
	return false
}

func LoadRegistrationForUnregister(app core.App, registrationID string) (UnregisterViewData, error) {
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
