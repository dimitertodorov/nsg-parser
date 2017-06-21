# nsg-parser
### NOTICE: ALPHA
This project is currently in alpha.
All functionality is subject to change.
Major refactoring is still ahead.

## Purpose
This tool is being developed in order to convert Azure NSG Flow logs into other formats.

Some background on NSG Flow Logs can be found here.
https://docs.microsoft.com/en-us/azure/network-watcher/network-watcher-nsg-flow-logging-overview

Go (golang) was choosen for its performance, cross-platform capabilities, and the great Azure SDK for GO
https://github.com/Azure/azure-sdk-for-go/tree/master/storage

## Features
### Basic
* Convert NSG Flow logs to flat local JSON files.
* Send NSG Flows to remote Syslog. 
* Cross-Platform (Windows, OSX, Linux)
* Can run as daemon
* Can be installed as a service on Windows/Linux
* Provides HTTP endpoint for status and metric information (WIP)
### Implementation
* Processing status is persisted to disk, keeping track of changes.
* Blob contents are paged to reduce loading the same parts of the Flow JSON multiple times.
* Processing can be interrupted and restarted at any time.

### Config
Base Config is stored in nsg-parser.yml file.
Most options can be overriden on the command line.
Path to this config file is provided on the command-line with `--config`

Example:
```yaml
# Base Azure settings.
storage_account_name: oivsjvoisjvoisjdvosdv
storage_account_key: secretSquirrelKey
container_name: insights-logs-networksecuritygroupflowevent
# Prefix is not required, but can be useful if sharing containers.
prefix: resourceId=/SUBSCRIPTIONS/SUBID/RESOURCEGROUPS/RGNAME/PROVIDERS/MICROSOFT.NETWORK/NETWORKSECURITYGROUPS/SOMENSG-NSG/y=2017/m=06/d=06
# User must have write privileges to this directory. Must be a full path.
data_path: L:\nsg-data\
# Serve HTTP Process/Metrics endpoint at /status. NOT Secure
serve_http: true
serve_http_bind: 127.0.0.1:3000
# How often do we poll? Less than 60 seconds is pointless since NSG Flows are paged by the minute.
poll_interval: 60
# Set begin_time here to ignore any Blobs stamped before this hour.
# Failing to set this sensibly could result in processing huge amounts of data.
begin_time: 2017-06-20-11
# file or syslog
destination: file
# syslog settings are required for syslog destination only
syslog_protocol: tcp
syslog_host: 127.0.0.1
syslog_port: 5514
# Equivalent to setting http_proxy and https_proxy variables. Useful for service config.
http_proxy: http://10.4.3.2:2222
```

### Logging
Log Path: `data_path` 

Logs are patterned and rotated every hour 

`nsg-parser-%Y%m%d%H%M.log`

Limited logging to Stdout

### Process to File
```yaml
destination: file
```
Processing to file will convert the Azure NSG Flow logs into a flat format.
Each file will be formatted with 
```
nsgLog-NSGNAME-HOURTIME-STARTTIMESTAMP-ENDTIMESTAMP
```

Example:
```
nsgLog-NSGNAME-201706201400-1497967953-1497968013
```

See: NsgFlowLog in `parser\types.go`

Sample Object:
```json
{
    "time": 1497967953,
    "systemId": null,
    "category": null,
    "resourceId": "/SUBSCRIPTIONS/SUBID/RESOURCEGROUPS/RGRP/PROVIDERS/MICROSOFT.NETWORK/NETWORKSECURITYGROUPS/MYNSG",
    "operationName": null,
    "rule": "UserRule_HTTP",
    "mac": "00:01:11:14:38:14",
    "sourceIp": "10.44.1.8",
    "destinationIp": "10.55.11.4",
    "sourcePort": "23653",
    "destinationPort": "80",
    "protocol": "T",
    "trafficFlow": "I",
    "traffic": "A"
  }
```

### Process to Syslog:
```yaml
destination: syslog
```
Send events to remote syslog.
Events are sent once only.

All Syslog is formatted as CEF

See here for documentation:

https://community.saas.hpe.com/t5/ArcSight-Connectors/ArcSight-Common-Event-Format-CEF-Guide/ta-p/1589306

Format is
```
timestamp host CEF:Version|Device Vendor|Device Product|Device Version|Device Event Class ID|Name|Severity|[Extension]
```

Example
```
Jun 21 13:15:34|CEF:0|Microsoft|Azure NSG|1|nsg-flow|nsg-flow|0|cs1=UserRule_HTTP deviceDirection=0 dmac=00:0D:3A:F3:38:54 dpt=80 dst=10.193.160.4 outcome=Allow proto=TCP spt=18166 src=10.199.1.8 start=1498065334000
```

#### Sample Config
```yaml
storage_account_name: oivsjvoisjvoisjdvosdv
storage_account_key: secretSquirrelKey
container_name: insights-logs-networksecuritygroupflowevent
data_path: L:\nsg-data-syslog\
serve_http: true
serve_http_bind: 127.0.0.1:3000
poll_interval: 60
destination: syslog
http_proxy: http://142.107.185.64:2222
begin_time: 2017-06-20-11
syslog_protocol: udp
syslog_host: 142.107.186.142
syslog_port: 514
```

Run:
```
λ nsg-parser.exe process --config l:\syslog-nsg.yml
INFO[0000] loaded config file                            config_file="l:\syslog-nsg.yml"
INFO[0000] started logging                               current_file="L:\nsg-data-syslog\nsg-parser-201706201100.log" logLevel=info path="L:\nsg-data-syslog\nsg-parser-%Y%m%d%H%M.log"
```

In the Log File:
```
time="2017-06-20T11:14:03-04:00" level=info msg="started logging" logLevel=info path="L:\nsg-data-syslog\nsg-parser-%Y%m%d%H%M.log" 
time="2017-06-20T11:14:03-04:00" level=info msg="using proxy" proxy="http://142.107.185.64:2222" 
time="2017-06-20T11:14:03-04:00" level=info msg="serving nsg-parser status  on HTTP" Host="127.0.0.1:3000" 
time="2017-06-20T11:14:04-04:00" level=info msg="processing new blob" LastModified=2017-06-20 13:01:16 +0000 GMT LastProcessedRecord=0001-01-01 00:00:00 +0000 UTC Nsg=NSG-NAME ShortName=NSG-NAME-2017-06-20-12 
time="2017-06-20T11:14:04-04:00" level=info msg="processing new blob" LastModified=2017-06-20 14:01:16 +0000 GMT LastProcessedRecord=0001-01-01 00:00:00 +0000 UTC Nsg=NSG-NAME ShortName=NSG-NAME-2017-06-20-13 
time="2017-06-20T11:14:04-04:00" level=info msg="processing new blob" LastModified=2017-06-20 15:01:19 +0000 GMT LastProcessedRecord=0001-01-01 00:00:00 +0000 UTC Nsg=NSG-NAME ShortName=NSG-NAME-2017-06-20-14 
time="2017-06-20T11:14:04-04:00" level=info msg="processing new blob" LastModified=2017-06-20 15:13:17 +0000 GMT LastProcessedRecord=0001-01-01 00:00:00 +0000 UTC Nsg=NSG-NAME ShortName=NSG-NAME-2017-06-20-15 
time="2017-06-20T11:14:04-04:00" level=info msg="LoadBlobRange()" end=378921 start=0 
time="2017-06-20T11:14:04-04:00" level=info msg="LoadBlobRange()" end=379077 start=0 
time="2017-06-20T11:14:05-04:00" level=info msg="LoadBlobRange()" end=379077 start=0 
time="2017-06-20T11:14:05-04:00" level=info msg="LoadBlobRange()" end=75641 start=0 
time="2017-06-20T11:14:05-04:00" level=info msg="processing completed" type=parser.SyslogClient 
```


### Running as a Service.
This is a WIP. There are some outstanding stability/restart tests to be done.

nsg-parser can be installed as a service.
When installing as a service a config file MUST be used.
```
λ nsg-parser.exe --config l:\syslog-nsg.yml process service --help
Manage nsg-parser service

Usage:
  nsg-parser process service [flags]
  nsg-parser process service [command]

Available Commands:
  install     Install/Reinstall nsg-parser service
  run         run nsg-parser service
  uninstall   Uninstall nsg-parser service

Flags:
  -h, --help                         help for service
      --service_description string   Service Description (default "Parser for MS Azure NSG Flow Logs")
      --service_name string          Service Name (default "nsg-parser")
```

### Example on Windows
```
λ nsg-parser.exe --config l:\syslog-nsg.yml process service install
INFO[0000] loaded config file                            config_file="l:\syslog-nsg.yml"
INFO[0000] started logging                               current_file="L:\nsg-data-syslog\nsg-parser-201706201100.log" logLevel=info path="L:\nsg-data-syslog\nsg-parser-%Y%m%d%H%M.log"
INFO[0000] installing service with config                config_file="l:\syslog-nsg.yml"
INFO[0000] installed service

λ net start nsg-parser
The nsg-parser service is starting.
The nsg-parser service was started successfully.
```


### Get Status over HTTP
An optional feature.
This is a WIP, will be split out into metrics and status.

Example Contents:
```aidl
{
  "GoVersion": "go1.8.3",
  "Version": "0.0.3",
  "ProcessStatus": {
    "resourceId=/SUBSCRIPTIONS/SUBID/RESOURCEGROUPS/RGNAME/PROVIDERS/MICROSOFT.NETWORK/NETWORKSECURITYGROUPS/NSG-NAME/y=2017/m=06/d=20/h=14/m=00/PT1H.json": {
      "name": "resourceId=/SUBSCRIPTIONS/SUBID/RESOURCEGROUPS/RGNAME/PROVIDERS/MICROSOFT.NETWORK/NETWORKSECURITYGROUPS/NSG-NAME/y=2017/m=06/d=20/h=14/m=00/PT1H.json",
      "etag": "0x8D4B7ED358F55A9",
      "last_modified": "2017-06-20T15:01:19Z",
      "last_processed": "2017-06-20T11:14:05.7873103-04:00",
      "last_processed_record": "2017-06-20T14:59:35.479Z",
      "last_processed_time": 1497970773,
      "last_count": 60,
      "last_processed_range": {
        "Start": 0,
        "End": 379077
      },
      "log_time": "2017-06-20T14:00:00Z",
      "nsg_name": "NSG-NAME"
    },
    "resourceId=/SUBSCRIPTIONS/SUBID/RESOURCEGROUPS/RGNAME/PROVIDERS/MICROSOFT.NETWORK/NETWORKSECURITYGROUPS/NSG-NAME/y=2017/m=06/d=20/h=15/m=00/PT1H.json": {
      "name": "resourceId=/SUBSCRIPTIONS/SUBID/RESOURCEGROUPS/RGNAME/PROVIDERS/MICROSOFT.NETWORK/NETWORKSECURITYGROUPS/NSG-NAME/y=2017/m=06/d=20/h=15/m=00/PT1H.json",
      "etag": "0x8D4B7F08F2A695D",
      "last_modified": "2017-06-20T15:25:18Z",
      "last_processed": "2017-06-20T11:25:36.8141728-04:00",
      "last_processed_record": "2017-06-20T15:23:35.472Z",
      "last_processed_time": 1497972213,
      "last_count": 1,
      "last_processed_range": {
        "Start": 145186,
        "End": 151513
      },
      "log_time": "2017-06-20T15:00:00Z",
      "nsg_name": "NSG-NAME"
    }
  },
  "BuildDate": "20170620-15:13:46",
  "BuildUser": "bobthebuilder@itsdtojadim2022",
  "Revision": "acffa303b7c9587e678d60e1281d021ca93685b7",
  "ProcessedFlowCount": 973
}
```

### Install from Binary
Only Windows binaries are being provided for the time being.

#### Windows Powershell
The following will download and extract the required files.

You must still edit/create the nsg-parser.yml file before using.
```
$url = "https://github.com/dimitertodorov/nsg-parser/releases/download/v0.0.4/nsg-parser-0.0.4.windows-amd64.zip"

$basePath = "l:\latest-nsg-parser\"
$dataPath = "$basePath\data"
$env:DATA_PATH=$dataPath

New-Item -ItemType Directory -Force -Path $basePath
New-Item -ItemType Directory -Force -Path $dataPath

$filePath = "$basePath\nsg-parser.zip"
Invoke-WebRequest -Uri $url -OutFile $filePath
Add-Type -assembly “system.io.compression.filesystem”
[io.compression.zipfile]::ExtractToDirectory($filePath, $basePath)
```


### Building
To build on Windows see `scripts\build_windows.ps1`

## HPE Arcsight Integration
Primary driver behind developing this was to integrate Azure NSG into our Arcsight Logging environment.

In our instance, we use SmartConnector running a Syslog Daemon

Then, place the following file into `user/agent/flexagent/microsoft_nsgflow.subagent.sdkrfilereader.properties` 
### Sample FlexConnector Props
```properties
#Microsoft Azure NSG Configuration File
#To be used with https://github.com/dimitertodorov/nsg-parser
replace.defaults=true
trim.tokens=true
comments.start.with=#

#nsgflow:1497052774,UserRule_HTTP,00:0D:3A:F3:38:54,10.199.1.8,25356,10.193.160.4,80,T,I,A

regex=.*nsgflow:(\\d+),(.*),(.*),(.*),(\\d+),(.*),(\\d+),([A-Z]),([A-Z]),([A-Z])

token.count=10

token[0].name=NsgFlowTime
token[0].type=Long
token[1].name=NsgRule
token[1].type=String
token[2].name=SourceMac
token[2].type=MacAddress
token[3].name=Source
token[3].type=IPAddress
token[4].name=SourcePort
token[4].type=Integer
token[5].name=Destination
token[5].type=IPAddress
token[6].name=DestinationPort
token[6].type=Integer
token[7].name=Protocol
token[8].name=FlowDirection
token[9].name=AllowDeny

additionaldata.enabled=false

event.deviceReceiptTime=__createLocalTimeStampFromSecondsSinceEpoch(NsgFlowTime)
event.deviceVendor=__stringConstant(Microsoft)
event.deviceProduct=__stringConstant(Azure NSG)
event.deviceCustomString1=NsgRule
event.deviceCustomString1Label=__oneOf(null,"Rule Label")
event.sourceMacAddress=SourceMac
event.sourceAddress=Source
event.sourcePort=SourcePort
event.destinationAddress=Destination
event.destinationPort=DestinationPort
event.transportProtocol=__simpleMap(Protocol,"T=TCP","U=UDP")
event.deviceDirection=__safeToInteger(__simpleMap(FlowDirection,"O=1","I=0"))
event.categoryOutcome=__simpleMap(AllowDeny,"A=Success","D=Failure")
```

### TODO
* Secure Syslog
* Send Directly to Arcsight SmartMessage, Bypass SmartConnector
* 

### Contributions
Any suggestions/contributions are welcome.



