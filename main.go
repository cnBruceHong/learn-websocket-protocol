package main

import (
	"crypto/sha1"
	"encoding/base64"
	"encoding/binary"
	"errors"
	"html/template"
	"log"
	"net"
	"net/http"
	"strings"
)

/* Websocket 协议包
0                   1                   2                   3       (字节 byte)
 0 1 2 3 4 5 6 7 8 9 0 1 2 3 4 5 6 7 8 9 0 1 2 3 4 5 6 7 8 9 0 1    (比特位 bit)
+-+-+-+-+-------+-+-------------+-------------------------------+
|F|R|R|R| opcode|M| Payload len |    Extended payload length    |
|I|S|S|S|  (4)  |A|     (7)     |             (16/64)           |
|N|V|V|V|       |S|             |   (if payload len==126/127)   |
| |1|2|3|       |K|             |                               |
+-+-+-+-+-------+-+-------------+ - - - - - - - - - - - - - - - +
|     Extended payload length continued, if payload len == 127  |
+ - - - - - - - - - - - - - - - +-------------------------------+
|     .......                   |Masking-key, if MASK set to 1  |
+-------------------------------+-------------------------------+
| Masking-key (continued)       |          Payload Data         |
+-------------------------------- - - - - - - - - - - - - - - - +
:                     Payload Data continued ...                :
+ - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - +
|                     Payload Data continued ...                |
+---------------------------------------------------------------+
*/

var keyGUID = []byte("258EAFA5-E914-47DA-95CA-C5AB0DC85B11")

func computeAcceptKey(challengeKey string) string {
	h := sha1.New()
	h.Write([]byte(challengeKey))
	h.Write(keyGUID)
	return base64.StdEncoding.EncodeToString(h.Sum(nil))
}

const (
	finalBit     = 1 << 7
	maskBit      = 1 << 7
	TextMessage  = 1
	CloseMessage = 8
)

type Conn struct {
	writeBuf []byte
	maskKey  [4]byte
	conn     net.Conn
}

func maskBytes(key [4]byte, b []byte) {
	pos := 0
	for i := range b {
		b[i] ^= key[pos&3]
		pos++
	}
}

// 发送数据
func (c *Conn) SendData(data []byte) {
	length := len(data)
	c.writeBuf = make([]byte, 10+length)
	playloadStart := 2
	c.writeBuf[0] = byte(TextMessage) | finalBit

	switch {
	case length >= 65535:
		c.writeBuf[1] = byte(0x00) | 127
		binary.BigEndian.PutUint64(c.writeBuf[playloadStart:], uint64(length))
		playloadStart += 8
	case length > 125:
		c.writeBuf[1] = byte(0x00) | 126
		binary.BigEndian.PutUint16(c.writeBuf[playloadStart:], uint16(length))
		playloadStart += 2
	default:
		c.writeBuf[1] = byte(0x00) | byte(length)
	}
	copy(c.writeBuf[playloadStart:], data[:])
	c.conn.Write(c.writeBuf[:playloadStart+length])
}

// 读取数据
func (c *Conn) ReadData() (data []byte, err error) {
	var b [8]byte

	if _, err := c.conn.Read(b[:2]); err != nil {
		return nil, err
	}

	// 提取FIN位
	final := b[0]&finalBit != 0

	if !final {
		log.Println("Recived fragmented frame, not support")
		return nil, errors.New("not support fragmented message")
	}

	frameType := int(b[0] & 0xf)

	if frameType == CloseMessage {
		c.conn.Close()
		log.Println("Recived closed message, connection will be closed")
		return nil, errors.New("recived closed message")
	}

	if frameType != TextMessage {
		return nil, errors.New("only support text message")
	}

	mask := b[1]&maskBit != 0

	payloadLen := int64(b[1] & 0x7F)
	dataLen := int64(payloadLen)

	// 根据payload length 判断数据的真实长度
	switch payloadLen {
	case 126:
		if _, err := c.conn.Read(b[:2]); err != nil {
			return nil, err
		}
		dataLen = int64(binary.BigEndian.Uint16(b[:2]))
	case 127:
		if _, err := c.conn.Read(b[:8]); err != nil {
			return nil, err
		}
		dataLen = int64(binary.BigEndian.Uint64(b[:8]))
	}

	log.Printf("Read data length: %d, payload length %d", payloadLen, dataLen)

	// 读取 mask key
	if mask {
		if _, err := c.conn.Read(c.maskKey[:]); err != nil {
			return nil, err
		}
	}

	// 读取数据内容
	p := make([]byte, dataLen)
	if _, err := c.conn.Read(p); err != nil {
		return nil, err
	}
	if mask {
		maskBytes(c.maskKey, p)
	}

	return p, nil
}

// 协议从http上升到websocket
func upgrade(w http.ResponseWriter, r *http.Request) (c *Conn, err error) {

	/*
		    一个ws request 请求的格式

			Accept-Encoding:gzip, deflate, br
			Accept-Language:zh-CN,zh;q=0.8,en;q=0.6,zh-TW;q=0.4
			Cache-Control:no-cache
			Connection:Upgrade
			Cookie:olfsk=olfsk1987334287312954; hblid=QOfjOXpze8nJoC873m39N0H8RE0Qt2wb
			DNT:1
			Host:127.0.0.1:8080
			Origin:http://localhost:8080
			Pragma:no-cache
			Sec-WebSocket-Extensions:permessage-deflate; client_max_window_bits
			Sec-WebSocket-Key:CkBSTPOI0xXQjX+cfLKYLQ==
			Sec-WebSocket-Version:13
			Upgrade:websocket
			User-Agent:Mozilla/5.0 (Macintosh; Intel Mac OS X 10_13_0) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/61.0.3163.100 Safari/537.36
	*/

	if r.Method != "GET" {
		http.Error(w, http.StatusText(http.StatusMethodNotAllowed), http.StatusMethodNotAllowed)
		return nil, errors.New("websocket: method not GET")
	}

	// 判断请求头中 Sec-Websocket-Version 是否为 13
	if value := r.Header["Sec-Websocket-Version"]; len(value) == 0 || value[0] != "13" {
		http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
		return nil, errors.New("websocket: version != 13")
	}

	// 判断请求头中的Connention
	if !tokenListContainsValue(r.Header, "Connection", "upgrade") {
		http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
		return nil, errors.New("websocket: could not find connection header with token 'upgrade'")
	}

	if !tokenListContainsValue(r.Header, "Upgrade", "websocket") {
		http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
		return nil, errors.New("websocket: could not find connection header with token 'websocket'")
	}

	challengeKey := r.Header.Get("Sec-Websocket-Key")

	if challengeKey == "" {
		http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
		return nil, errors.New("websocket: key missing or blank")
	}

	h, ok := w.(http.Hijacker)

	if !ok {
		http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
		return nil, errors.New("websocket: response dose not implement http.Hijacker")
	}

	conn, rw, err := h.Hijack()

	if err != nil {
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return nil, err
	}

	br := rw.Reader

	if br.Buffered() > 0 {
		conn.Close()
		return nil, errors.New("websocket: client sent data before handshake is complete")
	}

	p := []byte{}
	p = append(p,
		"HTTP/1.1 101 Switching Protocols\r\n"+ // 返回http 101 状态码切换协议
			"Upgrade: websocket\r\n"+
			"Connection: Upgrade\r\n"+
			"Sec-WebSocket-Accept: "+computeAcceptKey(challengeKey)+
			"\r\n\r\n"...)

	if _, err := conn.Write(p); err != nil {
		conn.Close()
		return nil, err
	}

	log.Println("Upgrade http to websocket successfully")

	// 实例化我们定义的数据对象
	newConn := &Conn{conn: conn}

	return newConn, nil
}

func tokenListContainsValue(headers http.Header, field string, value string) bool {
	return strings.ToLower(headers.Get(field)) == value
}

// index 页面处理器
func index(w http.ResponseWriter, r *http.Request) {
	if t, err := template.ParseFiles("index.html"); err != nil {
		http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
		log.Println("载入页面失败")
	} else {
		t.Execute(w, nil)
	}
}

// 回声函数
func echo(w http.ResponseWriter, r *http.Request) {

	// 协议升级
	c, err := upgrade(w, r)

	if err != nil {
		log.Print("Upgrade error:", err)
		return
	}

	defer c.conn.Close()

	for {
		message, err := c.ReadData()
		if err != nil {
			log.Println("read:", err)
			break
		}
		log.Printf("recv: %s", message)
		c.SendData(message)
	}
}

func main() {
	log.SetFlags(1)
	http.HandleFunc("/", index)
	http.HandleFunc("/echo", echo)
	log.Fatal(http.ListenAndServe("0.0.0.0:8080", nil))
}
