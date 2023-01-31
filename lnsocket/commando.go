package lnsocket

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/lightningnetwork/lnd/lnwire"
)

// Commando message types
const (
	CommandoCmd            = 0x4c4f
	CommandoReplyContinues = 0x594b
	CommandoReplyTerm      = 0x594d
)

// CommandoMsg struct
type CommandoMsg struct {
	Rune      string
	Method    string
	Params    string
	RequestID uint64
}

// NewCommandoMsg creates a new commando message
func NewCommandoMsg(token string, method string, params string) CommandoMsg {
	return CommandoMsg{
		Rune:   token,
		Method: method,
		Params: params,
	}
}

// A compile time check to ensure Init implements the lnwire.Message
// interface.

// MsgType API
func (msg *CommandoMsg) MsgType() lnwire.MessageType {
	return CommandoCmd
}

// Decode API
func (msg *CommandoMsg) Decode(reader io.Reader, size uint32) error {
	return fmt.Errorf("implement commando decode?")
}

// Encode API
func (msg *CommandoMsg) Encode(buf *bytes.Buffer, pver uint32) error {
	if err := lnwire.WriteUint64(buf, msg.RequestID); err != nil {
		return err
	}

	buf.WriteString("{\"method\": \"")
	buf.WriteString(msg.Method)
	buf.WriteString("\",\"params\":")
	buf.WriteString(msg.Params)
	buf.WriteString(",\"rune\":\"")
	buf.WriteString(msg.Rune)
	buf.WriteString("\"}")

	return nil
}

// CommandoReadAll reads all commando responses
func (ln *LN) CommandoReadAll() (string, error) {
	all := []byte{}

	start := time.Now()

	for time.Now().Before(start.Add(ln.Timeout)) {
		msgtype, res, err := ln.Read()
		if err != nil {
			return "", err
		}
		switch msgtype {
		case CommandoReplyContinues:
			all = append(all, res[8:]...)
			continue
		case CommandoReplyTerm:
			all = append(all, res[8:]...)
			return string(all), nil
		default:
			continue
		}
	}

	return "", os.ErrDeadlineExceeded
}

// NewCommandoReader invokes a command and retruns a reader to read reply
func (ln *LN) NewCommandoReader(ctx context.Context, rune, serviceMethod, params string) (io.Reader, error) {
	commando := NewCommandoMsg(rune, serviceMethod, params)

	var b bytes.Buffer
	_, err := lnwire.WriteMessage(&b, &commando, 0)
	if err != nil {
		return nil, err
	}

	_, err = ln.Write(b.Bytes())
	if err != nil {
		return nil, err
	}

	reader, writer := io.Pipe()
	w := bufio.NewWriter(writer)

	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			default:
				// Do nothing
			}

			msgtype, res, err := ln.Read()
			if err != nil {
				writer.CloseWithError(err)
			}
			switch msgtype {
			case CommandoReplyContinues:
				w.Write(res[8:])
				continue
			case CommandoReplyTerm:
				w.Write(res[8:])
				w.Flush()
				writer.Close()
				return
			default:
				continue
			}
		}
	}()

	return bufio.NewReader(reader), nil
}
