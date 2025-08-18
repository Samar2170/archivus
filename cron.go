package main

import (
	"archivus/config"
	"archivus/internal/helpers"
	"archivus/internal/service/image"
	"archivus/pkg/logging"
	"time"

	"github.com/go-co-op/gocron"
)

func startCronServer() {
	t := time.Now()
	logging.AuditLogger.Info().Msgf("Starting cron server at %s", t.Format(time.RFC3339))
	s := gocron.NewScheduler(time.UTC)
	s.Every(2).Hour().Do(func() {
		err := image.MarkImages()
		if err != nil {
			logging.Errorlogger.Error().Msgf("Error in Marking images: %s", err.Error())
		}
	})
	s.Every(1).Hour().Do(func() {
		err := image.CompressImages(config.CompressionQuality)
		if err != nil {
			logging.Errorlogger.Error().Msgf("Error in Compressing images: %s", err.Error())
		}
	})
	s.Every(1).Hour().Do(func() {
		helpers.UpdateDirsData()
		helpers.UpdateUserDirsData()
	})

	s.StartBlocking()
}
