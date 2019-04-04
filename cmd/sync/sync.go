package main

import (
	"fmt"
	cmdp "github.com/spacemeshos/go-spacemesh/cmd"
	"github.com/spacemeshos/go-spacemesh/database"
	"github.com/spacemeshos/go-spacemesh/log"
	"github.com/spacemeshos/go-spacemesh/mesh"
	"github.com/spacemeshos/go-spacemesh/p2p"
	"github.com/spacemeshos/go-spacemesh/sync"
	"github.com/spacemeshos/go-spacemesh/timesync"
	"github.com/spf13/cobra"
	"os"
	"time"
)

var lg = log.New("sync_test", "", "")

// Sync cmd
var Cmd = &cobra.Command{
	Use:   "sync",
	Short: "start sync",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("Starting sync")
		syncApp := NewSyncApp()
		defer syncApp.Cleanup()
		syncApp.Initialize(cmd)
		syncApp.Start(cmd, args)
	},
}

func init() {
	cmdp.AddCommands(Cmd)
}

type SyncApp struct {
	*cmdp.BaseApp
	sync *sync.Syncer
}

func NewSyncApp() *SyncApp {
	return &SyncApp{BaseApp: cmdp.NewBaseApp()}
}

func (app *SyncApp) Cleanup() {

}

func getMesh(path string) *mesh.Mesh {
	//"tests/sync/data"
	db := database.NewLevelDbStore(path, nil, nil)
	mdb := mesh.NewMeshDB(db, db, db, db, lg.WithName("meshDb"))
	layers := mesh.NewMesh(mdb, sync.ConfigTst(), &sync.MeshValidatorMock{}, sync.MockState{}, lg)
	return layers
}

func (app *SyncApp) Start(cmd *cobra.Command, args []string) {
	// start p2p services
	lg.Info("Initializing P2P services")
	swarm, err := p2p.New(cmdp.Ctx, app.Config.P2P)

	if err != nil {
		panic("something got fudged while creating p2p service ")
	}

	conf := sync.Configuration{SyncInterval: 1 * time.Second, Concurrency: 4, LayerSize: int(5), RequestTimeout: 100 * time.Millisecond}
	gTime, err := time.Parse(time.RFC3339, app.Config.GenesisTime)
	ld := time.Duration(app.Config.LayerDurationSec) * time.Second
	clock := timesync.NewTicker(timesync.RealClock{}, ld, gTime)
	msh := getMesh(app.Config.DataDir)
	defer msh.Close()
	if lyr, err := msh.GetLayer(100); err != nil || lyr == nil {
		lg.Error("could not load layers from disk ...   shutdown", err)
		return
	}
	lg.Info("woke up with %v layers in meshDB ", 100)
	app.sync = sync.NewSync(swarm, msh, sync.BlockValidatorMock{}, conf, clock.Subscribe(), lg)
	if err != nil {
		log.Error("Error starting p2p services, err: %v", err)
		panic("Error starting p2p services")
	}
}

func main() {
	if err := Cmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

}