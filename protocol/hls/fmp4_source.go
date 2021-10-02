package hls

import (
	"bytes"
	"container/list"
	"fmt"

	"github.com/gwuhaolin/livego/av"
	"github.com/gwuhaolin/livego/container/flv"
	"github.com/gwuhaolin/livego/container/fmp4"
	"github.com/gwuhaolin/livego/parser"
	log "github.com/sirupsen/logrus"
)

type Fragment struct {
}

type FragmentCache struct {
	fragList *list.List
	fragMap  map[string]Fragment
}

func NewFragmentCache() *FragmentCache {
	return &FragmentCache{
		fragList: list.New(),
		fragMap:  make(map[string]Fragment),
	}
}

type Fmp4Source struct {
	demuxer     *flv.Demuxer
	muxer       *fmp4.Muxer
	bwriter     *bytes.Buffer
	btswriter   *bytes.Buffer
	cache       FragmentCache
	parser      *parser.CodecParser
	closed      bool
	packetQueue chan *av.Packet
}

func NewFmp4Source() *Fmp4Source {
	s := &Fmp4Source{
		demuxer:     flv.NewDemuxer(),
		muxer:       fmp4.NewMuxer(),
		bwriter:     bytes.NewBuffer(make([]byte, 100*1024)),
		cache:       *NewFragmentCache(),
		parser:      parser.NewCodecParser(),
		closed:      false,
		packetQueue: make(chan *av.Packet),
	}

	go func() {
		err := s.SendPacket()
		if err != nil {
			log.Debug("send packet error: ", err)
			s.closed = true
		}
	}()

	return s
}

func (source *Fmp4Source) SendPacket() error {

	for {
		if source.closed {
			return fmt.Errorf("closed")
		}

		p, ok := <-source.packetQueue
		if ok {
			if p.IsMetadata {
				continue
			}

			err := source.demuxer.Demux(p)
			if err == flv.ErrAvcEndSEQ {
				log.Warning(err)
				continue
			} else {
				if err != nil {
					log.Warning(err)
					return err
				}
			}

		}
	}

	return nil
}

//到这里rtmp的header已经解析好，packet.data中可能有
//1.264的nalu 2.aac数据 3.264的sequence
//要做的：对当前帧的判断，对之前数据的切片
func (source *Fmp4Source) parse(p *av.Packet) (cts int32, isSeq bool, err error) {
	var compositionTime int32
	var ah av.AudioPacketHeader
	var vh av.VideoPacketHeader
	if p.IsVideo {
		vh = p.Header.(av.VideoPacketHeader)
		if vh.CodecID() != av.VIDEO_H264 {
			return compositionTime, false, ErrNoSupportVideoCodec
		}
		compositionTime = vh.CompositionTime()
		if vh.IsKeyFrame() && vh.IsSeq() {
			return compositionTime, true, source.tsparser.Parse(p, source.bwriter)
		}
	} else {
		ah = p.Header.(av.AudioPacketHeader)
		if ah.SoundFormat() != av.SOUND_AAC {
			return compositionTime, false, ErrNoSupportAudioCodec
		}
		if ah.AACPacketType() == av.AAC_SEQHDR {
			return compositionTime, true, source.tsparser.Parse(p, source.bwriter)
		}
	}
	source.bwriter.Reset()
	if err := source.tsparser.Parse(p, source.bwriter); err != nil {
		return compositionTime, false, err
	}
	p.Data = source.bwriter.Bytes()
	//切片条件
	if p.IsVideo && vh.IsKeyFrame() {
		source.cut()
	}
	return compositionTime, false, nil
	return
}

func (s *Fmp4Source) cut() {

}
