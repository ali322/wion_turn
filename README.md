wion turn service
===

network relay service for resolve communication of cross network segment

## Get Started

```bash
#1. clone this repository

#2. install all dependencies
make install

#3. start project
make start
```

## Release binary

```bash
#1. change arch or os in Makefile if needed

#2. build new binary release
make build
```

## Deploy service

```bash
#1. upload binary file
scp -C bin/turn_server config.yaml turn.service user@deploy.server:/path/to/turn/

#2. copy service file into systemd config directory
cp turn.service /etc/systemd/system/

#3. reload and start daemon  turn service
systemctl daemon-reload && systemctl start turn
```
