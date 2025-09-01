package client

import (
	"bytes"
	"encoding/hex"
	"errors"
	"fmt"
	"net"
	"strconv"
	"strings"
	"sync"
	"time"
)

type KVMNetConfig struct {
	IP   string
	Port int
	Mask string
	GW   string
}

type Client struct {
	addr  string
	mu    sync.Mutex
	getTO time.Duration
	setTO time.Duration
}

func New(ip string, port int, getTO, setTO time.Duration) *Client {
	return &Client{
		addr:  net.JoinHostPort(ip, strconv.Itoa(port)),
		getTO: getTO,
		setTO: setTO,
	}
}

func (c *Client) SetTarget(ip string, port int, getTO, setTO time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.addr = net.JoinHostPort(ip, strconv.Itoa(port))
	c.getTO = getTO
	c.setTO = setTO
}

/* Binary protocol: input/status */

func findFrames(buf []byte) [][]byte {
	var frames [][]byte
	for i := 0; i+5 < len(buf); i++ {
		if buf[i] == 0xAA && buf[i+1] == 0xBB && buf[i+2] == 0x03 && buf[i+5] == 0xEE {
			frames = append(frames, buf[i:i+6])
			i += 4
		}
	}
	return frames
}

func (c *Client) txrx(cmd, arg byte, totalDeadline time.Duration) ([]byte, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	d := net.Dialer{Timeout: totalDeadline}
	conn, err := d.Dial("tcp", c.addr)
	if err != nil {
		return nil, err
	}
	defer conn.Close()

	frame := []byte{0xAA, 0xBB, 0x03, cmd, arg, 0xEE}
	if _, err := conn.Write(frame); err != nil {
		return nil, err
	}

	deadline := time.Now().Add(totalDeadline)
	var buf []byte
	tmp := make([]byte, 256)

	for {
		if time.Now().After(deadline) {
			return buf, nil
		}
		_ = conn.SetReadDeadline(time.Now().Add(100 * time.Millisecond))
		n, err := conn.Read(tmp)
		if n > 0 {
			buf = append(buf, tmp[:n]...)
			if len(findFrames(buf)) > 0 {
				return buf, nil
			}
		}
		if err != nil {
			time.Sleep(15 * time.Millisecond)
		}
		if len(buf) > 4096 {
			return buf, nil
		}
	}
}

func scanActiveFrom(buf []byte) (int, bool) {
	if frames := findFrames(buf); len(frames) > 0 {
		for _, f := range frames {
			if len(f) == 6 && f[3] == 0x11 {
				return int(f[4]) + 1, true
			}
		}
	}
	for i := 0; i+4 < len(buf); i++ {
		if buf[i] == 0xAA && buf[i+1] == 0xBB && buf[i+2] == 0x03 && buf[i+3] == 0x11 {
			return int(buf[i+4]) + 1, true
		}
	}
	return 0, false
}

func (c *Client) GetActiveInput() (int, error) {
	resp, err := c.txrx(0x10, 0x00, c.getTO)
	if err != nil {
		return 0, err
	}
	if p, ok := scanActiveFrom(resp); ok {
		return p, nil
	}
	resp2, _ := c.txrx(0x10, 0x00, c.getTO)
	if p, ok := scanActiveFrom(resp2); ok {
		return p, nil
	}
	return 0, fmt.Errorf("no active-input reply in %s", strings.ToUpper(hex.EncodeToString(resp)))
}

func (c *Client) SetInput(n int) error {
	if n < 1 || n > 16 {
		return fmt.Errorf("input out of range: %d", n)
	}
	if _, err := c.txrx(0x01, byte(n), c.setTO); err == nil {
		return nil
	}
	_, err := c.txrx(0x11, byte(n-1), c.setTO)
	return err
}

func (c *Client) SetBuzzer(enabled bool) error {
	var v byte
	if enabled {
		v = 0x01
	}
	_, err := c.txrx(0x02, v, c.setTO)
	return err
}
func (c *Client) SetLEDTimeoutOff() error { _, err := c.txrx(0x03, 0x00, c.setTO); return err }
func (c *Client) SetLEDTimeout10s() error { _, err := c.txrx(0x03, 0x0A, c.setTO); return err }
func (c *Client) SetLEDTimeout30s() error { _, err := c.txrx(0x03, 0x1E, c.setTO); return err }

func (c *Client) Ping() error {
	_, err := c.GetActiveInput()
	return err
}

/* Raw hex sender */

func (c *Client) RawHexSend(hexstr string, deadline time.Duration) (string, error) {
	hexstr = strings.ReplaceAll(hexstr, " ", "")
	if hexstr == "" {
		return "", errors.New("empty hex")
	}
	payload, err := hex.DecodeString(hexstr)
	if err != nil {
		return "", fmt.Errorf("invalid hex: %w", err)
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	d := net.Dialer{Timeout: deadline}
	conn, err := d.Dial("tcp", c.addr)
	if err != nil {
		return "", err
	}
	defer conn.Close()

	if _, err := conn.Write(payload); err != nil {
		return "", err
	}

	var buf []byte
	tmp := make([]byte, 256)
	end := time.Now().Add(deadline)
	for {
		if time.Now().After(end) {
			break
		}
		_ = conn.SetReadDeadline(time.Now().Add(100 * time.Millisecond))
		n, er := conn.Read(tmp)
		if n > 0 {
			buf = append(buf, tmp[:n]...)
		}
		if er != nil {
			time.Sleep(15 * time.Millisecond)
		}
		if len(buf) > 4096 {
			break
		}
	}
	return strings.ToUpper(hex.EncodeToString(buf)), nil
}

/* ASCII LAN network config (IP) */

func (c *Client) sendAsciiOnce(pkt string, deadline time.Duration) ([]byte, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	d := net.Dialer{Timeout: deadline}
	conn, err := d.Dial("tcp", c.addr)
	if err != nil {
		return nil, err
	}
	defer conn.Close()

	if _, err := conn.Write([]byte(pkt)); err != nil {
		return nil, err
	}

	_ = conn.SetReadDeadline(time.Now().Add(deadline))
	var out []byte
	buf := make([]byte, 256)
	for {
		n, er := conn.Read(buf)
		if n > 0 {
			out = append(out, buf[:n]...)
		}
		if er != nil {
			break
		}
		if len(out) > 2048 {
			break
		}
	}
	return out, nil
}

func (c *Client) sendAsciiUntilTerm(pkt string, deadline time.Duration, term byte) (string, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	d := net.Dialer{Timeout: deadline}
	conn, err := d.Dial("tcp", c.addr)
	if err != nil {
		return "", err
	}
	defer conn.Close()

	if _, err := conn.Write([]byte(pkt)); err != nil {
		return "", err
	}

	end := time.Now().Add(deadline)
	var out []byte
	buf := make([]byte, 256)
	for {
		if time.Now().After(end) {
			break
		}
		_ = conn.SetReadDeadline(time.Now().Add(200 * time.Millisecond))
		n, er := conn.Read(buf)
		if n > 0 {
			out = append(out, buf[:n]...)
			if bytes.IndexByte(out, term) >= 0 {
				break
			}
		}
		if er != nil {
			time.Sleep(20 * time.Millisecond)
		}
		if len(out) > 2048 {
			break
		}
	}
	s := strings.ReplaceAll(string(out), "\x00", "")
	s = strings.ReplaceAll(s, "\r", "")
	s = strings.ReplaceAll(s, "\n", "")
	if cut := strings.IndexByte(s, term); cut >= 0 {
		s = s[:cut+1]
	}
	return s, nil
}

func keepDigitsDots(s string) string {
	var b strings.Builder
	for _, r := range s {
		if (r >= '0' && r <= '9') || r == '.' {
			b.WriteRune(r)
		}
	}
	return b.String()
}
func keepDigits(s string) string {
	var b strings.Builder
	for _, r := range s {
		if r >= '0' && r <= '9' {
			b.WriteRune(r)
		}
	}
	return b.String()
}

func (c *Client) GetNetworkConfigASCII() (KVMNetConfig, error) {
	readField := func(q, prefix string) (string, error) {
		s, err := c.sendAsciiUntilTerm(q, 2*time.Second, ';')
		if err != nil {
			return "", err
		}
		s = strings.TrimSpace(s)
		s = strings.TrimSuffix(s, ";")
		s = strings.TrimPrefix(s, prefix)
		return s, nil
	}

	ipRaw, err := readField("IP?", "IP:")
	if err != nil {
		return KVMNetConfig{}, err
	}
	ptRaw, err := readField("PT?", "PT:")
	if err != nil {
		return KVMNetConfig{}, err
	}
	maskRaw, err := readField("MA?", "MA:")
	if err != nil {
		return KVMNetConfig{}, err
	}
	gwRaw, err := readField("GW?", "GW:")
	if err != nil {
		return KVMNetConfig{}, err
	}

	depad := func(s string) string {
		parts := strings.Split(keepDigitsDots(s), ".")
		for i := range parts {
			if n, e := strconv.Atoi(parts[i]); e == nil {
				parts[i] = strconv.Itoa(n)
			}
		}
		return strings.Join(parts, ".")
	}
	ip := depad(ipRaw)
	mask := depad(maskRaw)
	gw := depad(gwRaw)

	ptDigits := keepDigits(ptRaw)
	if ptDigits == "" {
		return KVMNetConfig{}, fmt.Errorf("bad port reply: %q", ptRaw)
	}
	port, err := strconv.Atoi(ptDigits)
	if err != nil || port <= 0 || port > 65535 {
		return KVMNetConfig{}, fmt.Errorf("bad port: %v", ptDigits)
	}

	return KVMNetConfig{IP: ip, Port: port, Mask: mask, GW: gw}, nil
}

func (c *Client) SetNetworkConfigASCII(ip string, port int, mask, gw string) error {
	seq := []string{
		fmt.Sprintf("IP:%s;", ip),
		fmt.Sprintf("PT:%d;", port),
		fmt.Sprintf("MA:%s;", mask),
		fmt.Sprintf("GW:%s;", gw),
	}
	for _, pkt := range seq {
		b, err := c.sendAsciiOnce(pkt, 2*time.Second)
		if err != nil {
			return err
		}
		resp := strings.TrimSpace(strings.ReplaceAll(string(b), "\x00", ""))
		if resp != "" && !strings.Contains(resp, "OK") {
			return fmt.Errorf("unexpected reply to %q: %q", pkt, resp)
		}
	}
	return nil
}
