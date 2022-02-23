package main

import (
	"context"
	"errors"
	"flag"
	"log"
	"math/big"
	"net/http"

	"github.com/ArmanMazdaee/yaegpe/gasprice"
	"github.com/ArmanMazdaee/yaegpe/handler"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/ethereum/go-ethereum/rpc"
)

const sampleSize = 7
const estimatorHistory = 5
const estimatorSkip = 2

var estimatorTarget = []gasprice.Target{
	{Start: 0, End: 0.3},
	{Start: 0.3, End: 0.6},
	{Start: 0.6, End: 1},
}
var sampleMinPrice = big.NewInt(1e8)

func main() {
	providerURL := flag.String("provider", "", "ethereum provider url")
	addr := flag.String("addr", "0.0.0.0:8080", "server address")
	flag.Parse()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	provider, err := ethclient.Dial(*providerURL)
	if err != nil {
		log.Fatalln("could not connect to the provider: ", err)
	}
	defer provider.Close()

	var tracker gasprice.Tracker
	tracker, err = gasprice.NewSubscribedTracker(ctx, provider)
	if errors.Is(err, rpc.ErrNotificationsUnsupported) {
		log.Println("fallback to the polling tracker:", err)
		tracker = gasprice.NewPollingTracker(ctx, provider)
	} else if err != nil {
		log.Fatalln("could not create tracker:", err)
	}

	sampler, err := gasprice.NewMinimumSampler(provider, sampleSize, sampleMinPrice)
	if err != nil {
		log.Fatalln("could not create sampler:", err)
	}

	estimator, err := gasprice.NewEstimator(
		ctx,
		tracker,
		sampler,
		estimatorHistory,
		estimatorSkip,
		estimatorTarget,
	)
	if err != nil {
		log.Fatalln("could not create estimator:", err)
	}

	h := handler.New(estimator, []string{"low", "medium", "high"})
	log.Println("start server on:", *addr)
	if err := http.ListenAndServe(*addr, h); err != nil {
		log.Fatalln("server error:", err)
	}
}
