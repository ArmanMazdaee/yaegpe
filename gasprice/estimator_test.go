package gasprice

import (
	"context"
	"errors"
	"math/big"
	"sync"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
)

var ErrBlockNotFound = errors.New("could not get block")

type trackerMock struct {
	lastHead common.Hash
	subs     map[chan<- struct{}]struct{}
	lock     sync.RWMutex
}

func newTrackerMock(head common.Hash) *trackerMock {
	return &trackerMock{
		head,
		make(map[chan<- struct{}]struct{}),
		sync.RWMutex{},
	}
}

func (t *trackerMock) head(ctx context.Context) (common.Hash, error) {
	t.lock.RLock()
	defer t.lock.RUnlock()
	return t.lastHead, nil
}

func (t *trackerMock) subscribe() trackerSubscription {
	t.lock.Lock()
	defer t.lock.Unlock()
	ch := make(chan struct{}, 1)
	t.subs[ch] = struct{}{}

	unsubscribe := func() {
		t.lock.Lock()
		defer t.lock.Unlock()
		delete(t.subs, ch)
		close(ch)
	}

	return trackerSubscription{ch, unsubscribe}
}

func (t *trackerMock) changeHead(head common.Hash) {
	t.lock.Lock()
	defer t.lock.Unlock()
	if t.lastHead != head {
		t.lastHead = head
		for sub := range t.subs {
			select {
			case sub <- struct{}{}:
			default:
			}
		}
	}
}

type samplerMock struct {
	samples []Sample
	count   int
	lock    sync.RWMutex
}

func newSamplerMock(samples []Sample) *samplerMock {
	return &samplerMock{
		samples,
		0,
		sync.RWMutex{},
	}
}

func (s *samplerMock) sample(ctx context.Context, hash common.Hash) (Sample, error) {
	s.lock.Lock()
	s.count += 1
	s.lock.Unlock()
	for _, sample := range s.samples {
		if sample.header.Hash() == hash {
			return sample, nil
		}
	}
	return Sample{}, ErrBlockNotFound
}

func (s *samplerMock) requestCount() int {
	s.lock.RLock()
	defer s.lock.RUnlock()
	return s.count
}

func (s *samplerMock) resetCount() {
	s.lock.Lock()
	defer s.lock.Unlock()
	s.count = 0
}
func TestEstimatorGasPrices(t *testing.T) {
	samples := make([]Sample, 3)
	samples[0] = Sample{
		&types.Header{},
		[]*big.Int{big.NewInt(30), big.NewInt(20), big.NewInt(40), big.NewInt(30)},
	}
	samples[1] = Sample{
		&types.Header{ParentHash: samples[0].header.Hash()},
		[]*big.Int{big.NewInt(40), big.NewInt(30), big.NewInt(10)},
	}
	samples[2] = Sample{
		&types.Header{ParentHash: samples[1].header.Hash()},
		[]*big.Int{big.NewInt(35), big.NewInt(25), big.NewInt(45), big.NewInt(35)},
	}
	expected := []*big.Int{big.NewInt(21), big.NewInt(38), big.NewInt(31)}
	tracker := newTrackerMock(samples[2].header.Hash())
	sampler := newSamplerMock(samples)
	estimator := &Estimator{
		tracker:    tracker,
		sampler:    sampler,
		skip:       1,
		history:    2,
		targets:    []Target{{0, 0.5}, {0.5, 1}, {0, 1}},
		lastHead:   zeroHash,
		lastPrices: nil,
		chans:      nil,
		lock:       sync.RWMutex{},
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	results, err := estimator.GasPrices(ctx)
	if err != nil {
		t.Fatal("GasPrices returned error:", err)
	}
	if len(expected) != len(results) {
		t.Fatal("GasPrices returned wrong number of prices")
	}
	for i := range expected {
		if expected[i].Cmp(results[i]) != 0 {
			t.Fatal("GasPrices retuned wrong prices")
		}
	}

	sampler.resetCount()
	_, err = estimator.GasPrices(ctx)
	if err != nil {
		t.Fatal("GasPrices returned error when reading from cache:", err)
	}
	if sampler.requestCount() != 0 {
		t.Fatal("Gasprices should be returned from cache")
	}
}

func TestEstimatorListen(t *testing.T) {
	samples := make([]Sample, 3)
	samples[0] = Sample{
		&types.Header{},
		[]*big.Int{big.NewInt(30), big.NewInt(20), big.NewInt(40), big.NewInt(30)},
	}
	samples[1] = Sample{
		&types.Header{ParentHash: samples[0].header.Hash()},
		[]*big.Int{big.NewInt(40), big.NewInt(30), big.NewInt(10)},
	}
	samples[2] = Sample{
		&types.Header{ParentHash: samples[1].header.Hash()},
		[]*big.Int{big.NewInt(35), big.NewInt(25), big.NewInt(45), big.NewInt(35)},
	}
	expected := big.NewInt(31)
	tracker := newTrackerMock(samples[1].header.Hash())
	sampler := newSamplerMock(samples)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	estimator, err := NewEstimator(ctx, tracker, sampler, 0, 2, []Target{{0, 1}})
	if err != nil {
		t.Fatal("could not create estimator")
	}

	<-time.After(100 * time.Millisecond)
	tracker.changeHead(samples[2].header.Hash())
	<-time.After(100 * time.Millisecond)

	estimator.lock.RLock()
	defer estimator.lock.RUnlock()
	if estimator.lastHead != samples[2].header.Hash() {
		t.Fatal("the lastHead did not get updated")
	}
	if len(estimator.lastPrices) != 1 {
		t.Fatal("lastPrices has wrong length")
	}
	if estimator.lastPrices[0].Cmp(expected) != 0 {
		t.Fatal("lastPrice has wrong value")
	}
}
