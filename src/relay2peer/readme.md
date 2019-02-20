# Relay to peer protocol

Each listener potentially could be a relay point for listeners of the same mount point by p2p connection. So there is way to tell server about that fact for one client and to get information about such relay for another.  
The following HTTP requests are about it:

## Interaction with the managing server

### Relay point
1. Ask STUN server for your external IP and PORT
2. Send request to managing server to tell that information  
   (Keep sending this request to save the keep-alive status and get up-to-date information about the listeners until the connection with the listener is established)

```http
GET /Pi HTTP/1.0
Mount: [MountName]
MyAddr: [IP:PORT]
Latency: [latency, ms]
Flag: relay
```

### Managing server
3. Send answer to Relay with OK message and list possible listeners addresses if they are exists

```http
HTTP/1.0 200 OK
Server: [Name]/[Version]
Address: [IP:PORT:IP:PORT:IP:PORT]
```

### Listener point
4. Ask STUN server for your external IP and PORT
5. Send request to managing server to tell that you want to use relay point  
   (Keep sending this request to save the keep-alive status and get up-to-date information about the relays until the connection with the relay is established)

```http
GET /Pi HTTP/1.0
Mount: [MountName]
MyAddr: [IP:PORT]
```

### Managing server
6. Send answer to Listener with OK message and list possible relays addresses if they are exists

```http
HTTP/1.0 200 OK
Server: [Name]/[Version]
Address: [IP:PORT:IP:PORT:IP:PORT]
```

### Relay and Listener points
7. After getting candidates addresses each client should try to connect each other using UDP
8. After connection established each client should inform managing server about that

```http
GET /Pi HTTP/1.0
Connected: [IP:PORT, IP:PORT]
```


## Interaction between relay and listener points

After relay and listener know addresses of each other they have to send few handshake messages.

### Relay point
9. Send message to listener point

```http
helloListener
```

### Listener point
10. Send message to relay point

```http
helloRelay
```

### Relay point
11. After getting hello message send answer

```http
niceToMeetYouListener
```

### Listener point
12. After getting hello message send answer

```http
niceToMeetYouRelay
```

### Relay point
13. After sending answer, pack stream data to UDPMessage struct and start transmitting

```http
UDPMessage
```

UDPMessage consist of data begins from 8th byte, checksum and header marks at 00,01 and 06,07th bytes:  

        byte | 00 | 01 | 02 | 03 | 04 | 05 | 06 | 07 | 08
        ---- | -- | -- | -- | -- | -- | -- | -- | -- | --
         val | 45 | 61 |    CRC32(data)    | 61 | 45 | data ...

### Listener point
14. After sending answer, wait for UDPMessage with stream and start receiving data

```http
UDPMessage
```