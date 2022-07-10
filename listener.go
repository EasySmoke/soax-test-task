package main

import (
	"context"
	"io"
	"net"
	"time"

	"github.com/pkg/errors"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"go.uber.org/zap"
)

var (
	activeConnsGauge = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "active_connections",
		Help: "Active connections gauge",
	})

	totalConnectionsCounter = promauto.NewCounter(prometheus.CounterOpts{
		Name: "total_connections",
		Help: "Total connections count",
	})

	totalDisconnectionsCounter = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "total_disconnections",
		Help: "Total disconnections count",
	}, []string{"status"})
)

type Listener struct {
	lg *zap.SugaredLogger
	ln *net.TCPListener

	connQuotes       chan struct{}
	pingPongDeadline time.Duration
	maxConns         int
}

func NewListener(pingPongDuration time.Duration, maxConns int) (*Listener, error) {
	ln, err := net.ListenTCP("tcp", &net.TCPAddr{Port: 8081})
	l := &Listener{
		lg:               zap.NewExample().Sugar(),
		ln:               ln,
		connQuotes:       make(chan struct{}, maxConns),
		pingPongDeadline: pingPongDuration,
	}

	l.generateConnQuotes(maxConns)

	return l, err
}

func (l *Listener) Run(ctx context.Context) {
	for {
		select {
		case <-l.connQuotes:
			if err := l.processConnQuote(ctx); err != nil {
				l.lg.Errorw("can't process connection quote", "err", err.Error())
				l.generateConnQuotes(1)
			}

			totalConnectionsCounter.Inc()
			activeConnsGauge.Inc()
		case <-ctx.Done():
			return
		}
	}
}

func (l *Listener) processConnQuote(ctx context.Context) error {
	conn, err := l.ln.AcceptTCP()
	if err != nil {
		return errors.Wrap(err, "accept tcp")
	}

	go l.observeConn(ctx, conn)

	return nil
}

func (l *Listener) observeConn(ctx context.Context, conn *net.TCPConn) {
	ticker := time.NewTicker(l.pingPongDeadline)

	for {
		select {
		case <-ticker.C:
			if err := l.ping(conn); err != nil {
				if err == io.EOF {
					totalDisconnectionsCounter.WithLabelValues("success").Inc()
					l.lg.Debug("successfully disconnected")
				} else {
					totalDisconnectionsCounter.WithLabelValues(classifyNetError(err)).Inc()
					l.lg.Errorw("can't ping", "err", err.Error())
				}

				_ = conn.Close()
				activeConnsGauge.Dec()
				l.generateConnQuotes(1)

				return
			}
		case <-ctx.Done():
			_ = conn.Close()

			return
		}
	}
}

func (l *Listener) ping(conn *net.TCPConn) error {
	if err := conn.SetDeadline(time.Now().Add(l.pingPongDeadline)); err != nil {
		return errors.Wrap(err, "set deadline")
	}

	if _, err := conn.Write([]byte("PING")); err != nil {
		return err
	}

	chunk := make([]byte, 16)
	n, err := conn.Read(chunk)
	if err != nil {
		return err
	}

	l.lg.Debugf("ping requset answer: %s", string(chunk[:n]))

	return nil
}

func (l *Listener) generateConnQuotes(count int) {
	for i := 0; i < count; i++ {
		l.connQuotes <- struct{}{}
	}
}
