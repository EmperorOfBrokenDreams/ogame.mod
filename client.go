package ogame

import (
	"context"
	"fmt"
	"net/http"
	"sync/atomic"
	"time"

	"golang.org/x/time/rate"
)

// OGameClient ...
type OGameClient struct {
	http.Client
	UserAgent    string
	rpsCounter   int32
	rps          int32
	maxRPS       int32
	rpsStartTime int64
	ratelimiter  *rate.Limiter
}

// NewOGameClient ...
func NewOGameClient() *OGameClient {
	tr := &http.Transport{
		MaxIdleConnsPerHost: 10,
		MaxIdleConns:        100,
		MaxConnsPerHost:     10,
		IdleConnTimeout:     30 * time.Second,
		DisableCompression:  true,
	}
	//rl := rate.NewLimiter(rate.Every(10*time.Second), 50) // 50 request every 10 seconds
	rl := rate.NewLimiter(rate.Every(10*time.Second), 50) // 50 request every 10 seconds
	rl.SetLimit(12)
	client := &OGameClient{
		Client: http.Client{
			Timeout:   60 * time.Second,
			Transport: tr,
		},
		maxRPS:      0,
		ratelimiter: rl,
	}

	const delay = 1

	go func() {
		for {
			prevRPS := atomic.SwapInt32(&client.rpsCounter, 0)
			atomic.StoreInt32(&client.rps, prevRPS/delay)
			atomic.StoreInt64(&client.rpsStartTime, time.Now().Add(delay*time.Second).UnixNano())
			time.Sleep(delay * time.Second)
		}
	}()

	return client
}

// SetMaxRPS ...
func (c *OGameClient) SetMaxRPS(maxRPS int32) {
	c.maxRPS = maxRPS
}

// SetRateLimit ...
func (c *OGameClient) SetRateLimit(ratelimit int32) {
	c.ratelimiter.SetLimit(rate.Limit(ratelimit))
}

// SetHttpTimeout ...
func (c *OGameClient) SetHttpTimeout(timeout int64) {
	c.Timeout = time.Duration(timeout) * time.Second
}

// GetMaxRPS
func (c *OGameClient) GetMaxRPS() int32 {
	return c.maxRPS
}

func (c *OGameClient) incrRPS() {
	newRPS := atomic.AddInt32(&c.rpsCounter, 1)
	if c.maxRPS > 0 && newRPS > c.maxRPS {
		s := atomic.LoadInt64(&c.rpsStartTime) - time.Now().UnixNano()
		//fmt.Printf("throttle %d\n", s)
		time.Sleep(time.Duration(s))
	}
}

// Do executes a request
func (c *OGameClient) Do(req *http.Request) (*http.Response, error) {
	// Comment out the below 5 lines to turn off ratelimiting
	if c.ratelimiter.Limit() > rate.Limit(0) {
		ctx := context.Background()
		err := c.ratelimiter.Wait(ctx) // This is a blocking call. Honors the rate limit
		if err != nil {
			return nil, err
		}
	}
	c.incrRPS()
	req.Header.Add("User-Agent", c.UserAgent)
	//fmt.Printf("%s Request: %s #%d / %f\n", time.Now().Format("2006-01-02 15:04:05"), req.URL, c.ratelimiter.Burst(), c.ratelimiter.Limit())
	return c.Client.Do(req)
}

// FakeDo for testing purposes
func (c *OGameClient) FakeDo() {
	c.incrRPS()
	fmt.Println("FakeDo")
}

// GetRPS gets the current client RPS
func (c *OGameClient) GetRPS() int32 {
	return atomic.LoadInt32(&c.rps)
}
