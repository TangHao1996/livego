package fmp4

const (
	fragmentLen = 1
)

type Muxer struct {
}

func NewMuxer() *Muxer {
	return &Muxer{}
}
