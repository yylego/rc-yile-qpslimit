package service_test

import (
	"fmt"
	"net/http"
	"os"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	restyv2 "github.com/go-resty/resty/v2"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"github.com/yylego/rc-yile-qpslimit/internal/middleware"
	"github.com/yylego/rc-yile-qpslimit/internal/service"
	"github.com/yylego/rese"
)

var testBaseURL string
var testQuit chan struct{}

func TestMain(m *testing.M) {
	testQuit = make(chan struct{})

	testAddress := fmt.Sprintf(":%d", 19000+time.Now().UnixNano()%1000)
	go service.Run(100, testAddress, testQuit)
	time.Sleep(time.Second)

	testBaseURL = "http://localhost" + testAddress

	code := m.Run()

	close(testQuit)
	time.Sleep(time.Millisecond * 500)
	os.Exit(code)
}

func TestHealth(t *testing.T) {
	resp := rese.C1(restyv2.New().R().Get(testBaseURL + "/health"))
	require.Equal(t, http.StatusOK, resp.StatusCode())
	require.Contains(t, resp.String(), "ok")
}

func TestPingWithoutKey(t *testing.T) {
	resp := rese.C1(restyv2.New().R().Get(testBaseURL + "/api/ping"))
	require.Equal(t, http.StatusOK, resp.StatusCode())
	require.Contains(t, resp.String(), "value")
}

func TestSingleKeyOverLimit(t *testing.T) {
	key := uuid.New().String()
	var accepted, rejected atomic.Int64
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		for range 200 {
			resp := rese.C1(restyv2.New().R().
				SetHeader(middleware.RateLimitTokenName, key).
				Get(testBaseURL + "/api/ping"))
			if resp.StatusCode() == http.StatusOK {
				accepted.Add(1)
			} else if resp.StatusCode() == http.StatusTooManyRequests {
				rejected.Add(1)
			}
		}
	}()
	wg.Wait()

	t.Log("accepted:", accepted.Load(), "rejected:", rejected.Load())
	require.True(t, accepted.Load() <= 100)
	require.True(t, accepted.Load() >= 50)
	require.True(t, rejected.Load() > 0)
}

func TestMultiKeyIsolation(t *testing.T) {
	keyA := uuid.New().String()
	keyB := uuid.New().String()

	var acceptedA, acceptedB atomic.Int64
	var wg sync.WaitGroup

	wg.Add(2)
	go func() {
		defer wg.Done()
		for range 150 {
			resp := rese.C1(restyv2.New().R().
				SetHeader(middleware.RateLimitTokenName, keyA).
				Get(testBaseURL + "/api/ping"))
			if resp.StatusCode() == http.StatusOK {
				acceptedA.Add(1)
			}
		}
	}()
	go func() {
		defer wg.Done()
		for range 150 {
			resp := rese.C1(restyv2.New().R().
				SetHeader(middleware.RateLimitTokenName, keyB).
				Get(testBaseURL + "/api/ping"))
			if resp.StatusCode() == http.StatusOK {
				acceptedB.Add(1)
			}
		}
	}()
	wg.Wait()

	t.Log("keyA accepted:", acceptedA.Load())
	t.Log("keyB accepted:", acceptedB.Load())

	require.True(t, acceptedA.Load() <= 100)
	require.True(t, acceptedA.Load() >= 50)
	require.True(t, acceptedB.Load() <= 100)
	require.True(t, acceptedB.Load() >= 50)
}
