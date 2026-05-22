package session

import (
	"bytes"
	"compress/zlib"
	"context"
	"fmt"
	"io"

	root "github.com/walkline/ToCloud9/apps/gateway"
	"github.com/walkline/ToCloud9/apps/gateway/packet"
	pbChar "github.com/walkline/ToCloud9/gen/characters/pb"
)

const (
	numAccountDataTypes    = 8
	globalAccountDataMask  = 0x15
	maxAccountDataSize     = 0xFFFF
	accountDataSuccessCode = 0
)

func (s *GameSession) RequestAccountData(ctx context.Context, p *packet.Packet) error {
	if s.worldSocket != nil {
		s.worldSocket.SendPacket(p)
		return nil
	}

	reader := p.Reader()
	dataType := reader.Uint32()
	if err := reader.Error(); err != nil {
		return err
	}
	if dataType >= numAccountDataTypes {
		return nil
	}

	accountData, err := s.charServiceClient.AccountDataForAccount(ctx, &pbChar.AccountDataForAccountRequest{
		Api:       root.SupportedCharServiceVer,
		AccountID: s.accountID,
		RealmID:   root.RealmID,
	})
	if err != nil {
		return err
	}

	var storedData string
	var storedTime uint32
	for _, item := range accountData.AccountData {
		if item.Type == dataType {
			storedData = item.Data
			storedTime = uint32(item.Time)
			break
		}
	}

	compressedData, err := compressAccountDataPayload(storedData)
	if err != nil {
		return err
	}

	resp := packet.NewWriterWithSize(packet.SMsgUpdateAccountData, uint32(8+4+4+4+len(compressedData)))
	resp.Uint64(0)
	resp.Uint32(dataType)
	resp.Uint32(storedTime)
	resp.Uint32(uint32(len(storedData)))
	resp.Bytes(compressedData)
	s.gameSocket.Send(resp)
	return nil
}

func (s *GameSession) UpdateAccountData(ctx context.Context, p *packet.Packet) error {
	if s.worldSocket != nil {
		s.worldSocket.SendPacket(p)
		return nil
	}

	reader := p.Reader()
	dataType := reader.Uint32()
	timestamp := reader.Uint32()
	decompressedSize := reader.Uint32()
	if err := reader.Error(); err != nil {
		return err
	}
	if dataType >= numAccountDataTypes {
		return nil
	}
	if decompressedSize > maxAccountDataSize {
		return nil
	}

	data := ""
	if decompressedSize > 0 {
		compressedData, err := io.ReadAll(reader.RawReader())
		if err != nil {
			return err
		}

		decompressedData, err := decompressAccountDataPayload(compressedData)
		if err != nil {
			return err
		}
		if len(decompressedData) != int(decompressedSize) {
			return fmt.Errorf("account data decompressed size mismatch: got %d, expected %d", len(decompressedData), decompressedSize)
		}
		data = string(trimAccountDataCString(decompressedData))
	} else {
		timestamp = 0
	}

	if globalAccountDataMask&(uint32(1)<<dataType) != 0 {
		_, err := s.charServiceClient.UpdateAccountDataForAccount(ctx, &pbChar.UpdateAccountDataForAccountRequest{
			Api:       root.SupportedCharServiceVer,
			AccountID: s.accountID,
			RealmID:   root.RealmID,
			Type:      dataType,
			Time:      int64(timestamp),
			Data:      data,
		})
		if err != nil {
			return err
		}
	}

	s.sendAccountDataUpdateComplete(dataType)
	return nil
}

func (s *GameSession) sendAccountDataUpdateComplete(dataType uint32) {
	resp := packet.NewWriterWithSize(packet.SMsgUpdateAccountDataComplete, 8)
	resp.Uint32(dataType)
	resp.Uint32(accountDataSuccessCode)
	s.gameSocket.Send(resp)
}

func compressAccountDataPayload(data string) ([]byte, error) {
	var compressed bytes.Buffer
	zw := zlib.NewWriter(&compressed)
	if _, err := zw.Write([]byte(data)); err != nil {
		_ = zw.Close()
		return nil, err
	}
	if err := zw.Close(); err != nil {
		return nil, err
	}
	return compressed.Bytes(), nil
}

func decompressAccountDataPayload(data []byte) ([]byte, error) {
	zr, err := zlib.NewReader(bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	defer zr.Close()

	return io.ReadAll(zr)
}

func trimAccountDataCString(data []byte) []byte {
	if index := bytes.IndexByte(data, 0); index >= 0 {
		return data[:index]
	}
	return data
}
