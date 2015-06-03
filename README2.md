# chisel

Chisel is an HTTP client and server which acts as a TCP proxy, written in Go (Golang). Chisel useful in situations where you only have access to HTTP, for example – behind a corporate firewall. Chisel is very similar to [crowbar](https://github.com/q3k/crowbar) though achieves **much** higher [performance](#performance). **Warning** Chisel is currently beta software.

![overview](https://docs.google.com/drawings/d/1p53VWxzGNfy8rjr-mW8pvisJmhkoLl82vAgctO_6f1w/pub?w=960&h=720)

### Install

**Binaries**

See [bin](bin)

**Source**

``` sh
$ go get -v github.wdf.sap.corp/cloudfoundry/chisel
```

### Features

* Easy to use
* [Performant](#performance)*
* [Encrypted connections](#security) using `crypto/ssh`
* [Authenticated connections](#authentication), authenticate clients with a users config file, authenticate servers with fingerprint matching.
* Client auto-reconnects with [exponential backoff](https://github.com/morikat/backoff)
* Client can create multiple tunnel endpoints over one TCP connection
* Server optionally doubles as a [reverse proxy](http://golang.org/pkg/net/http/httputil/#NewSingleHostReverseProxy)

### Usage

<tmpl,code: chisel --help>
```

  Usage: chisel [command] [--help]

  Version: 0.0.0-src

  Commands:
    server - runs chisel in server mode
    client - runs chisel in client mode

  Read more:
    https://github.com/morikat/chisel

```
</tmpl>

`chisel server --help`

<tmpl,code: chisel server --help>
```

  Usage: chisel server [options]

  Options:

    --host, Defines the HTTP listening host – the network interface
    (defaults to 0.0.0.0).

    --port, Defines the HTTP listening port (defaults to 8080).

    --key, An optional string to seed the generation of a ECDSA public
    and private key pair. All commications will be secured using this
    key pair. Share this fingerprint with clients to enable detection
    of man-in-the-middle attacks.

    --authfile, An optional path to a users.json file. This file should
    be an object with users defined like:
      "<user:pass>": ["<addr-regex>","<addr-regex>"]
      when <user> connects, their <pass> will be verified and then
      each of the remote addresses will be compared against the list
      of address regular expressions for a match. Addresses will
      always come in the form "<host/ip>:<port>".

    --proxy, Specifies the default proxy target to use when chisel
    receives a normal HTTP request.

    -v, Enable verbose logging

    --help, This help text

  Read more:
    https://github.com/morikat/chisel

```
</tmpl>

`chisel client --help`

<tmpl,code: chisel client --help>
```

  Usage: chisel client [options] <server> <remote> [remote] [remote] ...

  server is the URL to the chisel server.

  remotes are remote connections tunnelled through the server, each of
  which come in the form:

    <local-host>:<local-port>:<remote-host>:<remote-port>

    * remote-port is required.
    * local-port defaults to remote-port.
    * local-host defaults to 0.0.0.0 (all interfaces).
    * remote-host defaults to 0.0.0.0 (server localhost).

    example remotes

      3000
      example.com:3000
      3000:google.com:80
      192.168.0.5:3000:google.com:80

  Options:

    --fingerprint, An optional fingerprint (server authentication)
    string to compare against the server's public key. You may provide
    just a prefix of the key or the entire string. Fingerprint
    mismatches will close the connection.

    --auth, An optional username and password (client authentication)
    in the form: "<user>:<pass>". These credentials are compared to
    the credentials inside the server's --authfile.

    --skip-ssl-validation, If specified wss and https connections do not
    validate certificates.

    --keepalive, An optional keepalive interval. Since the underlying
    transport is HTTP, in many instances we'll be traversing through
    proxies, often these proxies will close idle connections. You must
    specify a time with a unit, for example '30s' or '2m'. Defaults
    to '0s' (disabled).

    -v, Enable verbose logging

    --help, This help text

  Read more:
    https://github.com/morikat/chisel

```
</tmpl>

See also [programmatic usage](https://github.com/jpillora/chisel/wiki/Programmatic-Usage).

### How to connect Cloud Foundry v2 Service

```
$ cf create-service postgresql default chisel-pg
$ cf push chisel-bind-pg --no-start
$ cf bind-service chisel-bind-pg chisel-pg
$ ./bin/caldecott.sh chisel-bind-pg
2015/04/04 10:59:08 client: Connecting to ws://chisel-bind-pg.paas.jp-e1.cloudn-service.com:80
2015/04/04 10:59:08 client: 153.149.13.35:5434#1: Enabled
2015/04/04 10:59:08 client: Fingerprint b1:f5:89:19:71:6f:b0:23:f0:a3:d0:ca:14:e1:19:f9
2015/04/04 10:59:08 client: Sending configurating
2015/04/04 10:59:08 client: Connected (Latency 11.493224ms)
2015/04/04 10:59:11 client: 153.149.13.35:5434#1: conn#1: Open
psql (9.1.15, server 9.2.4)
WARNING: psql version 9.1, server version 9.2.
         Some psql features might not work.
Type "help" for help.

d3ba5929bbe2e40c2851d42b999d0fa5e=> \d
No relations found.
d3ba5929bbe2e40c2851d42b999d0fa5e=> \q
2015/04/04 10:59:17 client: 153.149.13.35:5434#1: conn#1: Close (sent 781 received 0)
```

### Security

Encryption is always enabled. When you start up a chisel server, it will generate an in-memory ECDSA public/private key pair. The public key fingerprint will be displayed as the server starts. Instead of generating a random key, the server may optionally specify a key seed, using the `--key` option, which will be used to seed the key generation. When clients connect, they will also display the server's public key fingerprint. The client can force a particular fingerprint using the `--fingerprint` option. See the `--help` above for more information.

### Authentication

Using the `--authfile` option, the server may optionally provide a `user.json` configuration file to create a list of accepted users. The client then authenticates using the `--auth` option. See [users.json](example/users.json) for an example authentication configuration file. See the `--help` above for more information.

Internally, this is done using the *Password* authentication method provided by SSH. Learn more about `crypto/ssh` here http://blog.gopheracademy.com/go-and-ssh/.

### Performance

With [crowbar](https://github.com/q3k/crowbar), a connection is tunnelled by repeatedly querying the server with updates. This results in a large amount of HTTP and TCP connection overhead. Chisel overcomes this using WebSockets combined with [crypto/ssh](https://golang.org/x/crypto/ssh) to create hundreds of logical connections, resulting in **one** TCP connection per client.

In this simple benchmark, we have:

```
          (direct)
        .--------------->----------------.
       /    chisel         chisel         \
request--->client:2001--->server:2002---->fileserver:3000
       \                                  /
        '--> crowbar:4001--->crowbar:4002'
             client           server
```

Note, we're using an in-memory "file" server on localhost for these tests

*direct*

```
:3000 => 1 bytes in 1.440608ms
:3000 => 10 bytes in 658.833µs
:3000 => 100 bytes in 669.6µs
:3000 => 1000 bytes in 570.242µs
:3000 => 10000 bytes in 655.795µs
:3000 => 100000 bytes in 693.761µs
:3000 => 1000000 bytes in 2.156777ms
:3000 => 10000000 bytes in 18.562896ms
:3000 => 100000000 bytes in 146.355886ms
```

`chisel`

```
:2001 => 1 bytes in 1.393731ms
:2001 => 10 bytes in 1.002992ms
:2001 => 100 bytes in 1.082757ms
:2001 => 1000 bytes in 1.096081ms
:2001 => 10000 bytes in 1.215036ms
:2001 => 100000 bytes in 2.09334ms
:2001 => 1000000 bytes in 9.136138ms
:2001 => 10000000 bytes in 84.170904ms
:2001 => 100000000 bytes in 796.713039ms
```

~100MB in **0.8 seconds**

`crowbar`

```
:4001 => 1 bytes in 3.335797ms
:4001 => 10 bytes in 1.453007ms
:4001 => 100 bytes in 1.811727ms
:4001 => 1000 bytes in 1.621525ms
:4001 => 10000 bytes in 5.20729ms
:4001 => 100000 bytes in 38.461926ms
:4001 => 1000000 bytes in 358.784864ms
:4001 => 10000000 bytes in 3.603206487s
:4001 => 100000000 bytes in 36.332395213s
```

~100MB in **36 seconds**

See more [test/](test/)

### Known Issues

* WebSockets support is required
  * IaaS providers all will support WebSockets
    * Unless an unsupporting HTTP proxy has been forced in front of you, in which case I'd argue that you've been downgraded to PaaS.
  * PaaS providers vary in their support for WebSockets
    * Heroku has full support
    * Openshift has full support though connections are only accepted on ports 8443 and 8080
    * Google App Engine has **no** support

### Contributing

* http://golang.org/doc/code.html
* http://golang.org/doc/effective_go.html
* `github.com/jpillora/chisel/share` contains the shared package
* `github.com/jpillora/chisel/server` contains the server package
* `github.com/jpillora/chisel/client` contains the client package

### Changelog

* `1.0.0` - Init
* `1.1.0` - Swapped out simple symmetric encryption for ECDSA SSH

### Todo

* Better, faster tests
* Expose a stats page for proxy throughput
* Treat client stdin/stdout as a socket

#### MIT License

Copyright © 2015 Jaime Pillora &lt;dev@jpillora.com&gt;

Permission is hereby granted, free of charge, to any person obtaining
a copy of this software and associated documentation files (the
'Software'), to deal in the Software without restriction, including
without limitation the rights to use, copy, modify, merge, publish,
distribute, sublicense, and/or sell copies of the Software, and to
permit persons to whom the Software is furnished to do so, subject to
the following conditions:

The above copyright notice and this permission notice shall be
included in all copies or substantial portions of the Software.

THE SOFTWARE IS PROVIDED 'AS IS', WITHOUT WARRANTY OF ANY KIND,
EXPRESS OR IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF
MERCHANTABILITY, FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT.
IN NO EVENT SHALL THE AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY
CLAIM, DAMAGES OR OTHER LIABILITY, WHETHER IN AN ACTION OF CONTRACT,
TORT OR OTHERWISE, ARISING FROM, OUT OF OR IN CONNECTION WITH THE
SOFTWARE OR THE USE OR OTHER DEALINGS IN THE SOFTWARE.
