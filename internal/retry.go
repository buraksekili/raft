package internal

import (
	"math"
	"net/rpc"
	"time"
)

const (
	maxRetries = 3
	baseDelay  = 200 * time.Millisecond
)

// retryWithBackoff retries the given operation with exponential backoff
func retryWithBackoff(retries int, delay time.Duration, operation func() (*rpc.Client, error)) (*rpc.Client, error) {
	var lastError error

	for i := 0; i < retries; i++ {
		client, err := operation()
		if err == nil {
			return client, nil
		}
		secRetry := math.Pow(2, float64(i))
		delay := time.Duration(secRetry) * delay
		time.Sleep(delay)
		lastError = err
	}

	return nil, lastError
}

func RetryGeneric(maxRetries int, delay time.Duration, f func() (*rpc.Client, error)) error {
	_, err := retryWithBackoff(maxRetries, delay, f)
	return err
}

func RetryRPCDial(addr string) (*rpc.Client, error) {
	a, b := retryWithBackoff(maxRetries, baseDelay, func() (*rpc.Client, error) {
		client, err := rpc.Dial("tcp", addr)
		if err != nil {
			return nil, err
		}
		return client, nil
	})
	return a, b
}
