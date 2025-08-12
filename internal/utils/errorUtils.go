package utils

import (
	"archivus/pkg/logging"
)

func HandleError(prefix, message string, err error) error {
	if err != nil {
		logging.Errorlogger.Printf("%s: %s: %v", prefix, message, err)
		return err
	}
	return nil
}

func LogError(prefix, message string, err error) {
	if err != nil {
		logging.Errorlogger.Printf("%s: %s: %v", prefix, message, err)
	}
}
