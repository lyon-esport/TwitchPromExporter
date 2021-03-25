# TwitchStats

Prometheus exporter for twitch.

Also export on the **/** endpoint stream statistics in JSON to integrate in your JS app.

## Get your client id

Go to https://dev.twitch.tv/dashboard/apps/create

Use these informations to submit an application
```
Name: PromExporter
URL oAuth: https://127.0.0.1 (not used)
Category: Analytics Tool
```

On https://dev.twitch.tv/console/apps click on manage PromExporter application and get your client indentifier & client secret

## Log level
```
$ export LOG_LEVEL="info"
$ export LOG_LEVEL="debug"
$ export LOG_LEVEL="warn"
$ export LOG_LEVEL="error"
```

## Run

Define an **CLIENT_ID** in yours env variable. \
For the channels, define an env variable **CHANNELS**, with a list of channel separated by commas, ex:

```bash
$ export CHANNELS="youyoud2,froggedtv,mistermv"
$ export CLIENT_ID="DFSKJFSDKJFSDKDFSJ"
$ export CLIENT_SECRET="DFSKJFSDKJFSDKDFSJ"
$ export LOG_LEVEL="info"
$ export LISTEN_ADDR="0.0.0.0"
$ go run .
```

## Metrics

The exporter is available on `http://<IP>:2112/metrics`
