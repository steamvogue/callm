package client

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
)

func TestUserAgentOnEveryCatalogPage(t *testing.T) {
	var calls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		if r.Header.Get("User-Agent") != DefaultUserAgent {
			t.Error("project user agent missing")
		}
		if r.URL.Query().Get("after_id") == "" {
			io.WriteString(w, `{"data":[{"id":"a"}],"has_more":true,"last_id":"a"}`)
		} else {
			io.WriteString(w, `{"data":[{"id":"b"}],"has_more":false}`)
		}
	}))
	defer server.Close()
	models, err := NewClient(server.URL, "dummy", "ant").ListModels(context.Background())
	if err != nil || calls.Load() != 2 || len(models) != 2 {
		t.Fatalf("calls=%d models=%v err=%v", calls.Load(), models, err)
	}
}
