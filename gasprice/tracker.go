package gasprice

import (
	"context"
	"log"
	"sync"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
)

var zeroHash = common.Hash{}
var pollWait = 5 * time.Second

type trackerSubscription struct {
	ch          <-chan struct{}
	unsubscribe func()
}

type Tracker interface {
	head(ctx context.Context) (common.Hash, error)
	subscribe() trackerSubscription
}

type headResult struct {
	header common.Hash
	err    error
}

type PollingTracker struct {
	provider  Provider
	chans     []chan<- headResult
	subs      map[chan<- struct{}]struct{}
	lastHead  common.Hash
	lastFetch time.Time
	lock      sync.RWMutex
}

func NewPollingTracker(ctx context.Context, provider Provider) *PollingTracker {
	t := &PollingTracker{
		provider,
		nil,
		make(map[chan<- struct{}]struct{}),
		zeroHash,
		time.Time{},
		sync.RWMutex{},
	}
	go t.poll(ctx)
	return t
}

func (t *PollingTracker) broadcastHead() {
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()

	header, err := t.provider.HeaderByNumber(ctx, nil)
	t.lock.Lock()
	defer t.lock.Unlock()
	if err != nil {
		for _, ch := range t.chans {
			ch <- headResult{zeroHash, err}
			close(ch)
		}
		t.chans = nil
		return
	}

	t.lastFetch = time.Now()
	head := header.Hash()
	for _, ch := range t.chans {
		ch <- headResult{head, nil}
		close(ch)
	}
	t.chans = nil
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

func (t *PollingTracker) asyncHead() <-chan headResult {
	ch := make(chan headResult, 1)
	t.lock.Lock()
	defer t.lock.Unlock()
	chans := append(t.chans, ch)
	t.chans = chans
	if len(chans) == 1 {
		go t.broadcastHead()
	}
	return ch
}

func (t *PollingTracker) head(ctx context.Context) (common.Hash, error) {
	select {
	case r := <-t.asyncHead():
		return r.header, r.err
	case <-ctx.Done():
		return zeroHash, ctx.Err()
	}
}

func (t *PollingTracker) poll(ctx context.Context) {
	for {
		now := time.Now()
		t.lock.RLock()
		duration := now.Sub(t.lastFetch)
		t.lock.RUnlock()

		var wait <-chan time.Time
		if duration < pollWait {
			wait = time.After(pollWait - duration)
		} else {
			for {
				_, err := t.head(ctx)
				if err == nil {
					break
				}
			}
			wait = time.After(pollWait)
		}

		select {
		case <-wait:
		case <-ctx.Done():
			return
		}
	}
}

func (t *PollingTracker) subscribe() trackerSubscription {
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

type SubscribedTracker struct {
	provider Provider
	subs     map[chan<- struct{}]struct{}
	lastHead common.Hash
	lock     sync.RWMutex
}

func NewSubscribedTracker(ctx context.Context, provider Provider) (*SubscribedTracker, error) {
	t := &SubscribedTracker{
		provider,
		make(map[chan<- struct{}]struct{}),
		zeroHash,
		sync.RWMutex{},
	}
	header, err := provider.HeaderByNumber(ctx, nil)
	if err != nil {
		return nil, err
	}
	t.lastHead = header.Hash()

	if err := t.listen(ctx); err != nil {
		return nil, err
	}

	return t, nil
}

func (t *SubscribedTracker) listen(ctx context.Context) error {
	ch := make(chan *types.Header)
	sub, err := t.provider.SubscribeNewHead(ctx, ch)
	if err != nil {
		return err
	}

	go func() {
		for {
			select {
			case header := <-ch:
				t.lock.Lock()
				t.lastHead = header.Hash()
				t.lock.Unlock()
				t.lock.RLock()
				for sub := range t.subs {
					select {
					case sub <- struct{}{}:
					default:
					}
				}
				t.lock.RUnlock()
			case <-ctx.Done():
				return
			case err := <-sub.Err():
				for err != nil {
					log.Println("subscriber returned err:", err)
					err = t.listen(ctx)
				}
				return
			}
		}
	}()

	return nil
}

func (t *SubscribedTracker) head(ctx context.Context) (common.Hash, error) {
	t.lock.RLock()
	defer t.lock.RUnlock()
	return t.lastHead, nil
}

func (t *SubscribedTracker) subscribe() trackerSubscription {
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
