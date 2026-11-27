package context

import (
	"context"
	"time"

	"backend/logger"
)

type FlCotextIE struct {
	DbType string
	DbPath string

	DeployWorkDir  string
	WebconsolePort int
	DeployTimeout  time.Duration

	*logger.BackendLogger
}

type FlContext struct {
	*dbContext
	*composeContext
	*ipamContext

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

	ipam := newIpamContext(dbContext.db, dbContext.ListCoreNetworks, ie.BackendLogger)

	composeContext, err := newComposeContext(&composeContextIE{
		WorkDir:        ie.DeployWorkDir,
		WebconsolePort: ie.WebconsolePort,
		DeployTimeout:  ie.DeployTimeout,
		Ipam:           ipam,

		BackendLogger: ie.BackendLogger,
	})
	if err != nil {
		ie.BackendLogger.CtxLog.Errorf("Failed to create composeContext: %v", err)
		return nil
	}

	// the addresses gNBs deployed before fru-lab had IPAM hold become
	// their leases; then what Docker holds is checked against the plan
	if err := ipam.adopt(composeContext.gnbLeaseRecords()); err != nil {
		ie.BackendLogger.CtxLog.Warnf("IPAM: failed to adopt the deployed gNBs' addresses: %v", err)
	}
	if st, err := ipam.reconcile(context.Background()); err != nil {
		ie.BackendLogger.CtxLog.Warnf("IPAM: reconcile failed: %v", err)
	} else {
		for _, w := range st.warnings {
			ie.BackendLogger.CtxLog.Warnf("IPAM: %s", w)
		}
	}

	return &FlContext{
		dbContext:      dbContext,
		composeContext: composeContext,
		ipamContext:    ipam,

		BackendLogger: ie.BackendLogger,
	}
}

func (ctx *FlContext) Release() {
	ctx.CtxLog.Infoln("Release FlContext...")

	// compose first: tearing the lab down reads the address plan (the saved
	// core networks) and leases from the database, so it must still be open
	ctx.composeContext.release()
	ctx.dbContext.release()

	ctx.CtxLog.Infoln("FlContext released")
}
