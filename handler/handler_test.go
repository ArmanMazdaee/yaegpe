package handler

import (
	"context"
	"encoding/json"
	"errors"
	"math/big"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strconv"
	"testing"
)

type estimatorMock []*big.Int

func (e estimatorMock) GasPrices(ctx context.Context) ([]*big.Int, error) {
	return e, nil
}

func TestHandlerServeHttp(t *testing.T) {
	tests := []struct {
		estimator estimatorMock
		names     []string
		expect    map[string]string
	}{
		{
			estimatorMock{big.NewInt(32), big.NewInt(64)},
			[]string{"low", "high"},
			map[string]string{"low": "32", "high": "64"},
		},
		{
			estimatorMock{big.NewInt(32), big.NewInt(64), big.NewInt(128)},
			[]string{"low", "high"},
			map[string]string{"low": "32", "high": "64"},
		},
		{
			estimatorMock{big.NewInt(32)},
			[]string{"low", "high"},
			map[string]string{"low": "32"},
		},
	}

	for i, test := range tests {
		t.Run(strconv.FormatInt(int64(i), 10), func(t *testing.T) {
			handler := Handler{test.estimator, test.names}
			r := httptest.NewRequest(http.MethodGet, "/", nil)
			w := httptest.NewRecorder()
			handler.ServeHTTP(w, r)
			if w.Code != http.StatusOK {
				t.Errorf("status code should be %d but it is %d", http.StatusOK, w.Code)
			}
			result := make(map[string]string)
			json.NewDecoder(w.Body).Decode(&result)
			if !reflect.DeepEqual(test.expect, result) {
				t.Error("response body is not correct")
			}
		})
	}
}

type faultyEstimatorMock struct{}

func (e faultyEstimatorMock) GasPrices(ctx context.Context) ([]*big.Int, error) {
	return nil, errors.New("some error")
}

func TestHandlerServeHttpError(t *testing.T) {
	estimator := faultyEstimatorMock{}
	handler := Handler{estimator, []string{"low", "high"}}
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, r)
	if w.Code != http.StatusInternalServerError {
		t.Errorf("status code should be %d but it is %d", http.StatusInternalServerError, w.Code)
	}
}
