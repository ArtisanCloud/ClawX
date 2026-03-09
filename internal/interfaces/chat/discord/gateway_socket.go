package discord

import (
	"bufio"
	"context"
	"crypto/rand"
	"crypto/sha1"
	"crypto/tls"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
)

const (
	websocketOpcodeContinuation = 0x0
	websocketOpcodeText         = 0x1
	websocketOpcodeClose        = 0x8
	websocketOpcodePing         = 0x9
	websocketOpcodePong         = 0xA
)

type gatewaySocket struct {
	conn   net.Conn
	reader *bufio.Reader
	mu     sync.Mutex
}

func dialGateway(ctx context.Context, rawURL string) (*gatewaySocket, error) {
	parsed, err := url.Parse(strings.TrimSpace(rawURL))
	if err != nil {
		return nil, err
	}
	if parsed.Scheme != "wss" && parsed.Scheme != "ws" {
		return nil, fmt.Errorf("unsupported discord gateway scheme: %s", parsed.Scheme)
	}

	host := parsed.Hostname()
	port := parsed.Port()
	if port == "" {
		if parsed.Scheme == "wss" {
			port = "443"
		} else {
			port = "80"
		}
	}

	address := net.JoinHostPort(host, port)
	dialer := net.Dialer{Timeout: 10 * time.Second}
	var conn net.Conn
	if parsed.Scheme == "wss" {
		conn, err = tls.DialWithDialer(&dialer, "tcp", address, &tls.Config{ServerName: host})
	} else {
		conn, err = dialer.DialContext(ctx, "tcp", address)
	}
	if err != nil {
		return nil, err
	}
	if err := conn.SetDeadline(time.Now().Add(10 * time.Second)); err != nil {
		_ = conn.Close()
		return nil, err
	}
	defer conn.SetDeadline(time.Time{})

	reader := bufio.NewReader(conn)
	keyBytes := make([]byte, 16)
	if _, err := rand.Read(keyBytes); err != nil {
		_ = conn.Close()
		return nil, err
	}
	key := base64.StdEncoding.EncodeToString(keyBytes)

	path := parsed.RequestURI()
	if path == "" {
		path = "/"
	}

	hostHeader := parsed.Host
	if hostHeader == "" {
		hostHeader = address
	}

	request := fmt.Sprintf("GET %s HTTP/1.1\r\nHost: %s\r\nUpgrade: websocket\r\nConnection: Upgrade\r\nSec-WebSocket-Version: 13\r\nSec-WebSocket-Key: %s\r\nUser-Agent: synapsex-discord-gateway\r\n\r\n", path, hostHeader, key)
	if _, err := io.WriteString(conn, request); err != nil {
		_ = conn.Close()
		return nil, err
	}

	req, _ := http.NewRequest(http.MethodGet, rawURL, nil)
	resp, err := http.ReadResponse(reader, req)
	if err != nil {
		_ = conn.Close()
		return nil, err
	}
	if resp.StatusCode != http.StatusSwitchingProtocols {
		_ = conn.Close()
		return nil, fmt.Errorf("discord gateway handshake failed: %s", resp.Status)
	}

	expectedAccept := websocketAccept(key)
	if strings.TrimSpace(resp.Header.Get("Sec-WebSocket-Accept")) != expectedAccept {
		_ = conn.Close()
		return nil, errors.New("discord gateway handshake returned invalid accept key")
	}

	return &gatewaySocket{conn: conn, reader: reader}, nil
}

func (s *gatewaySocket) Close() error {
	if s == nil || s.conn == nil {
		return nil
	}
	return s.conn.Close()
}

func (s *gatewaySocket) WriteJSON(ctx context.Context, payload any) error {
	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	return s.writeFrame(ctx, websocketOpcodeText, body)
}

func (s *gatewaySocket) ReadText(ctx context.Context) ([]byte, error) {
	var message []byte
	inFragment := false

	for {
		fin, opcode, payload, err := s.readFrame(ctx)
		if err != nil {
			return nil, err
		}

		switch opcode {
		case websocketOpcodePing:
			if err := s.writeFrame(ctx, websocketOpcodePong, payload); err != nil {
				return nil, err
			}
			continue
		case websocketOpcodePong:
			continue
		case websocketOpcodeClose:
			return nil, io.EOF
		case websocketOpcodeText:
			if fin {
				return payload, nil
			}
			message = append(message[:0], payload...)
			inFragment = true
		case websocketOpcodeContinuation:
			if !inFragment {
				continue
			}
			message = append(message, payload...)
			if fin {
				return message, nil
			}
		default:
			continue
		}
	}
}

func (s *gatewaySocket) readFrame(ctx context.Context) (bool, byte, []byte, error) {
	for {
		if err := setReadDeadline(ctx, s.conn); err != nil {
			return false, 0, nil, err
		}

		header := make([]byte, 2)
		if _, err := io.ReadFull(s.reader, header); err != nil {
			if isTimeout(err) && ctx.Err() == nil {
				continue
			}
			return false, 0, nil, err
		}

		fin := header[0]&0x80 != 0
		opcode := header[0] & 0x0F
		masked := header[1]&0x80 != 0
		payloadLen := int(header[1] & 0x7F)

		switch payloadLen {
		case 126:
			extended := make([]byte, 2)
			if _, err := io.ReadFull(s.reader, extended); err != nil {
				return false, 0, nil, err
			}
			payloadLen = int(binary.BigEndian.Uint16(extended))
		case 127:
			extended := make([]byte, 8)
			if _, err := io.ReadFull(s.reader, extended); err != nil {
				return false, 0, nil, err
			}
			payloadLen64 := binary.BigEndian.Uint64(extended)
			if payloadLen64 > 16*1024*1024 {
				return false, 0, nil, errors.New("discord gateway frame too large")
			}
			payloadLen = int(payloadLen64)
		}

		var maskKey []byte
		if masked {
			maskKey = make([]byte, 4)
			if _, err := io.ReadFull(s.reader, maskKey); err != nil {
				return false, 0, nil, err
			}
		}

		payload := make([]byte, payloadLen)
		if payloadLen > 0 {
			if _, err := io.ReadFull(s.reader, payload); err != nil {
				return false, 0, nil, err
			}
		}
		if masked {
			for i := range payload {
				payload[i] ^= maskKey[i%4]
			}
		}

		return fin, opcode, payload, nil
	}
}

func (s *gatewaySocket) writeFrame(ctx context.Context, opcode byte, payload []byte) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if err := setWriteDeadline(ctx, s.conn); err != nil {
		return err
	}

	frame := make([]byte, 0, len(payload)+14)
	frame = append(frame, 0x80|opcode)

	payloadLen := len(payload)
	switch {
	case payloadLen <= 125:
		frame = append(frame, 0x80|byte(payloadLen))
	case payloadLen <= 65535:
		frame = append(frame, 0x80|126)
		extended := make([]byte, 2)
		binary.BigEndian.PutUint16(extended, uint16(payloadLen))
		frame = append(frame, extended...)
	default:
		frame = append(frame, 0x80|127)
		extended := make([]byte, 8)
		binary.BigEndian.PutUint64(extended, uint64(payloadLen))
		frame = append(frame, extended...)
	}

	maskKey := make([]byte, 4)
	if _, err := rand.Read(maskKey); err != nil {
		return err
	}
	frame = append(frame, maskKey...)

	maskedPayload := make([]byte, payloadLen)
	for i := range payload {
		maskedPayload[i] = payload[i] ^ maskKey[i%4]
	}
	frame = append(frame, maskedPayload...)

	_, err := s.conn.Write(frame)
	return err
}

func websocketAccept(key string) string {
	hash := sha1.Sum([]byte(strings.TrimSpace(key) + "258EAFA5-E914-47DA-95CA-C5AB0DC85B11"))
	return base64.StdEncoding.EncodeToString(hash[:])
}

func setReadDeadline(ctx context.Context, conn net.Conn) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	deadline := time.Now().Add(30 * time.Second)
	if current, ok := ctx.Deadline(); ok && current.Before(deadline) {
		deadline = current
	}
	return conn.SetReadDeadline(deadline)
}

func setWriteDeadline(ctx context.Context, conn net.Conn) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	deadline := time.Now().Add(10 * time.Second)
	if current, ok := ctx.Deadline(); ok && current.Before(deadline) {
		deadline = current
	}
	return conn.SetWriteDeadline(deadline)
}

func isTimeout(err error) bool {
	var netErr net.Error
	return errors.As(err, &netErr) && netErr.Timeout()
}
