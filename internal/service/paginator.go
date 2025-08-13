package service

import (
	"archivus/pkg/logging"
	"fmt"
	"strconv"

	"gorm.io/gorm"
)

type PaginatedResults struct {
	Results    interface{} `json:"data"`
	Page       int         `json:"page"`
	PageSize   int         `json:"pageSize"`
	TotalCount int         `json:"totalCount"`
	TotalPages int         `json:"pages"`
}

// not working

func paginateResults(query *gorm.DB, pageNo string, pageSize int, dest interface{}) (PaginatedResults, error) {
	var totalPages int
	var errorMsg string
	var totalCount int64
	err := query.Count(&totalCount).Error
	if err != nil {
		logging.Errorlogger.Error().Msgf("Error counting total results: %s", err.Error())
		return PaginatedResults{}, fmt.Errorf("error counting total results: %w", err)
	}
	if int(totalCount)%pageSize == 0 {
		totalPages = int(totalCount) / pageSize
	} else {
		totalPages = int(totalCount)/pageSize + 1
	}
	page, err := strconv.Atoi(pageNo)
	if err != nil || page < 1 {
		page = 1
		errorMsg = ";Invalid page number, defaulting to page 1"
	}
	offset := (page - 1) * pageSize
	if offset >= int(totalCount) {
		errorMsg += ";Page number exceeds total pages"
		offset = 0
	}
	err = query.Offset(offset).Limit(pageSize).Find(&dest).Error
	if err != nil {
		logging.Errorlogger.Error().Msgf("Error fetching paginated results: %s", err.Error())
		return PaginatedResults{}, fmt.Errorf("error fetching paginated results: %w", err)
	}
	paginatedResults := PaginatedResults{
		Results:    dest,
		Page:       page,
		PageSize:   pageSize,
		TotalCount: int(totalCount),
		TotalPages: totalPages,
	}
	if errorMsg != "" {
		logging.Errorlogger.Error().Msg(errorMsg)
		return paginatedResults, fmt.Errorf("error in pagination: %s", errorMsg)
	}
	return paginatedResults, nil
}
