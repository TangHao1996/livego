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
	codecParser *parser.CodecParser
	pts, dts    uint64
	stat        *fmp4Status
	closed      bool
	packetQueue chan *av.Packet
}

type fmp4Status struct {
	id             int64
	firstTimestamp int64
	lastTimestamp  int64
}

type fmp4Track struct {
	data []byte
}

func NewFmp4Source() *Fmp4Source {
	s := &Fmp4Source{
		demuxer:     flv.NewDemuxer(),
		muxer:       fmp4.NewMuxer(),
		bwriter:     bytes.NewBuffer(make([]byte, 100*1024)),
		cache:       *NewFragmentCache(),
		codecParser: parser.NewCodecParser(),
		stat:        &fmp4Status{},
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
			// get media data to be added, decide whether cut previous data
			_, isSeq, err := source.parse(p)
			if err != nil {
				log.Warning(err)
			}
			if err != nil || isSeq {
				continue
			}
			if source.btswriter != nil {
				// source.stat.update(p.IsVideo, p.TimeStamp)
				// source.calcPtsDts(p.IsVideo, p.TimeStamp, uint32(compositionTime))
				// source.tsMux(p)
				source.stat.lastTimestamp = int64(p.TimeStamp) //for calculating last sample duration
				source.fmp4Mux(p)                              // add media data into btswriter
			}

		} else {
			return fmt.Errorf("packet queue closed")
		}
	}

	return nil
}

//到这里rtmp header(flv tag header)，flv tag已经解析好，packet.data中可能有
//1.264的nalu 2.aac数据 3.264的sequence
//todo：将nalu写进p.Data便于后续写入mdat, 根据当前帧判断cut
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
			return compositionTime, true, source.codecParser.Parse(p, source.bwriter)
		}
	} else {
		ah = p.Header.(av.AudioPacketHeader)
		if ah.SoundFormat() != av.SOUND_AAC {
			return compositionTime, false, ErrNoSupportAudioCodec
		}
		if ah.AACPacketType() == av.AAC_SEQHDR {
			return compositionTime, true, source.codecParser.Parse(p, source.bwriter)
		}
	}
	source.bwriter.Reset()
	//重新解析p.Data
	if err := source.codecParser.Parse(p, source.bwriter); err != nil {
		return compositionTime, false, err
	}
	p.Data = source.bwriter.Bytes()
	//切片条件
	if p.IsVideo && vh.IsKeyFrame() {
		source.cut()
	}
	return compositionTime, false, nil
}

func (source *Fmp4Source) cut() {

}

func (source *Fmp4Source) fmp4Mux(p *av.Packet) error {
	if p.IsVideo {
		return source.muxer.Mux(p, source.btswriter)
	} else {
		// source.cache.Cache(p.Data, source.pts)
		// return source.muxAudio(cache_max_frames)
	}
}
