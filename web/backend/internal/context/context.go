package context

import "backend/logger"

type FlCotextIE struct {
	DbType string
	DbPath string

	DeployWorkDir string

	*logger.BackendLogger
}

type FlContext struct {
	*dbContext
	*composeContext

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

	composeContext, err := newComposeContext(&composeContextIE{
		WorkDir: ie.DeployWorkDir,

		BackendLogger: ie.BackendLogger,
	})
	if err != nil {
		ie.BackendLogger.CtxLog.Errorf("Failed to create composeContext: %v", err)
		return nil
	}

	return &FlContext{
		dbContext:      dbContext,
		composeContext: composeContext,

		BackendLogger: ie.BackendLogger,
	}
}

func (ctx *FlContext) Release() {
	ctx.CtxLog.Infoln("Release FlContext...")

	ctx.dbContext.release()
	ctx.composeContext.release()

	ctx.CtxLog.Infoln("FlContext released")
}
