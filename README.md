# Chisel

This is a fork of [Chisel](https://github.com/morikat/chisel).
It has been enhanced to work behind a proxy (like a Corporate Wireless LAN) and to deal with self signed certificates.

Chisel is an HTTP client and server which acts as a TCP proxy, written in Go (Golang).
On Cloud Foundry, it can be used to map TCP endpoints of your backing services to your local workstation. It provides binaries for 64 bit OSX, Windows and Linux.

*NOTE*: 
- Use this feature with care and only in the intended way to access your backing services. You need to understand the security relevance of that feature. 
- Use this feature _only_ for development purposes, where you need access to the backing services (like PostgreSQL, Redis, etc.) from a developer system

### How to use

You need a Cloud Foundry (CF) account and the CF command line tool on your workstation.
You might also have a service created to be used a backing service `my-backing-service`
for application(s) running on CF. The backing service can be connected via TCP to
your application.

* Clone the repository to your local workstation
```
  git clone https://github.com/abh1kg/chisel-service-connector.git
  cd chisel-service-connector
```
* For security reasons you should *always* use authenticated connection.
  Create a file `auth.json` in the root directory of this project for user
  `myuser` and password `mysecret` like this:
```
  echo '{ "myuser:mysecret": [""] }' > auth.json
```
  Note for Windows Command Prompt users: Remove the single quotes when you run this in a dos shell:
```
  echo { "myuser:mysecret": [""] } > auth.json
```
* Push the chisel app under a free name
```
  cf push my-chisel-app --no-start
```
* Bind your backing service to the chisel app.
```
  cf bind-service my-chisel-app my-backing-service
```
* Start the chisel app.
```
  cf start my-chisel-app
```
* Fetch the service meta data from the environment of your chisel app.
```
  cf env my-chisel-app
```
* You should find your service metadata in the environment. E.g:
``` json
...
 "VCAP_SERVICES": {
  "redis-lite": [
   {
    "credentials": {
     "hostname": "10.78.148.124",
     "password": "upnvyqvr2turzjh2z",
     "port": "32764",
     "ports": {
      "6379/tcp": "32764"
     }
    },
    "label": "redis-lite",
    "name": "my-backing-service",
    "plan": "free",
    "tags": [
     "redis28",
     "redis",
     "key-value"
    ]
   }
  ]
 }
}
...
```
* Use the binary for your platform in bin folder to start the chisel client.
```
  bin/chisel_windows_amd64 client --auth myuser:mysecret https://<url_to_chisel_server_app> localhost:12345:10.78.148.124:32764
```
  Where the connect string `localhost:12345:10.78.148.124:32764` defines the local port
  on your work station (i.e. `12345`), the host running your service
  (i.e. `10.78.148.124`) and the port to connect to your service
  (i.e. `32764`). The auth user and secret must match your settings in
  `auth.json`.

* Now you can connect to your backing service on your local work station. E.g.
  If your backing service is redis (as in this example), you can connect a
  redis_cli like:
``` sh
  redis-cli -h localhost -p 12345 -a upnvyqvr2turzjh2z
```
  If your backing service is PostgreSQL, you can connect using a PostgreSQL client like PgAdmin or PSQL, passing along the required variables from the `VCAP_SERVICES` JSON structure.

* Although, access to the chisel app is secured and protected by password,
  you should delete the chisel app if you don't use it anymore for security reasons. Note that this should be used for development purposes _only_.

### Advanced

When using a proxy you neet to access the chisel up via https. This is because
most proxies do not handle ws connections correctly. If there is no proxy in
between you can use http access to the chisel server as well. All data
send over the forwarded port is still encrypted, even if the websocket connection
itself is not encrypted.

In case you use self signed certificates on your application route, you need to
skip strict ssl validation:
``` sh
  chisel client --skip-ssl-validation ...
```
Several portforwardings can be multiplexed onto a single websocket connection.
I.e.:
``` sh
  chisel client --auth <user>:<password> https://<chisel-server> \
    localhost:<local_port_1>:<remote_host_1>:<remote_port_1> \
    localhost:<local_port_2>:<remote_host_2>:<remote_port_2>
```
This will forward two remote ports to your local workstation using one and the
same websocket connection.

You might also look into [README2](README2.md) for further details.




