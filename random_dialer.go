package main

import (
	"context"
	"math/rand"
	"net"
	"time"

	"github.com/pkg/errors"
	"go.uber.org/zap"
)

type RandomDialer struct {
	lg *zap.SugaredLogger

	delay         *time.Ticker
	minTTL        int
	maxTTL        int
	badDialChance float32
}

func NewRandomDialer(delay, minTTL, maxTTL time.Duration, badDialChance float32) *RandomDialer {
	return &RandomDialer{
		lg:            zap.NewExample().Sugar(),
		delay:         time.NewTicker(delay),
		minTTL:        int(minTTL),
		maxTTL:        int(maxTTL),
		badDialChance: badDialChance,
	}
}

func (d *RandomDialer) Run(ctx context.Context) {
	for {
		select {
		case <-d.delay.C:
			go func() {
				if err := d.dial(d.isBadDial()); err != nil {
					d.lg.Errorw("can't dial", "err", err.Error())
				}
			}()
		case <-ctx.Done():
			return
		}
	}
}

func (d *RandomDialer) dial(isBad bool) error {
	conn, err := net.Dial("tcp", ":8081")
	if err != nil {
		return errors.Wrap(err, "dial")
	}

	if isBad {
		return nil
	}

	defer conn.Close()

	if err = d.handleConn(conn); err != nil {
		if classifyNetError(err) != timeoutStr {
			return errors.Wrap(err, "handle conn")
		}
	}

	return nil
}

func (d *RandomDialer) handleConn(conn net.Conn) error {
	ttl := d.calculateTTL()
	if err := conn.SetDeadline(time.Now().Add(ttl)); err != nil {
		return errors.Wrap(err, "set deadline")
	}

	chunk := make([]byte, 16)

	for {
		n, err := conn.Read(chunk)
		if err != nil {
			return err
		}

		d.lg.Debugf("request: %s", string(chunk[:n]))

		if _, err = conn.Write([]byte("PONG")); err != nil {
			return err
		}
	}
}

func (d *RandomDialer) calculateTTL() time.Duration {
	return time.Duration(rand.Intn(d.maxTTL-d.minTTL+1) + d.minTTL)
}

func (d *RandomDialer) isBadDial() bool {
	return rand.Float32() < d.badDialChance
}
