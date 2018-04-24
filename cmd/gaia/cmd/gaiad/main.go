package main

import (
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	abci "github.com/tendermint/abci/types"
	"github.com/tendermint/tmlibs/cli"
	dbm "github.com/tendermint/tmlibs/db"
	"github.com/tendermint/tmlibs/log"

	"github.com/cosmos/cosmos-sdk/cmd/gaia/app"
	"github.com/cosmos/cosmos-sdk/server"
	stake "github.com/cosmos/cosmos-sdk/x/stake"
	stakecmd "github.com/cosmos/cosmos-sdk/x/stake/commands"
)

// rootCmd is the entry point for this binary
var (
	context = server.NewDefaultContext()
	rootCmd = &cobra.Command{
		Use:               "gaiad",
		Short:             "Gaia Daemon (server)",
		PersistentPreRunE: server.PersistentPreRunEFn(context),
	}
)

func generateApp(rootDir string, logger log.Logger) (abci.Application, error) {
	dataDir := filepath.Join(rootDir, "data")
	db, err := dbm.NewGoLevelDB("gaia", dataDir)
	if err != nil {
		return nil, err
	}
	bapp := app.NewGaiaApp(logger, db)
	return bapp, nil
}

func main() {
	server.AddCommands(rootCmd, app.DefaultGenAppState, generateApp, context)

	rootDir := os.ExpandEnv("$HOME/.gaiad")

	export := func() (stake.GenesisState, error) {
		dataDir := filepath.Join(rootDir, "data")
		db, err := dbm.NewGoLevelDB("gaia", dataDir)
		if err != nil {
			return stake.GenesisState{}, err
		}
		app := app.NewGaiaApp(log.NewNopLogger(), db)
		if err != nil {
			return stake.GenesisState{}, err
		}
		return app.ExportStake(), nil
	}

	rootCmd.AddCommand(stakecmd.GetCmdExport(export, app.MakeCodec()))

	// prepare and add flags
	executor := cli.PrepareBaseCmd(rootCmd, "GA", rootDir)
	executor.Execute()
}
