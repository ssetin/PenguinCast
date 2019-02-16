# Relay to peer protocol

Each listener potentially could be a relay point for listeners of the same mount point by p2p connection. So there is way to tell server about that fact for one client and to get information about such relay for another.  
The following HTTP requests are about it:

### Relay point
1. Ask STUN server for your external IP and PORT
2. Send request to PenguinCast to tell that information  
   (Keep sending this request to save the keep-alive status and get up-to-date information about the listeners until the connection with the listener is established)

```http
GET /Pi HTTP/1.0
Mount: [MountName]
MyAddr: [IP:PORT]
Latency: [latency, ms]
Flag: relay
```

### PenguinCast
3. Send answer to Relay with OK message and list possible listeners addresses if they are exists

```http
HTTP/1.0 200 OK
Server: [Name]/[Version]
Address: [IP:PORT:IP:PORT:IP:PORT]
```

### Listener point
4. Ask STUN server for your external IP and PORT
5. Send request to PenguinCast to tell that you want to use relay point  
   (Keep sending this request to save the keep-alive status and get up-to-date information about the relays until the connection with the relay is established)

```http
GET /Pi HTTP/1.0
Mount: [MountName]
MyAddr: [IP:PORT]
```

### PenguinCast
6. Send answer to Listener with OK message and list possible relays addresses if they are exists

```http
HTTP/1.0 200 OK
Server: [Name]/[Version]
Address: [IP:PORT:IP:PORT:IP:PORT]
```

### Relay and Listener points
7. After getting candidates addresses each client should try to connect each other using UDP
8. After connection established each client should inform PenguinCast about that

```http
GET /Pi HTTP/1.0
Connected: [IP:PORT, IP:PORT]
```