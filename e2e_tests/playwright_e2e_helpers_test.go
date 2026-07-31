package e2e_tests

import (
	"fmt"
	"strings"

	"github.com/mxschmitt/playwright-go"
)

func submitRegistrationIgnoringValidation(page playwright.Page) error {
	_, err := page.Evaluate(`() => {
		const form = document.querySelector('#registration-panel form');
		if (!form) {
			throw new Error('registration form not found');
		}
		form.noValidate = true;
		const button = form.querySelector('button[type="submit"]');
		if (!button) {
			throw new Error('registration submit button not found');
		}
		button.click();
	}`)
	return err
}

func recordIDByFieldValue(records []map[string]any, field, value string) (string, error) {
	for _, record := range records {
		rawValue, ok := record[field]
		if !ok {
			continue
		}
		if strings.TrimSpace(fmt.Sprint(rawValue)) == value {
			id, ok := record["id"].(string)
			if !ok || strings.TrimSpace(id) == "" {
				return "", fmt.Errorf("record matched %s=%q but had no id", field, value)
			}
			return id, nil
		}
	}
	return "", fmt.Errorf("record with %s=%q not found", field, value)
}
