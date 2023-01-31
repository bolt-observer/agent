package lnsocket

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"io"

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

	buf.WriteString(fmt.Sprintf(`{"method": "%s","params": %s, "rune": "%s"}`, msg.Method, msg.Params, msg.Rune))

	return nil
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

// CommandoReadAll reads complete commando response as string - used with internal lib
func (ln *LN) CommandoReadAll(ctx context.Context, rune, serviceMethod, params string) (string, error) {
	reader, err := ln.NewCommandoReader(ctx, rune, serviceMethod, params)
	if err != nil {
		return "", err
	}

	buf := new(bytes.Buffer)
	buf.ReadFrom(reader)
	return buf.String(), nil
}
