package irc

import (
	"bufio"
	"context"
	"crypto/tls"
	"io"
	"net"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/gregseb/freyabot/chat"
	"github.com/pkg/errors"
	"github.com/rs/zerolog/log"
)

const ApiName = "irc"

const (
	DefaultNick                    = "freyabot"
	DefaultLoginDelaySeconds       = 5
	DefaultDialTimeoutSeconds      = 10
	DefaultKeepAliveSeconds        = 60
	DefaultMsgBufferSize           = 100
	DefaultTlsPort                 = 6697
	DefaultPlainPort               = 6667
	ReadDelimiter             byte = '\n'
)

const (
	AuthMethodNone = iota
	AuthMethodNickServ
	AuthMethodSASL
	AuthMethodCertFP
)

const linePattern = `^:(?P<sender>\S+) (?P<command>\S+) (?P<recipient>\S+) :(.*)\r\n$`
const pingPattern = `^PING :(?P<arg>.*)\r\n$`
const errPattern = `^ERROR :(?P<msg>.*)\r\n$`

func WithNetwork(host string, port int) Option {
	return func(a *API) error {
		a.networkHost = host
		a.networkPort = port
		return nil
	}
}

func WithNick(nick string) Option {
	return func(a *API) error {
		a.nick = nick
		return nil
	}
}

func WithPassword(password string) Option {
	return func(a *API) error {
		a.password = password
		return nil
	}
}

func WithAuthMethod(method int) Option {
	return func(a *API) error {
		a.authMethod = method
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

func WithTLS(cfg *tls.Config) Option {
	return func(a *API) error {
		a.tls = cfg
		return nil
	}
}

func WithMessageBufferSize(size int) Option {
	return func(a *API) error {
		a.msgBufSize = size
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
	nick               string
	authMethod         int
	password           string
	networkHost        string
	networkPort        int
	channels           []string
	tls                *tls.Config
	loginDelaySeconds  float64
	dialTimeoutSeconds float64
	keepAliveSeconds   float64

	ready       bool
	conn        io.ReadWriteCloser
	lnRe        *regexp.Regexp
	pingRe      *regexp.Regexp
	errRe       *regexp.Regexp
	msgBufSize  int
	rawMsgs     chan []byte
	lastMsgTime time.Time
	reader      *bufio.Reader
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
		nick:               DefaultNick,
		loginDelaySeconds:  DefaultLoginDelaySeconds,
		dialTimeoutSeconds: DefaultDialTimeoutSeconds,
		keepAliveSeconds:   DefaultKeepAliveSeconds,
		msgBufSize:         DefaultMsgBufferSize,
	}
	if err := a.ApplyOptions(opts...); err != nil {
		return nil, err
	}
	// Make sure a server was specified
	if a.networkHost == "" {
		return nil, errors.WithMessage(chat.ErrInvalidConfig, "irc: no server specified")
	}
	log.Info().Str("api", ApiName).Msgf("server: %s", a.networkHost)
	if a.networkPort == 0 {
		if a.tls != nil {
			log.Info().Str("api", ApiName).Msgf("no port specified and tls is enabled, using default TLS port: %d", DefaultTlsPort)
			a.networkPort = DefaultTlsPort
		} else {
			log.Info().Str("api", ApiName).Msgf("no port specified and tls is disabled, using default plain port: %d", DefaultPlainPort)
			a.networkPort = DefaultPlainPort
		}
	} else {
		log.Info().Str("api", ApiName).Msgf("port: %d", a.networkPort)
	}
	log.Info().Str("api", ApiName).Msgf("nick: %s", a.nick)
	log.Info().Str("api", ApiName).Msgf("channels: %v", a.channels)

	if re, err := regexp.Compile(linePattern); err != nil {
		return nil, err
	} else {
		a.lnRe = re
	}
	if re, err := regexp.Compile(pingPattern); err != nil {
		return nil, err
	} else {
		a.pingRe = re
	}
	if re, err := regexp.Compile(errPattern); err != nil {
		return nil, err
	} else {
		a.errRe = re
	}

	a.rawMsgs = make(chan []byte, a.msgBufSize)

	return chat.CombineOptions(
			chat.WithAPI(a),
			chat.RegisterAction("003", "", "", "", a.actionOnReady),
			chat.RegisterAction("PRIVMSG", "!join (.*)", "!join #channel", "Join the specified channel", a.actionJoinChannel, chat.RoleAdmin),
			chat.RegisterAction("PRIVMSG", "!(part|leave)( (.*))?", "!part #channel", "leave the specified channel", a.actionLeaveChannel, chat.RoleAdmin),
		),
		nil
}

// TODO Handle long messages
func (a *API) SendMessage(c context.Context, msg *chat.Message) error {
	parts := []string{msg.Command}
	if msg.Receiver != "" {
		parts = append(parts, msg.Receiver)
	}
	if msg.Text != "" {
		parts = append(parts, ":"+msg.Text)
	}
	str := strings.Join(parts, " ")
	bts := []byte(str + "\n")
	_, err := a.conn.Write(bts)
	log.Debug().Str("api", ApiName).Str("irc", str).Msg("sent message")
	if err != nil {
		return err
	}
	return nil
}

// TODO Handle long messages
func (a *API) readMessage(c context.Context) error {
	// Setup bufio reader
	bts, err := a.reader.ReadBytes(ReadDelimiter)
	if err != nil {
		return err
	}
	a.rawMsgs <- bts
	return nil
}

func (a *API) ReceiveMessage(c context.Context) (*chat.Message, error) {
	if ct := len(a.rawMsgs); ct == a.msgBufSize {
		log.Warn().Str("api", ApiName).Msgf("message buffer full (%d messages)", ct)
	}
	bts := <-a.rawMsgs
	line := string(bts)
	log.Debug().Str("api", ApiName).Str("irc", line).Msg("received message")
	msg := &chat.Message{}
	if a.lnRe.MatchString(line) {
		parts := a.lnRe.FindStringSubmatch(line)
		msg.Sender = parts[1]
		msg.Command = parts[2]
		msg.Receiver = parts[3]
		msg.Text = parts[4]
	} else if a.pingRe.MatchString(line) {
		parts := a.pingRe.FindStringSubmatch(line)
		return nil, a.pong(c, parts[1])
	} else if a.errRe.MatchString(line) {
		parts := a.errRe.FindStringSubmatch(line)
		return nil, errors.Errorf("irc: error: %s", parts[1])
	} else {
		// TODO return custom error
		return nil, errors.Errorf("irc: line does not match pattern: %s", line)
	}
	a.lastMsgTime = time.Now()
	return msg, nil
}

func (a *API) Start(c context.Context) error {
	if err := a.connect(c); err != nil {
		return err
	}
	go a.pollConn(c)
	return nil
}

func (a *API) Stop(c context.Context) error {
	a.SendMessage(c, &chat.Message{
		Command: "QUIT",
		Text:    "I must go! My people need me.",
	})
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
	a.reader = bufio.NewReader(a.conn)

	// Wait to start receiving messages
	go func() {
		start := time.Now()
		for {
			if !a.lastMsgTime.IsZero() {
				break
			} else if time.Since(start) > time.Duration(float64(time.Second)*a.dialTimeoutSeconds) {
				log.Error().Str("api", ApiName).Msg("timed out waiting for message")
				return
			} else {
				time.Sleep(time.Duration(float64(time.Second) * 0.1))
			}
		}
		// Wait for login delay
		time.Sleep(time.Duration(float64(time.Second) * a.loginDelaySeconds))
		// Attempt to login
		if err := a.login(c); err != nil {
			// TODO If we fail to log in we should try again after a delay and fail if we can't
			// log in after a certain number of attempts.
			log.Error().Str("api", ApiName).Err(err).Msg("error logging in")
			return
		}
	}()
	return nil
}

// pollConn polls the server for messages and queues them for parsing.
// We are doing it this way because the server may send messages faster
// than we can parse them.
// TODO It shouldn't be possible to miss messages, but it's happening with motd after registering.
// And before implementing a queue, it was happening with most of the messages after registering.
func (a *API) pollConn(c context.Context) {
	for {
		err := a.readMessage(c)
		if err != nil {
			log.Error().Str("api", ApiName).Err(err).Msg("error reading message")
		}
	}
}

func (a *API) disconnect() error {
	return a.conn.Close()
}

func (a *API) login(c context.Context) error {
	if err := a.SendMessage(c, &chat.Message{
		Command: "NICK" + " " + a.nick,
	}); err != nil {
		return err
	}
	realname := "FreyaBot"
	if a.nick != DefaultNick {
		realname = realname + " (" + a.nick + ")"
	}
	if err := a.SendMessage(c, &chat.Message{
		Command: "USER " + a.nick + " 0 *",
		Text:    realname,
	}); err != nil {
		return err
	}
	return nil
}

func (a *API) joinChannels(c context.Context) error {
	for _, channel := range a.channels {
		if err := a.joinChannel(c, channel); err != nil {
			return err
		}
	}
	return nil
}

func (a *API) joinChannel(c context.Context, channel string) error {
	if err := a.SendMessage(c, &chat.Message{
		Command: "JOIN " + channel,
	}); err != nil {
		return err
	}
	return nil
}

func (a *API) leaveChannel(c context.Context, channel string) error {
	if err := a.SendMessage(c, &chat.Message{
		Command: "PART " + channel,
	}); err != nil {
		return err
	}
	return nil
}

func (a *API) pong(c context.Context, arg string) error {
	err := a.SendMessage(c, &chat.Message{
		Command: "PONG",
		Text:    arg,
	})
	if err != nil {
		return err
	}
	return nil
}

func (a *API) actionOnReady(c context.Context, re *regexp.Regexp, msg *chat.Message) error {
	a.ready = true
	if err := a.joinChannels(c); err != nil {
		return err
	}
	return nil
}

func (a *API) actionJoinChannel(c context.Context, re *regexp.Regexp, msg *chat.Message) error {
	if err := a.joinChannel(c, re.FindStringSubmatch(msg.Text)[1]); err != nil {
		return err
	}
	return nil
}

func (a *API) actionLeaveChannel(c context.Context, re *regexp.Regexp, msg *chat.Message) error {
	var channel string
	if parts := re.FindStringSubmatch(msg.Text); parts[3] == "" {
		channel = msg.Receiver
	} else {
		channel = parts[3]
	}
	if err := a.leaveChannel(c, channel); err != nil {
		return err
	}
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
