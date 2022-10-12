package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/BurntSushi/toml"
	"github.com/filecoin-project/go-jsonrpc"
	"github.com/filecoin-project/lotus/api"
	"github.com/prometheus/common/expfmt"
)

func main() {
	port, err := portNumber()
	if err != nil {
		fatal("Unable to find port number: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Second*10)
	defer cancel()

	lotus, closer, err := lotusApi(ctx, port)
	if err != nil {
		fatal("Failed to create API: %v", err)
	}
	defer closer()

	// Wait for `lotus daemon` to start
	if err := checkDaemonRunning(ctx, lotus); err != nil {
		fatal("lotus daemon not running: %v", err)
	}

	// Wait for `lotus-miner run` to finish starting
	if err := checkMinerRunning(ctx, port); err != nil {
		fatal("lotus-miner not running: %v", err)
	}
}

func fatal(format string, v ...any) {
	fmt.Printf(format, v...)
	os.Exit(1)
}

func portNumber() (int, error) {
	base, present := os.LookupEnv("LOTUS_PATH")
	if !present {
		return 0, fmt.Errorf("missing LOTUS_PATH environment variable")
	}

	f, err := os.ReadFile(filepath.Join(base, "config.toml"))
	if err != nil {
		return 0, err
	}

	var cfg struct {
		API struct {
			ListenAddress string
		}
	}
	if err := toml.Unmarshal(f, &cfg); err != nil {
		return 0, err
	}

	parts := strings.Split(cfg.API.ListenAddress, "/")
	port, err := strconv.Atoi(parts[4])
	if err != nil {
		return 0, err
	}

	return port, nil
}

func lotusApi(ctx context.Context, port int) (api.FullNodeStruct, func(), error) {
	addr := fmt.Sprintf("ws://localhost:%d/rpc/v0", port)

	var lotus api.FullNodeStruct
	closer, err := jsonrpc.NewMergeClient(ctx, addr, "Filecoin", []interface{}{&lotus.Internal, &lotus.CommonStruct.Internal}, nil)
	if err != nil {
		return api.FullNodeStruct{}, nil, err
	}
	return lotus, closer, nil
}

func checkDaemonRunning(ctx context.Context, lotus api.FullNodeStruct) error {
	state, err := lotus.SyncState(ctx)
	if err != nil {
		return err
	}

	for _, sync := range state.ActiveSyncs {
		if sync.Stage != api.StageIdle {
			return fmt.Errorf("sync %v is in stage %v rather than %v", sync.WorkerID, sync.Stage, api.StageIdle)
		}
	}
	return nil
}

func checkMinerRunning(ctx context.Context, port int) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, fmt.Sprintf("http://localhost:%d/debug/metrics", port), nil)
	if err != nil {
		return err
	}

	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}

	defer func() {
		if err := res.Body.Close(); err != nil {
			log.Printf("Failed to close body: %v", err)
		}
	}()

	var parser expfmt.TextParser
	mf, err := parser.TextToMetricFamilies(res.Body)
	if err != nil {
		return err
	}

	if _, ok := mf["lotus_chain_node_worker_height"]; !ok {
		return fmt.Errorf("missing miner metrics")
	}

	return nil
}
