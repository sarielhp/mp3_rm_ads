package message_info

import (
	"encoding/binary"
	"fmt"
	"io"
	"os"
)

type Compact struct {
	Date      uint32
	Flags     uint8
	SpamScore float32
}

const (
	FlagIsRead uint8 = 1 << iota
	FlagHasICS
	FlagIsSpam
	FlagIsPolitical
	FlagIsBlacklisted
	FlagClassified
)

func (c Compact) IsRead() bool        { return c.Flags&FlagIsRead != 0 }
func (c Compact) HasICS() bool        { return c.Flags&FlagHasICS != 0 }
func (c Compact) IsSpam() bool        { return c.Flags&FlagIsSpam != 0 && c.Classified() }
func (c Compact) IsPolitical() bool   { return c.Flags&FlagIsPolitical != 0 }
func (c Compact) IsBlacklisted() bool { return c.Flags&FlagIsBlacklisted != 0 }
func (c Compact) Classified() bool    { return c.Flags&FlagClassified != 0 }

const (
	EntrySize = 46
	Version   = 0x01
)

func IDToBytes(id string) [36]byte {
	var b [36]byte
	n := copy(b[:], id)
	if n > 36 {
		n = 36
	}
	_ = n
	return b
}

func WriteOne(w io.Writer, id [36]byte, c Compact) error {
	buf := make([]byte, EntrySize)
	buf[0] = Version
	copy(buf[1:37], id[:])
	binary.BigEndian.PutUint32(buf[37:41], c.Date)
	buf[41] = c.Flags
	binary.BigEndian.PutUint32(buf[42:46], uint32(c.SpamScore))
	_, err := w.Write(buf)
	return err
}

func ReadAll(r io.Reader) (map[[36]byte]Compact, error) {
	result := make(map[[36]byte]Compact)
	buf := make([]byte, EntrySize)
	for {
		_, err := io.ReadFull(r, buf)
		if err == io.EOF || err == io.ErrUnexpectedEOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("reading message_info: %w", err)
		}
		if buf[0] != Version {
			continue
		}
		var id [36]byte
		copy(id[:], buf[1:37])
		date := binary.BigEndian.Uint32(buf[37:41])
		flags := buf[41]
		score := int32(binary.BigEndian.Uint32(buf[42:46]))
		result[id] = Compact{
			Date:      date,
			Flags:     flags,
			SpamScore: float32(score),
		}
	}
	return result, nil
}

func AppendToFile(path string, id [36]byte, c Compact) error {
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0600)
	if err != nil {
		return fmt.Errorf("open message_info: %w", err)
	}
	defer f.Close()
	return WriteOne(f, id, c)
}

func ReadFromFile(path string) (map[[36]byte]Compact, error) {
	f, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return make(map[[36]byte]Compact), nil
		}
		return nil, fmt.Errorf("open message_info: %w", err)
	}
	defer f.Close()
	return ReadAll(f)
}

func RewriteFile(path string, entries map[[36]byte]Compact) error {
	f, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("create message_info: %w", err)
	}
	defer f.Close()
	for id, c := range entries {
		if err := WriteOne(f, id, c); err != nil {
			return err
		}
	}
	return nil
}
