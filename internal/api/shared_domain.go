package api

import (
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/pocketbase/pocketbase/daos"
	"github.com/pocketbase/pocketbase/models"
)

type MergeRequestError struct {
	Message string
}

func (e MergeRequestError) Error() string {
	return e.Message
}

func ParseDateTime(v string) (time.Time, error) {
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

func NormalizeRegistrationIDs(ids []string) ([]string, error) {
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
		return nil, MergeRequestError{Message: "at least two unique registration ids are required"}
	}
	return normalized, nil
}

func ParseRegistrationIDsInput(raw string) ([]string, error) {
	fields := strings.FieldsFunc(raw, func(r rune) bool {
		return r == ',' || r == '\n' || r == '\r' || r == '\t' || r == ' '
	})
	return NormalizeRegistrationIDs(fields)
}

func SaveRegistrations(
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

func MergeRegistrations(
	dao *daos.Dao,
	col *models.Collection,
	registrationIDs []string,
	teamName, email string,
) (*models.Record, error) {
	records := make([]*models.Record, 0, len(registrationIDs))
	for _, registrationID := range registrationIDs {
		rec, err := dao.FindRecordById("registrations", registrationID)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return nil, MergeRequestError{Message: fmt.Sprintf("registration not found: %s", registrationID)}
			}
			return nil, err
		}
		records = append(records, rec)
	}

	mergedQuizID := records[0].GetString("quiz")
	mergedTeamSize := 0
	teamNames := make([]string, 0, len(records))

	for _, rec := range records {
		quizID := rec.GetString("quiz")
		if strings.TrimSpace(quizID) == "" {
			return nil, MergeRequestError{Message: "all registrations must have a quiz id"}
		}
		if quizID != mergedQuizID {
			return nil, MergeRequestError{Message: "all registrations must belong to the same quiz"}
		}
		if !rec.GetBool("willing_to_merge") {
			return nil, MergeRequestError{Message: "all selected registrations must have willing_to_merge set to true"}
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
		mergedRecord.Set("willing_to_merge", false)
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
