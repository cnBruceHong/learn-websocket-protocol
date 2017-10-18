# learn-websocket-protocol
用Go语言实现了服务端的websocket协议

# websocket协议 数据帧

 0                   1                   2                   3
 0 1 2 3 4 5 6 7 8 9 0 1 2 3 4 5 6 7 8 9 0 1 2 3 4 5 6 7 8 9 0 1
+-+-+-+-+-------+-+-------------+-------------------------------+
|F|R|R|R| opcode|M| Payload len |    Extended payload length    |
|I|S|S|S|  (4)  |A|     (7)     |             (16/64)           |
|N|V|V|V|       |S|             |   (if payload len==126/127)   |
| |1|2|3|       |K|             |                               |
+-+-+-+-+-------+-+-------------+ - - - - - - - - - - - - - - - +
|     Extended payload length continued, if payload len == 127  |
+ - - - - - - - - - - - - - - - +-------------------------------+
|                               |Masking-key, if MASK set to 1  |
+-------------------------------+-------------------------------+
| Masking-key (continued)       |          Payload Data         |
+-------------------------------- - - - - - - - - - - - - - - - +
:                     Payload Data continued ...                :
+ - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - +
|                     Payload Data continued ...                |
+---------------------------------------------------------------+

# ws 请求

一个ws request 请求的格式

```
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
```

# 服务端返回的response

```
Connection:Upgrade
Sec-WebSocket-Accept:JgYliGM2qJBfM/AeSAbgus3dS4E=
Upgrade:websocket
```
