package handler

import (
	"context"
	"encoding/json"
	"log"
	"math/big"
	"net/http"
)

type Estimator interface {
	GasPrices(ctx context.Context) ([]*big.Int, error)
}

type Handler struct {
	estimator Estimator
	names     []string
}

func New(estimator Estimator, names []string) *Handler {
	return &Handler{estimator, names}
}

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	prices, err := h.estimator.GasPrices(r.Context())
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("internal server error"))
		log.Println("could not get gas price:", err)
		return
	}

	results := make(map[string]string)

	n := len(h.names)
	if n > len(prices) {
		n = len(prices)
	}

	for i := 0; i < n; i++ {
		results[h.names[i]] = prices[i].String()
	}
	if err := json.NewEncoder(w).Encode(results); err != nil {
		log.Println("could not encode prices:", err)
	}
}
