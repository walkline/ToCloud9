package worldsocket

import (
	"context"
	"encoding/binary"
	"fmt"
	"net"
	"time"

	"github.com/rs/zerolog"

	"github.com/walkline/ToCloud9/apps/game-load-balancer/packet"
	"github.com/walkline/ToCloud9/apps/game-load-balancer/sockets"
)

type WorldSocket struct {
	logger           *zerolog.Logger
	conn             net.Conn
	packetsReader    *sockets.PacketsReader
	readPacketsChan  chan *packet.Packet
	writePacketsChan chan *packet.Packet
	address          string
}

func (s *WorldSocket) Close() {
	s.conn.Close()
}

func (s *WorldSocket) ListenAndProcess(ctx context.Context) error {
	closeChan := make(chan struct{}, 2)
	go func() {
		for s.packetsReader.Next() {
			s.readPacketsChan <- s.packetsReader.Packet()
		}
		close(s.readPacketsChan)
		closeChan <- struct{}{}
		closeChan <- struct{}{}
		close(closeChan)
		//close(s.writePacketsChan)
	}()

	go func() {
		for {
			select {
			case p, ok := <-s.writePacketsChan:
				if !ok {
					return
				}

				s.SendPacket(p)
			case <-closeChan:
				return
			}
		}
	}()

	select {
	case <-ctx.Done():
		s.conn.Close()
	case <-closeChan:
	}

	return nil
}

func (s *WorldSocket) SendPacket(p *packet.Packet) {
	if e := s.logger.Trace(); e.Enabled() {
		s.logger.Trace().
			Str("opcode", fmt.Sprintf("%s (0x%X)", p.Opcode.String(), uint16(p.Opcode))).
			Uint32("size", p.Size).
			Msg("📦 Balancer=>World")
	}

	header := make([]byte, 6, len(p.Data)+6)
	binary.BigEndian.PutUint16(header[0:2], uint16(len(p.Data)+4))
	binary.LittleEndian.PutUint32(header[2:6], uint32(p.Opcode))

	_, err := s.conn.Write(append(header, p.Data...))
	if err != nil {
		s.logger.Error().Err(err).Msg("can't send message to world server")
	}
}

func (s *WorldSocket) Send(p *packet.Writer) {
	s.SendPacket(&packet.Packet{
		Opcode: p.Opcode,
		Size:   uint32(p.Payload.Len()),
		Data:   p.Payload.Bytes(),
	})
}

func (s *WorldSocket) ReadChannel() <-chan *packet.Packet {
	return s.readPacketsChan
}

func (s *WorldSocket) WriteChannel() chan<- *packet.Packet {
	return s.writePacketsChan
}

func (s *WorldSocket) Address() string {
	return s.address
}

func NewWorldSocketWithAddress(logger *zerolog.Logger, addr string) (sockets.Socket, error) {
	dialer := net.Dialer{Timeout: time.Second * 5}
	net, err := dialer.Dial("tcp", addr)
	if err != nil {
		return nil, err
	}

	return &WorldSocket{
		logger:           logger,
		conn:             net,
		packetsReader:    sockets.NewPacketsReader(net, 2, packet.SourceWorldServer),
		readPacketsChan:  make(chan *packet.Packet, 100),
		writePacketsChan: make(chan *packet.Packet, 100),
		address:          addr,
	}, nil
}

func NewWorldSocketWithConnection(logger *zerolog.Logger, net net.Conn) (sockets.Socket, error) {
	return &WorldSocket{
		logger:           logger,
		conn:             net,
		packetsReader:    sockets.NewPacketsReader(net, 2, packet.SourceWorldServer),
		readPacketsChan:  make(chan *packet.Packet, 100),
		writePacketsChan: make(chan *packet.Packet, 100),
	}, nil
}
