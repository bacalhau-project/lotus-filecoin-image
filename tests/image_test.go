package tests

import (
	"context"
	"fmt"
	"log"
	"math/rand"
	"net/http"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/filecoin-project/go-fil-markets/storagemarket"
	"github.com/filecoin-project/go-jsonrpc"
	"github.com/filecoin-project/lotus/build"
	"github.com/filecoin-project/lotus/chain/types"
	"github.com/gruntwork-io/terratest/modules/docker"
	"github.com/gruntwork-io/terratest/modules/retry"
	"github.com/gruntwork-io/terratest/modules/shell"
	"github.com/stretchr/testify/require"

	lotusapi "github.com/filecoin-project/lotus/api"
)

func TestImage(t *testing.T) {
	img := os.Getenv("TEST_IMAGE")
	require.NotEmpty(t, img)

	dir := t.TempDir()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	containerTestDir := "/tmp/testdata"
	testFile := "temp.bin"
	testData := make([]byte, 1024)
	rand.Read(testData)
	require.NoError(t, os.WriteFile(filepath.Join(dir, testFile), testData, 0644))

	containerId := docker.RunAndGetID(t, img, &docker.RunOptions{
		Detach:       true,
		Volumes:      []string{fmt.Sprintf("%s:%s", dir, containerTestDir)},
		OtherOptions: []string{"--publish=1234"},
	})

	t.Cleanup(func() {
		docker.Stop(t, []string{containerId}, &docker.StopOptions{})
	})

	waitUntilHealthy(t, containerId)

	tokenFile := filepath.Join(dir, "token")

	shell.RunCommand(t, shell.Command{
		Command:    "docker",
		Args:       []string{"container", "cp", fmt.Sprintf("%s:/home/lotus_user/.lotus-local-net/token", containerId), tokenFile},
		WorkingDir: dir,
	})

	container := docker.Inspect(t, containerId)
	apiPort := container.GetExposedHostPort(1234)
	require.NotEqual(t, 0, apiPort)

	api := lotusApi(t, ctx, apiPort, tokenFile)

	miners, err := api.StateListMiners(ctx, types.NewTipSetKey())
	require.NoError(t, err)
	require.Len(t, miners, 1)

	wallets, err := api.WalletList(ctx)
	require.NoError(t, err)
	require.Len(t, wallets, 1)
	t.Log(wallets)

	imported, err := api.ClientImport(ctx, lotusapi.FileRef{IsCAR: false, Path: fmt.Sprintf("%s/%s", containerTestDir, testFile)})
	require.NoError(t, err)

	deal := &lotusapi.StartDealParams{
		Data: &storagemarket.DataRef{
			TransferType: storagemarket.TTGraphsync,
			Root:         imported.Root,
		},
		Wallet:            wallets[0],
		Miner:             miners[0],
		MinBlocksDuration: uint64(build.MinDealDuration), // This seems to equal 180 days rather than 24 that you can enter via the command line?
		EpochPrice:        types.NewInt(1000),
	}

	cid, err := api.ClientStartDeal(ctx, deal)
	require.NoError(t, err)

	// Something isn't completely right with this test - occasionally the deal gets stuck in `StorageDealFundsReserved`...
	retry.DoWithRetry(t, "waiting for deal acceptance", 50, 5*time.Second, func() (string, error) {
		deal, err := api.ClientGetDealInfo(ctx, *cid)
		if err != nil {
			return "", err
		}

		if deal.State == storagemarket.StorageDealCheckForAcceptance {
			return "accepted", nil
		}

		return "", fmt.Errorf("deal %v is in state %v", cid.String(), storagemarket.DealStates[deal.State])
	})
}

func waitUntilHealthy(t *testing.T, containerId string) {
	retry.DoWithRetry(t, "waiting for sync", 50, 5*time.Second, func() (string, error) {
		container, err := docker.InspectE(t, containerId)
		if err != nil {
			return "", err
		}

		if container.Health.Status == "healthy" {
			t.Logf("Container %s is healthy", containerId)
			return container.Health.Status, nil
		}

		return "", fmt.Errorf("health status: %s, %#v", container.Health.Status, container.Health.Log)
	})
}

func lotusApi(t *testing.T, ctx context.Context, port uint16, tokenFile string) lotusapi.FullNodeStruct {
	token, err := os.ReadFile(tokenFile)
	require.NoError(t, err)

	headers := http.Header{"Authorization": []string{"Bearer " + string(token)}}
	addr := fmt.Sprintf("ws://localhost:%d/rpc/v0", port)

	var api lotusapi.FullNodeStruct

	closer, err := jsonrpc.NewMergeClient(ctx, addr, "Filecoin", []interface{}{&api.Internal, &api.CommonStruct.Internal}, headers)
	require.NoError(t, err)
	if err != nil {
		log.Fatalf("connecting with lotus failed: %s", err)
	}
	t.Cleanup(func() {
		closer()
	})
	return api
}
