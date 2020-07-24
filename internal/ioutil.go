package internal

import (
	"io"
	"io/ioutil"

	"github.com/go-kit/kit/log"
	"github.com/go-kit/kit/log/level"
)

// ExhaustCloseWithLogOnErr closes the io.ReadCloser with a log message on error but exhausts the reader before.
func ExhaustCloseWithLogOnErr(logger log.Logger, rc io.ReadCloser) {
	if _, err := io.Copy(ioutil.Discard, rc); err != nil {
		level.Error(logger).Log("msg", "failed to exhaust reader, performance may be impeded", "err", err)
	}

	if err := rc.Close(); err != nil {
		level.Error(logger).Log("msg", "failed to exhaust reader, performance may be impeded", "err", err)
		return
	}
}
