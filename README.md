# NCMDump
PoC Administrative tool used for dumping all switch configs stored in Solarwinds Orion Network Configuration Monitor

## Overview

ncmdump is a proof-of-concept tool originally part of a larger offensive framework for Solarwinds Orion. The tool allows you to automate the extraction of switch configuration files from all devices under management by NCM. 

## Usage
```
git clone https://github.com/0daysimpson/ncmdump 
cd ./ncmdump
go build

./ncmdump -e -ip $ORION_SERVER_IP -tls -u $ORION_USER -p $ORION_PASS

Usage of ./ncmdump:
  -e    Export server configurations to file
  -ip string
        SW Orion server IP
  -m    Extract server configs  using export instead of edit
  -p string
        SW Orion server password
  -port string
        tcp port for connection
  -tls
        Connect using TLS
  -u string
        SW Orion server username (default "guest")
```
