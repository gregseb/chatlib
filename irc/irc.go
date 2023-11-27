package irc

import (
	"bufio"
	"context"
	"crypto/tls"
	"io"
	"net"
	"strconv"
	"sync"
	"time"

	"github.com/gregseb/freyabot/chat"
)

const (
	DefaultDialTimeoutSeconds      = 10
	DefaultKeepAliveSeconds        = 60
	ReadDelimiter             byte = '\n'
)

func WithNetwork(host string, port int) Option {
	return func(a *API) error {
		a.networkHost = host
		a.networkPort = port
		return nil
	}
}

func WithChannel(channel string) Option {
	return func(a *API) error {
		if a.channels == nil {
			a.channels = make([]string, 0)
		}
		a.channels = append(a.channels, channel)
		return nil
	}
}

func WithChannels(channels []string) Option {
	return func(a *API) error {
		if a.channels == nil {
			a.channels = make([]string, 0)
		}
		a.channels = append(a.channels, channels...)
		return nil
	}
}

func WithDialTimeout(seconds float64) Option {
	return func(a *API) error {
		a.dialTimeoutSeconds = seconds
		return nil
	}
}

func WithKeepAlive(seconds float64) Option {
	return func(a *API) error {
		a.keepAliveSeconds = seconds
		return nil
	}
}

func WithTLS(tls *tls.Config) Option {
	return func(a *API) error {
		a.tls = tls
		return nil
	}
}

func CombineOptions(opts ...Option) Option {
	return func(a *API) error {
		return a.ApplyOptions(opts...)
	}
}

type Option func(*API) error

type API struct {
	networkHost        string
	networkPort        int
	channels           []string
	tls                *tls.Config
	dialTimeoutSeconds float64
	keepAliveSeconds   float64

	mutex sync.Mutex
	conn  io.ReadWriteCloser
}

var _ chat.API = (*API)(nil)

func (a *API) ApplyOptions(opts ...Option) error {
	for _, opt := range opts {
		if err := opt(a); err != nil {
			return err
		}
	}
	return nil
}

func New(opts ...Option) (chat.Option, error) {
	a := &API{
		dialTimeoutSeconds: DefaultDialTimeoutSeconds,
		keepAliveSeconds:   DefaultKeepAliveSeconds,
	}
	if err := a.ApplyOptions(opts...); err != nil {
		return nil, err
	}
	return chat.CombineOptions(
			chat.WithAPI(a),
			chat.RegisterAction("!join", "!join #channel", "Join the specified channel", a.joinChannel, chat.RoleAdmin),
			chat.RegisterAction("!(part|leave)", "!part #channel", "leave the specified channel", a.leaveChannel, chat.RoleAdmin),
		),
		nil
}

func (a *API) SendMessage(c context.Context, msg *chat.Message) error {
	return nil
}

func (a *API) ReceiveMessage(c context.Context) (*chat.Message, error) {
	// Setup bufio reader
	r := bufio.NewReader(a.conn)
	a.mutex.Lock()
	line, err := r.ReadString(ReadDelimiter)
	a.mutex.Unlock()
	if err != nil {
		return nil, err
	}
	return &chat.Message{
		Text:     line,
		Sender:   "irc",
		Receiver: "irc",
	}, nil
}

func (a *API) Start(c context.Context) error {
	if err := a.connect(c); err != nil {
		return err
	}
	return nil
}

func (a *API) Stop(c context.Context) error {
	if err := a.disconnect(); err != nil {
		return err
	}
	return nil
}

func (a *API) connect(c context.Context) error {
	var conn io.ReadWriteCloser
	if a.tls != nil {
		cn, err := a.connectTLS(c)
		if err != nil {
			return err
		}
		conn = cn
	} else {
		cn, err := a.connectPlain(c)
		if err != nil {
			return err
		}
		conn = cn
	}
	a.conn = conn
	return nil
}

func (a *API) disconnect() error {
	return a.conn.Close()
}

func (a *API) joinChannel(msg *chat.Message) error {
	return nil
}

func (a *API) leaveChannel(msg *chat.Message) error {
	return nil
}

func (a *API) getDialer() (*net.Dialer, error) {
	d := &net.Dialer{
		Timeout:   time.Duration(float64(time.Second) * a.dialTimeoutSeconds),
		KeepAlive: time.Duration(float64(time.Second) * a.keepAliveSeconds),
	}
	return d, nil
}

func (a *API) connectPlain(c context.Context) (net.Conn, error) {
	var dialer *net.Dialer
	if d, err := a.getDialer(); err != nil {
		return nil, err
	} else {
		dialer = d
	}
	conn, err := dialer.DialContext(c, "tcp", a.serverPort())
	if err != nil {
		return nil, err
	}
	return conn, nil
}

func (a *API) connectTLS(c context.Context) (net.Conn, error) {
	var dialer *tls.Dialer
	if d, err := a.getDialer(); err != nil {
		return nil, err
	} else {
		dialer = &tls.Dialer{
			NetDialer: d,
			Config:    a.tls,
		}
	}
	conn, err := dialer.DialContext(c, "tcp", a.serverPort())
	if err != nil {
		return nil, err
	}
	return conn, nil
}

func (a *API) serverPort() string {
	return a.networkHost + ":" + strconv.Itoa(a.networkPort)
}
