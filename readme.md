# PenguinCast
Under construction

Icecast compatible streaming audio server, which can be used as a server part of your internet radio station.

## Capabilities
* Recieving stream from Source
* Sending stream to clients
* Operating with shoutcast metadata
* Collecting statistics at access.log
* Html and json interface for accessing server status (__http://host:port/info.html__ and __http://host:port/info.json__)
* Real time monitoring (__http://host:port/monitor.html__)

## Configuring
Configuration parameters are stored in config.json.

```json
{
    "Name": "Rollstation radio",
    "Admin": "admin@site.com",
    "Location": "Saint Petersburg",
    "Host": "http://127.0.0.1",
    "Socket": { "Port": 8008},
    "Limits": {
        "Clients": 30, "Sources": 5, 
        "SourceIdleTimeOut": 5, "EmptyBufferIdleTimeOut": 5
    },
    "Auth": {"AdminPassword": "admin"},
    "Paths": {"Log": "log/", "Web":"html/"},
    "Logging": {"Loglevel": 4, "Logsize": 50000, "UseMonitor": true, "UseStat": true},
    "Mounts": [
        {
            "Name": "RockRadio96", "User":"admin", "Password": "admin", "Genre":"Rock", 
            "Description": "Rock radio station, Saint Petersburg", "BitRate":96, 
            "BurstSize": 65536, "DumpFile": "rock96.mp3"
        }
    ]
}
```

#### Socket
- Port - the TCP port that will be used to accept client connections

#### Limits
- Clients - maximum clients per server
- Sources - maximum Sources per server
- SourceIdleTimeOut - data timeout for source
- EmptyBufferIdleTimeOut - silence timeout for client

#### Mounts
- Name - required, mount point name
- User - required, user name for source
- Password - required, password for source
- Genre - optional, Genre
- Description - optional, stream description
- BitRate - optional, stream bitrate
- BurstSize - number of bytes to collect before send to client on start streaming
- DumpFile - optional, detect filename in which audio data from source will be stored

#### Logging
- Loglevel - determine what will be stored in error.log 
    - 1 - Errors
    - 2 - Warning
    - 3 - Info
    - 4 - Debug