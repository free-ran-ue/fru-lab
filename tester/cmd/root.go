package cmd

import (
	"context"
	"errors"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	loggergo "github.com/Alonza0314/logger-go/v2"
	loggergoUtil "github.com/Alonza0314/logger-go/v2/util"
	"github.com/spf13/cobra"

	"tester/api"
	"tester/config"
	"tester/gnb"
	"tester/netcfg"
	"tester/run"
)

var rootCmd = &cobra.Command{
	Use:   "fru-tester",
	Short: "5G throughput tester engine (driven by fru-lab)",
	RunE:  serve,
}

func init() {
	rootCmd.Flags().StringP("config", "c", "tester.yaml", "path to fru-tester config")
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

func serve(cmd *cobra.Command, _ []string) error {
	path, _ := cmd.Flags().GetString("config")
	cfg, err := config.Load(path)
	if err != nil {
		return err
	}

	lg := loggergo.NewLogger("", true)
	lg.SetLevel(loggergoUtil.LogLevelString(cfg.Logger.Level))
	mainLog := lg.WithTags("TESTER")

	ctrl := run.NewController(run.Deps{
		Addrs:  netcfg.Netlink{},
		Dialer: gnb.SCTPDialer{},
		Log:    lg.WithTags("RUN"),
	})
	srv := &http.Server{Addr: cfg.Listen, Handler: api.NewRouter(ctrl, cfg.ApiToken)}

	errCh := make(chan error, 1)
	go func() { errCh <- srv.ListenAndServe() }()
	mainLog.Infof("fru-tester listening on %s", cfg.Listen)

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, os.Interrupt, syscall.SIGTERM)
	select {
	case err := <-errCh:
		if !errors.Is(err, http.ErrServerClosed) {
			return err
		}
	case <-sig:
	}

	mainLog.Infoln("shutting down: stopping active run and removing gNB IPs")
	ctrl.Shutdown()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	return srv.Shutdown(ctx)
}
