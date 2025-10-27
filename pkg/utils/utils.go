package utils

import (
	"context"
	"encoding/json"
	"math/rand"
	"time"
)

func Dumps(object any) {
	// print as JSON for debugging purposes
	data, err := json.MarshalIndent(object, "", "  ")
	if err != nil {
		panic(err)
	}
	println(string(data))
}

func DoWithBackoff(ctx context.Context, operation func() error, attempts int, backoff time.Duration) error {
	var err error
	for i := 0; i < attempts; i++ {
		if err = ctx.Err(); err != nil {
			return err
		}

		err = operation()
		if err == nil {
			return nil
		}

		if i >= attempts {
			return err
		}

		sleep := backoff * time.Duration(1<<i)
		jitter := time.Duration(rand.Int63n(int64(backoff / 4)))
		sleep += jitter

		select {
		case <-time.After(sleep):
			// continue to the next attempt
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	return err
}
