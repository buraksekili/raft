package internal

import (
	"math"
	"net/rpc"
	"time"
)

const maxRetries = 3
const baseDelay = 200 * time.Millisecond

// retryWithBackoff retries the given operation with exponential backoff
func retryWithBackoff(operation func() (*rpc.Client, error)) (*rpc.Client, error) {
	var lastError error

	for i := 0; i < maxRetries; i++ {
		client, err := operation()
		if err == nil {
			return client, nil
		}
		secRetry := math.Pow(2, float64(i))
		delay := time.Duration(secRetry) * baseDelay
		time.Sleep(delay)
		lastError = err
	}

	return nil, lastError
}

func RetryRPCDial(addr string) (*rpc.Client, error) {
	a, b := retryWithBackoff(func() (*rpc.Client, error) {
		client, err := rpc.Dial("tcp", addr)
		if err != nil {
			return nil, err
		}
		return client, nil
	})
	return a, b
}
