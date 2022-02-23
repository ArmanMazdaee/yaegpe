package gasprice

import (
	"math/big"
	"strconv"
	"testing"
)

func TestBigIntHeapLen(t *testing.T) {
	tests := []struct {
		heap bigIntHeap
	}{
		{
			bigIntHeap{big.NewInt(64), big.NewInt(32)},
		},
		{
			bigIntHeap{},
		},
	}

	for i, test := range tests {
		t.Run(strconv.FormatInt(int64(i), 10), func(t *testing.T) {
			l := test.heap.Len()
			if l != len(test.heap) {
				t.Errorf("Len expected to return %d but returned %d", len(test.heap), l)
			}
		})
	}
}

func TestBigIntHeapLess(t *testing.T) {
	tests := []struct {
		heap bigIntHeap
		i, j int
		less bool
	}{
		{
			bigIntHeap{big.NewInt(64), big.NewInt(32)},
			0, 1,
			false,
		},
		{
			bigIntHeap{big.NewInt(32), big.NewInt(64)},
			0, 1,
			true,
		},
		{
			bigIntHeap{big.NewInt(128), big.NewInt(32), big.NewInt(64)},
			0, 2,
			false,
		},
	}

	for i, test := range tests {
		t.Run(strconv.FormatInt(int64(i), 10), func(t *testing.T) {
			less := test.heap.Less(test.i, test.j)
			if less != test.less {
				t.Errorf("Less expected to return %t but returned %t", test.less, less)
			}
		})
	}
}

func TestBigIntHeapSwap(t *testing.T) {
	tests := []struct {
		heap bigIntHeap
		i, j int
	}{
		{
			bigIntHeap{big.NewInt(64), big.NewInt(32)},
			0, 1,
		},
		{
			bigIntHeap{big.NewInt(128), big.NewInt(32), big.NewInt(64)},
			0, 2,
		},
		{
			bigIntHeap{big.NewInt(128), big.NewInt(32), big.NewInt(64)},
			1, 1,
		},
	}

	for i, test := range tests {
		t.Run(strconv.FormatInt(int64(i), 10), func(t *testing.T) {
			heap := make(bigIntHeap, len(test.heap))
			copy(test.heap, heap)
			heap.Swap(test.i, test.j)
			if len(heap) != len(test.heap) {
				t.Error("swap has change the len")
			}
			for i := range heap {
				if i != test.i && i != test.j && heap[i] == test.heap[i] {
					continue
				}
				if (i == test.i || i == test.j) && heap[test.i] == heap[test.j] {
					continue
				}
				t.Error("swap is no correct")
			}
		})
	}
}

func TestBigIntHeapPush(t *testing.T) {
	tests := []struct {
		heap   bigIntHeap
		number *big.Int
	}{
		{
			bigIntHeap{big.NewInt(64), big.NewInt(32)},
			big.NewInt(128),
		},
		{
			bigIntHeap{},
			big.NewInt(128),
		},
	}

	for i, test := range tests {
		t.Run(strconv.FormatInt(int64(i), 10), func(t *testing.T) {
			heap := make(bigIntHeap, len(test.heap))
			copy(test.heap, heap)
			heap.Push(test.number)
			if len(heap) != len(test.heap)+1 {
				t.Error("push did not increase the length")
			}
			if heap[len(heap)-1].Cmp(test.number) != 0 {
				t.Error("last item is not correct")
			}
			for i := range test.heap {
				if heap[i] != test.heap[i] {
					t.Error("push did not keep the rests")
				}
			}
		})
	}
}

func TestBigIntHeapPop(t *testing.T) {
	tests := []struct {
		heap bigIntHeap
	}{
		{
			bigIntHeap{big.NewInt(64), big.NewInt(32)},
		},
		{
			bigIntHeap{big.NewInt(64)},
		},
	}

	for i, test := range tests {
		t.Run(strconv.FormatInt(int64(i), 10), func(t *testing.T) {
			heap := make(bigIntHeap, len(test.heap))
			copy(test.heap, heap)
			number, ok := heap.Pop().(*big.Int)
			if !ok {
				t.Error("pop did not return a bigint")
			}
			if len(heap) != len(test.heap)-1 {
				t.Error("pop has not decreased the length")
			}
			if test.heap[len(test.heap)-1].Cmp(number) != 0 {
				t.Error("pop returned wrong number")
			}
			for i := range heap {
				if heap[i] != test.heap[i] {
					t.Error("pop did not keep the rests")
				}
			}
		})
	}
}

func TestBigIntHeapPopPanic(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Error("pop did not paniced on empty heap")
		}
	}()

	heap := bigIntHeap{}
	heap.Pop()
}
