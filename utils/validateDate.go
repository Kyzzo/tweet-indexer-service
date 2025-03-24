package utils

import (
	"fmt"
	"time"
)

func ValidateDate(targetDate string) (time.Time, error) {
	formats := []string{
		"2006-01-02",
		"01/02/2006",
		"02-01-2006",
		"2006/01/02",
	}

	var parsedTime time.Time
	var err error

	for _, format := range formats {
		parsedTime, err = time.Parse(format, targetDate)
		if err == nil {
			if parsedTime.After(time.Now()) {
				return time.Time{}, fmt.Errorf("target date cannot be in the future")
			}
			return parsedTime, nil
		}
	}
	
	return time.Time{}, fmt.Errorf("invalid date format: please use YYYY-MM-DD, MM/DD/YYYY, DD-MM-YYYY, or YYYY/MM/DD")
}
