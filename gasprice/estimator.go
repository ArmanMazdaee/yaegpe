package gasprice

import (
	"context"
	"errors"
	"math/big"
	"sort"
	"sync"
	"time"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
)

var ErrBadTargets = errors.New("targets is invalid")
var ErrNoSample = errors.New("no sample to estimate")

type Provider interface {
	HeaderByNumber(ctx context.Context, number *big.Int) (*types.Header, error)
	BlockByHash(ctx context.Context, hash common.Hash) (*types.Block, error)
	SubscribeNewHead(ctx context.Context, ch chan<- *types.Header) (ethereum.Subscription, error)
}

func clonePrices(src []*big.Int) []*big.Int {
	n := len(src)
	dst := make([]*big.Int, n)
	for i := 0; i < n; i++ {
		dst[i] = new(big.Int).Set(src[i])
	}
	return dst
}

type Target struct {
	Start float64
	End   float64
}

type gasPricesResult struct {
	prices []*big.Int
	err    error
}

type Estimator struct {
	tracker    Tracker
	sampler    Sampler
	skip       int
	history    int
	targets    []Target
	lastHead   common.Hash
	lastPrices []*big.Int
	chans      []chan<- gasPricesResult
	lock       sync.RWMutex
}

func NewEstimator(
	ctx context.Context,
	tracker Tracker,
	sampler Sampler,
	skip int,
	history int,
	targets []Target,
) (*Estimator, error) {
	for _, t := range targets {
		if t.Start < 0 || t.End <= t.Start || t.End > 1 {
			return nil, ErrBadTargets
		}
	}

	e := &Estimator{
		tracker,
		sampler,
		skip,
		history,
		targets,
		zeroHash,
		nil,
		nil,
		sync.RWMutex{},
	}
	go e.listen(ctx)

	return e, nil
}

func (e *Estimator) listen(ctx context.Context) {
	subscription := e.tracker.subscribe()
	for {
		select {
		case <-subscription.ch:
			e.GasPrices(ctx)
		case <-ctx.Done():
			subscription.unsubscribe()
			return
		}
	}
}

func (e *Estimator) estimate(ctx context.Context, head common.Hash) ([]*big.Int, error) {
	prices := make(bigIntHeap, 0)
	tip := head
	for i := 0; i < e.skip; i++ {

	}
	for i := 0; i < e.history; i++ {
		sample, err := e.sampler.sample(ctx, tip)
		if err != nil {
			return nil, err
		}
		prices = append(prices, sample.prices...)
		tip = sample.header.ParentHash
	}

	nprices := len(prices)
	if nprices == 0 {
		return nil, ErrNoSample
	}
	sort.Sort(prices)
	estimates := make([]*big.Int, len(e.targets))
	for i, t := range e.targets {
		start := int(t.Start * float64(nprices))
		end := int(t.End * float64(nprices))
		if end-start == 0 {
			end += 1
		}
		sum := new(big.Int)
		for j := start; j < end; j++ {
			sum.Add(sum, prices[j])
		}
		n := big.NewInt(int64(end - start))
		estimates[i] = new(big.Int).Div(sum, n)
	}

	return estimates, nil
}

func (e *Estimator) broadcastGasPrices(head common.Hash) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	prices, err := e.estimate(ctx, head)
	e.lock.Lock()
	defer e.lock.Unlock()
	if err != nil {
		for _, ch := range e.chans {
			ch <- gasPricesResult{nil, err}
			close(ch)
		}
		e.chans = nil
		return
	}

	if e.lastHead != head {
		return
	}

	e.lastPrices = prices
	for _, ch := range e.chans {
		ch <- gasPricesResult{clonePrices(prices), nil}
		close(ch)
	}
	e.chans = nil
}

func (e *Estimator) asyncGasPrices(head common.Hash) <-chan gasPricesResult {
	ch := make(chan gasPricesResult, 1)
	e.lock.Lock()
	defer e.lock.Unlock()
	lastHead := e.lastHead
	lastPrices := e.lastPrices
	if lastHead == head && lastPrices != nil {
		ch <- gasPricesResult{clonePrices(lastPrices), nil}
		close(ch)
		return ch
	}

	chans := append(e.chans, ch)
	e.chans = chans
	if lastHead != head {
		e.lastHead = head
		e.lastPrices = nil
		go e.broadcastGasPrices(head)
	}
	return ch
}

func (e *Estimator) GasPrices(ctx context.Context) ([]*big.Int, error) {
	head, err := e.tracker.head(ctx)
	if err != nil {
		return nil, err
	}

	e.lock.RLock()
	lastHead := e.lastHead
	lastPrices := e.lastPrices
	if lastPrices != nil {
		lastPrices = clonePrices(lastPrices)
	}
	e.lock.RUnlock()
	if lastHead == head && lastPrices != nil {
		return lastPrices, nil
	}

	select {
	case r := <-e.asyncGasPrices(head):
		return r.prices, r.err
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}
