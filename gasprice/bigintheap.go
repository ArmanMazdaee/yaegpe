package gasprice

import "math/big"

type bigIntHeap []*big.Int

func (h bigIntHeap) Len() int { return len(h) }

func (h bigIntHeap) Less(i, j int) bool { return h[i].Cmp(h[j]) == -1 }

func (h bigIntHeap) Swap(i, j int) { h[i], h[j] = h[j], h[i] }

func (h *bigIntHeap) Push(tx interface{}) { *h = append(*h, tx.(*big.Int)) }

func (h *bigIntHeap) Pop() interface{} {
	old := *h
	n := len(old)
	*h = old[0 : n-1]
	value := old[n-1]
	return value
}
