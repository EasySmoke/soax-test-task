package main

import "net"

const timeoutStr = "timeout"

func classifyNetError(err error) string {
	cause := err
	for {
		if unwrap, ok := cause.(interface{ Unwrap() error }); ok {
			cause = unwrap.Unwrap()
			continue
		}
		break
	}

	if cause, ok := cause.(net.Error); ok && cause.Timeout() {
		return timeoutStr
	}

	return "unknown_error"
}
