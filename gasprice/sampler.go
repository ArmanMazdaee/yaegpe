package gasprice

import (
	"context"
	"math/big"
	"sync"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	lru "github.com/hashicorp/golang-lru"
)

const cacheSize = 128

type Sample struct {
	header *types.Header
	prices []*big.Int
}

type Sampler interface {
	sample(ctx context.Context, hash common.Hash) (Sample, error)
}

type sampleResult struct {
	sample Sample
	err    error
}

type MinimumSampler struct {
	provider Provider
	size     int
	minPrice *big.Int
	cache    *lru.Cache
	chans    map[common.Hash][]chan<- sampleResult
	lock     sync.Mutex
}

func NewMinimumSampler(provider Provider, size int, minPrice *big.Int) (*MinimumSampler, error) {
	cache, err := lru.New(cacheSize)
	if err != nil {
		return nil, err
	}
	return &MinimumSampler{
		provider,
		size,
		minPrice,
		cache,
		make(map[common.Hash][]chan<- sampleResult),
		sync.Mutex{},
	}, nil
}

func (s *MinimumSampler) fetch(ctx context.Context, hash common.Hash) (Sample, error) {
	block, err := s.provider.BlockByHash(ctx, hash)
	if err != nil {
		return Sample{}, err
	}

	baseFee := block.BaseFee()
	coinbase := block.Coinbase()
	txs := block.Transactions()
	pricesHeap := make(bigIntHeap, 0, len(txs))
	for _, tx := range txs {
		tip, err := tx.EffectiveGasTip(baseFee)
		if err != nil {
			return Sample{}, err
		}

		price := new(big.Int).Add(baseFee, tip)
		if price.Cmp(s.minPrice) == -1 {
			continue
		}

		sender, err := types.NewLondonSigner(tx.ChainId()).Sender(tx)
		if err != nil {
			return Sample{}, err
		}
		if sender == coinbase {
			continue
		}

		pricesHeap = append(pricesHeap, price)
	}

	prices := make([]*big.Int, 0, s.size)
	for len(prices) < s.size && pricesHeap.Len() > 0 {
		price := pricesHeap.Pop().(*big.Int)
		prices = append(prices, price)
	}

	return Sample{block.Header(), prices}, nil
}

func (s *MinimumSampler) broadcastSample(hash common.Hash) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()

	sample, err := s.fetch(ctx, hash)
	s.lock.Lock()
	defer s.lock.Unlock()
	if err != nil {
		for _, ch := range s.chans[hash] {
			ch <- sampleResult{Sample{}, err}
			close(ch)
		}
		delete(s.chans, hash)
		return
	}

	s.cache.Add(hash, sample)
	for _, ch := range s.chans[hash] {
		ch <- sampleResult{sample, nil}
		close(ch)
	}
	delete(s.chans, hash)
}

func (s *MinimumSampler) asyncSample(hash common.Hash) <-chan sampleResult {
	ch := make(chan sampleResult, 1)
	s.lock.Lock()
	defer s.lock.Unlock()
	if value, ok := s.cache.Get(hash); ok {
		ch <- sampleResult{value.(Sample), nil}
		close(ch)
		return ch
	}

	chans := append(s.chans[hash], ch)
	s.chans[hash] = chans
	if len(chans) == 1 {
		go s.broadcastSample(hash)
	}
	return ch
}

func (s *MinimumSampler) sample(ctx context.Context, hash common.Hash) (Sample, error) {
	if value, ok := s.cache.Get(hash); ok {
		return value.(Sample), nil
	}
	select {
	case r := <-s.asyncSample(hash):
		return r.sample, r.err
	case <-ctx.Done():
		return Sample{}, ctx.Err()
	}
}
