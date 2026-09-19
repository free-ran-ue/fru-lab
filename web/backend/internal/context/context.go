package context

import "backend/logger"

type FlCotextIE struct {
	DbType string
	DbPath string

	*logger.BackendLogger
}

type FlContext struct {
	*dbContext

	*logger.BackendLogger
}

func NewFlContext(ie *FlCotextIE) *FlContext {
	dbContext, err := newDbContext(&dbContextIE{
		dbType: ie.DbType,
		dbPath: ie.DbPath,

		BackendLogger: ie.BackendLogger,
	})
	if err != nil {
		ie.BackendLogger.CtxLog.Errorf("Failed to create dbContext: %v", err)
		return nil
	}

	return &FlContext{
		dbContext: dbContext,

		BackendLogger: ie.BackendLogger,
	}
}

func (ctx *FlContext) Release() {
	ctx.CtxLog.Infoln("Release FlContext...")

	ctx.dbContext.release()

	ctx.CtxLog.Infoln("FlContext released")
}
