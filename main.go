package main

import (
	"context"
	"math/rand"
	"net/http"
	"time"

	"github.com/prometheus/client_golang/prometheus/promhttp"
)

func main() {
	ctx := context.Background()
	rand.Seed(time.Now().UnixNano())

	l, err := NewListener(time.Second/5, 5)
	if err != nil {
		panic(err)
	}
	go l.Run(ctx)

	d := NewRandomDialer(time.Second/5, time.Second, time.Second*3, 0.5)
	go d.Run(ctx)

	http.Handle("/metrics", promhttp.Handler())
	http.ListenAndServe(":8082", nil)
}
