package logger

import (
	loggergo "github.com/Alonza0314/logger-go/v2"
	loggergoModel "github.com/Alonza0314/logger-go/v2/model"
	loggergoUtil "github.com/Alonza0314/logger-go/v2/util"
)

type BackendLogger struct {
	*loggergo.Logger

	CfgLog  loggergoModel.LoggerInterface
	AccLog  loggergoModel.LoggerInterface
	BckLog  loggergoModel.LoggerInterface
	ProcLog loggergoModel.LoggerInterface
	GinLog  loggergoModel.LoggerInterface
	CtxLog  loggergoModel.LoggerInterface
	DbLog   loggergoModel.LoggerInterface
}

func NewBackendLogger(level loggergoUtil.LogLevelString, filePath string, debugMode bool) *BackendLogger {
	logger := loggergo.NewLogger(filePath, debugMode)
	logger.SetLevel(level)

	return &BackendLogger{
		Logger: logger,

		CfgLog:  logger.WithTags(CFG_LOG),
		AccLog:  logger.WithTags(ACC_LOG),
		BckLog:  logger.WithTags(BCK_LOG),
		ProcLog: logger.WithTags(PROC_LOG),
		GinLog:  logger.WithTags(API_LOG),
		CtxLog:  logger.WithTags(CTX_LOG),
		DbLog:   logger.WithTags(DB_LOG),
	}
}
