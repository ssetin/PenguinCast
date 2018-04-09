# PenguinCast

Streaming audio server
Under construction


## Configuring

Configuration parameters are stored in config.json.

```json
{
    "Name": "Rollstation radio",
    "Admin": "admin@penguin.com",
    "Location": "Saint Petersburg",
    "Host": "http://127.0.0.1",
    "Socket": { "Port": 8008},
    "Limits": {"Clients": 30, "Sources": 5, "SourceIdleTimeOut": 5,"EmptyBufferIdleTimeOut": 5, "MaxBufferSize":20},
    "Auth": {"AdminPassword": "admin"},
    "Paths": {"Log": "log/", "Web":"html/"},
    "Logging": {"Loglevel": 4, "Logsize": 50000},
    "Mounts": [
        {"Name": "RockRadio96", "User":"admin", "Password": "admin", "Genre":"Rock", 
            "Description": "Rock radio station, Saint Petersburg", "BitRate":96, "BurstSize": 65536, "DumpFile": "rock96.mp3"
        },
        {"Name": "JazzMe", "User":"admin", "Password": "admin",  "Genre":"Jazz", 
            "Description": "Indie jazz radio station, Online cast", "BitRate":128, "BurstSize": 65536, "DumpFile": ""
        }
    ]
}
```