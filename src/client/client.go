package chclient

import (
	"crypto/tls"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"

	"github.com/morikat/backoff"
	"github.wdf.sap.corp/cloudfoundry/chisel/share"
	"golang.org/x/crypto/ssh"
	"golang.org/x/net/websocket"
)

type Config struct {
	shared            *chshare.Config
	Fingerprint       string
	Auth              string
	KeepAlive         time.Duration
  SkipSslValidation bool
	Server            string
	Remotes           []string
}

type Client struct {
	*chshare.Logger
	config    *Config
	sshConfig *ssh.ClientConfig
	proxies   []*Proxy
	sshConn   ssh.Conn
	server    string
	running   bool
	runningc  chan error
}

func NewClient(config *Config) (*Client, error) {

	//apply default scheme
	if !strings.HasPrefix(config.Server, "http") {
		config.Server = "http://" + config.Server
	}

	u, err := url.Parse(config.Server)
	if err != nil {
		return nil, err
	}

	//apply default port
	if !regexp.MustCompile(`:\d+$`).MatchString(u.Host) {
		if u.Scheme == "https" || u.Scheme == "wss" {
			u.Host = u.Host + ":443"
		} else {
			u.Host = u.Host + ":80"
		}
	}

	//swap to websockets scheme
	u.Scheme = strings.Replace(u.Scheme, "http", "ws", 1)

	shared := &chshare.Config{}
	for _, s := range config.Remotes {
		r, err := chshare.DecodeRemote(s)
		if err != nil {
			return nil, fmt.Errorf("Failed to decode remote '%s': %s", s, err)
		}
		shared.Remotes = append(shared.Remotes, r)
	}
	config.shared = shared

	client := &Client{
		Logger:   chshare.NewLogger("client"),
		config:   config,
		server:   u.String(),
		running:  true,
		runningc: make(chan error, 1),
	}

	user, pass := chshare.ParseAuth(config.Auth)

	client.sshConfig = &ssh.ClientConfig{
		User:            user,
		Auth:            []ssh.AuthMethod{ssh.Password(pass)},
		ClientVersion:   chshare.ProtocolVersion + "-client",
		HostKeyCallback: client.verifyServer,
	}

	return client, nil
}

//Start then Wait
func (c *Client) Run() error {
	go c.start()
	return c.Wait()
}

func (c *Client) verifyServer(hostname string, remote net.Addr, key ssh.PublicKey) error {
	expect := c.config.Fingerprint
	got := chshare.FingerprintKey(key)
	if expect != "" && !strings.HasPrefix(got, expect) {
		return fmt.Errorf("Invalid fingerprint (%s)", got)
	}
	//overwrite with complete fingerprint
	c.Infof("Fingerprint %s", got)
	return nil
}

//Starts the client
func (c *Client) Start() {
	go c.start()
}

type dialer struct {
  conn net.Conn
}

func (d *dialer) dial(network, addr string) (net.Conn, error) {
  conn, err := net.Dial(network, addr)
  d.conn = conn
  return d.conn, err
}

// connnect ws, wss and wss via proxy w/ or w/o valid certificate
func (c *Client) wsdial(protocol, origin string) (ws *websocket.Conn, err error) {
	// prepare some state

	useTls := strings.HasPrefix(c.server, "wss")

	req, err := http.NewRequest("GET", c.server, nil)
	if err != nil {
		return nil, err
	}
	proxy, err := http.ProxyFromEnvironment(req)
	if err != nil {
		return nil, err
	}

	wsConfig, err := websocket.NewConfig(c.server, origin)
	if err != nil {
		return nil, err
	}
	if useTls && c.config.SkipSslValidation {
		wsConfig.TlsConfig = &tls.Config{InsecureSkipVerify: true}
	}
	if protocol != "" {
		wsConfig.Protocol = []string{protocol}
	}

	targetUrl, err := url.Parse(c.server)
	if err != nil {
		return nil, err
	}

	targetHost, _, err :=  net.SplitHostPort(targetUrl.Host)
	if err != nil {
		return nil, err
	}

	// lets do the work

	if proxy == nil {
		// the easy part
		return websocket.DialConfig(wsConfig)
	}

	// NOTE: only supporting http proxy
	if ! strings.HasPrefix(proxy.String(), "http:") {
		return nil, errors.New("unsupported proxy protocol")
	}
	// the dammed proxy stuff
	if useTls {
		// wss access through proxy
		d := &dialer{}
		hc := &http.Client{
			Transport: &http.Transport{
				Dial: d.dial,
			},
		}
		req, err = http.NewRequest("CONNECT", proxy.String(), nil)
		if err != nil {
			return nil, err
		}
		req.Host = targetUrl.Host
	  req.Header.Set("Host", targetUrl.Host)
	  req.Header.Set("Proxy-Connection", "Keep-Alive")

		_, err := hc.Do(req)
		if err != nil {
			return nil, err
		}
		tlsConfig := &tls.Config{}
		if c.config.SkipSslValidation {
			tlsConfig.InsecureSkipVerify = true
		} else {
			tlsConfig.ServerName = targetHost
		}
		conn := tls.Client(d.conn, tlsConfig)
		return websocket.NewClient(wsConfig, conn)

	} else {
		return nil, errors.New("ws over proxy not implemented, use wss instead")
		// ws access through proxy
		// TODO: implement

	}
}

func (c *Client) start() {
	c.Infof("Connecting to %s\n", c.server)

	//prepare proxies
	for id, r := range c.config.shared.Remotes {
		proxy := NewProxy(c, id, r)
		go proxy.start()
		c.proxies = append(c.proxies, proxy)
	}

	//optional keepalive loop
	if c.config.KeepAlive > 0 {
		go func() {
			for range time.Tick(c.config.KeepAlive) {
				if c.sshConn != nil {
					c.sshConn.SendRequest("ping", true, nil)
				}
			}
		}()
	}

	//connection loop!
	var connerr error
	b := &backoff.Backoff{Max: 5 * time.Minute}

	for {
		if !c.running {
			break
		}
		if connerr != nil {
			d := b.Duration()
			c.Infof("Retrying in %s...\n", d)
			connerr = nil
			time.Sleep(d)
		}

   	// TODO: support --skip-ssl-validation
		ws, err := c.wsdial(chshare.ProtocolVersion, "http://localhost")

		if err != nil {
			c.Infof("Failed to connect: %v", err)
			connerr = err
			continue
		}

		sshConn, chans, reqs, err := ssh.NewClientConn(ws, "", c.sshConfig)

		//NOTE: break == dont retry on handshake failures
		if err != nil {
			if strings.Contains(err.Error(), "unable to authenticate") {
				c.Infof("Authentication failed")
				c.Debugf(err.Error())
			} else {
				c.Infof(err.Error())
			}
			break
		}
		conf, _ := chshare.EncodeConfig(c.config.shared)
		c.Debugf("Sending configurating")
		t0 := time.Now()
		_, configerr, err := sshConn.SendRequest("config", true, conf)
		if err != nil {
			c.Infof("Config verification failed")
			break
		}
		if len(configerr) > 0 {
			c.Infof(string(configerr))
			break
		}
		c.Infof("Connected (Latency %s)", time.Now().Sub(t0))
		//connected
		b.Reset()

		c.sshConn = sshConn
		go ssh.DiscardRequests(reqs)
		go chshare.RejectStreams(chans) //TODO allow client to ConnectStreams
		err = sshConn.Wait()
		//disconnected
		c.sshConn = nil
		if err != nil && err != io.EOF {
			connerr = err
			c.Infof("Disconnection error: %s", err)
			continue
		}
		c.Infof("Disconnected\n")
	}
	close(c.runningc)
}

//Wait blocks while the client is running
func (c *Client) Wait() error {
	return <-c.runningc
}

//Close manual stops the client
func (c *Client) Close() error {
	c.running = false
	if c.sshConn == nil {
		return nil
	}
	return c.sshConn.Close()
}