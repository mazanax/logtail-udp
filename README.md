# Monolog Logtail UDP Proxy

This proxy makes it possible to send logs to Logtail from Monolog using UDP transport (doesn't require establishing
connection between your php backend and Logtail or this proxy)

### How it works

1. Deploy this proxy to your local network
2. Send logs from Monolog to this proxy
3. Read your logs in logtail

### Configuration

There are environment variables that you need to set up to get this proxy working:

`LOGTAIL_TOKEN` - logtail source token which you can get in your logtail account

`LISTEN_HOST` - default: 127.0.0.1 - ip address of server where this proxy will be deployed. If you want to use this
proxy from local network
make sure that you set correct address (if you set 127.0.0.1 and want to connect from another server it won't work)

`LISTEN_PORT` - default: 49152 - port of this proxy

`LOGTAIL_SEND_INTERVAL` - default: 10s - Interval for sending logs to logtail. Used to group multiple entries into a
single query. Syntax: 1h 20m 30s = 1 hour 20 minutes 30 seconds

`LOGS_BUFFER_SIZE` - default: 64 - The size of the log pack that can be sent to logtail at one time.