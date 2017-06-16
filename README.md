# nsg-parser
### NOTICE: ALPHA

Currently in alpha.
All functionality is subject to change.
Major refactoring is still ahead.


## Purpose
This tool was written in order to convert Azure NSG Flow logs into other formats.

Go was choosen for its performance.

Some background on NSG Flow Logs can be found here.
https://docs.microsoft.com/en-us/azure/network-watcher/network-watcher-nsg-flow-logging-overview

### Config
Base Config is stored in nsg-parser.yml file.
But can also be defined on the command line.

Example:
```yaml
storage_account_name: oivsjvoisjvoisjdvosdv
storage_account_key: secretSquirrelKey
prefix: resourceId=/SUBSCRIPTIONS/SUBID/RESOURCEGROUPS/RGNAME/PROVIDERS/MICROSOFT.NETWORK/NETWORKSECURITYGROUPS/SOMENSG-NSG/y=2017/m=06/d=06
container_name: insights-logs-networksecuritygroupflowevent
data_path: /tmp/azlogs
syslog_protocol: tcp
syslog_host: 127.0.0.1
syslog_port: 5514
```

## Features

### Process to File
This is a POC module to pre-process the NSG files into a flat format.

Full options:
```
Process NSG Files from Azure Blob Storage to Local File

Usage:
  nsg-parser process file [flags]

Flags:
      --begin_time string   Only Process Files after this time. 2017-01-01-01 (default "2017-06-14-01")
  -h, --help                help for file

Global Flags:
      --config string                 config file (default is $HOME/nsg-parser.json)
      --container_name string         Container Name
      --data_path string              Where to Save the files
      --debug                         DEBUG? Turn on Debug logging with this.
      --dev_mode                      DEV MODE: Use Storage Emulator?
 Must be reachable at http://127.0.0.1:10000
      --prefix string                 Prefix
      --storage_account_key string    Key
      --storage_account_name string   Account
```

Example:
```
go run main.go process file --data_path /tmp/azlog --begin_time=2017-06-14-20
```

Specify Prefix in Command Line
```
go run main.go process file \
 --prefix resourceId=/SUBSCRIPTIONS/SUBID/RESOURCEGROUPS/RGNAME/PROVIDERS/MICROSOFT.NETWORK/NETWORKSECURITYGROUPS/NSGNAME/y=2017/m=06/d=14 \
 --data_path /tmp/azlog \
 --begin_time=2017-06-14-20
```
Output 
```
WARN[0000] process-status.json does not exist. Processing All Files File error: open /tmp/azlog/process-status.json: no such file or directory
...
INFO[0000] before  begin_date ignoring NSGNAME-2017-06-14-17
INFO[0000] before  begin_date ignoring NSGNAME-2017-06-14-18
INFO[0000] before  begin_date ignoring NSGNAME-2017-06-14-19
INFO[0000] before  begin_date ignoring NSGNAME-2017-06-14-20
INFO[0000] processed /tmp/azlog/nsgLog-NSGNAME-201706142100-1497473974-1497477572.json
INFO[0000] processed /tmp/azlog/nsgLog-NSGNAME-201706142200-1497477574-1497477692.json
```
## Syslog
Sending Syslog is similar to pre-processing to file; however all work is done in-memory and directly streamed to remote Syslog.

`--data_path` is still required to store metadata.

Processing Status is preserved between runs so that events are not sent more than once.
NOTE: Currently, no guarantees are made about event receipt.



#### Config
Configuration is done in the nsg-parser.yml or on the command-line.
```
Process NSG Files from Azure Blob Storage to Remote Syslog

Usage:
  nsg-parser process syslog [flags]

Flags:
  -h, --help                     help for syslog
      --syslog_host string       Syslog Hostname or IP (default "127.0.0.1")
      --syslog_port string       Syslog Port (default "5514")
      --syslog_protocol string   tcp or udp (default "tcp")

Global Flags:
      --begin_time string             Only Process Files after this time. 2017-01-01-01 (default "2017-06-15-18")
      --config string                 config file (default is $HOME/nsg-parser.json)
      --container_name string         Container Name
      --data_path string              Where to Save the files
      --debug                         DEBUG? Turn on Debug logging with this.
      --dev_mode                      DEV MODE: Use Storage Emulator?
 Must be reachable at http://127.0.0.1:10000
      --prefix string                 Prefix
      --storage_account_key string    Key
      --storage_account_name string   Account
```

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
* CEF?
* Send Directly to Arcsight SmartMessage

### Contributions
Any suggestions/contributions are welcome.



