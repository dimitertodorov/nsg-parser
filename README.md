# nsg-parser
### NOTICE: ALPHA

Currently in alpha.
All functionality is subject to change.

### Config
Base Config is stored in nsg-parser.yml file.
But can also be defined on the command line.

Example:
```yaml
storage_account_name: oivsjvoisjvoisjdvosdv
storage_account_key: secretSquirrelKey
prefix: resourceId=/SUBSCRIPTIONS/A8BB5151-C23C-4C2A-8043-AAA/RESOURCEGROUPS/SDCCDEV01RGP01/PROVIDERS/MICROSOFT.NETWORK/NETWORKSECURITYGROUPS/SOMENSG-NSG/y=2017/m=06/d=06
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
 --prefix resourceId=/SUBSCRIPTIONS/SUBID/RESOURCEGROUPS/SDCCDEV01RGP01/PROVIDERS/MICROSOFT.NETWORK/NETWORKSECURITYGROUPS/NSGNAME/y=2017/m=06/d=14 \
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




