package fmp4

import (
	"io"

	"github.com/gwuhaolin/livego/av"
)

const (
	fragmentLen = 1
)

type Muxer struct {
}

func NewMuxer() *Muxer {
	return &Muxer{}
}

func (muxer *Muxer) Mux(p *av.Packet, w io.Writer) error {

	return nil
}
