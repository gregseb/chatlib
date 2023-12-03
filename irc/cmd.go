package irc

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"os"

	"github.com/gregseb/chatlib"
	"github.com/pkg/errors"
	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

func Init() (*chatlib.Option, error) {
	if !viper.GetBool(ApiName + ".enable") {
		log.Info().Msg("IRC disabled")
		return nil, nil
	}
	log.Info().Msg("IRC enabled")
	var t *tls.Config
	if !viper.GetBool(ApiName + ".no-tls") {
		log.Info().Str("api", ApiName).Msg("TLS Enabled")
		t = &tls.Config{}
		t.ServerName = viper.GetString(ApiName + ".server")
		log.Info().Str("api", ApiName).Msgf("tls using servername: %s", t.ServerName)
		t.InsecureSkipVerify = viper.GetBool(ApiName + ".tls-insecure-skip-verify")
		log.Info().Str("api", ApiName).Msgf("tls insecure skip verify: %t", t.InsecureSkipVerify)
		if len(viper.GetStringSlice(ApiName+".tls-ca-certs")) > 0 {
			t.RootCAs = x509.NewCertPool()
			for _, ca := range viper.GetStringSlice(ApiName + ".tls-ca-certs") {
				if ca != "" {
					if _, err := os.Stat(ca); os.IsNotExist(err) {
						return nil, errors.Wrapf(fmt.Errorf("%s: %w", chatlib.ErrInvalidConfig, err), "irc: CA certificate does not exist: %s", ca)
					}
					if caCert, err := os.ReadFile(ca); err != nil {
						return nil, errors.Wrapf(fmt.Errorf("%s: %w", chatlib.ErrInvalidConfig, err), "irc: failed to read CA certificate: %s", ca)
					} else {
						t.RootCAs.AppendCertsFromPEM(caCert)
					}
					log.Trace().Str("api", ApiName).Msgf("tls added CA certificate: %s", ca)
				}
			}
			log.Info().Str("api", ApiName).Msgf("tls using CA certificates: %v", viper.GetStringSlice(ApiName+".tls-ca-certs"))
		}
		if viper.GetString(ApiName+".tls-client-cert") != "" && viper.GetString(ApiName+".tls-client-key") != "" {
			cert, err := tls.LoadX509KeyPair(viper.GetString(ApiName+".tls-client-cert"), viper.GetString(ApiName+".tls-client-key"))
			if err != nil {
				return nil, errors.Wrapf(fmt.Errorf("%s: %w", chatlib.ErrInvalidConfig, err), "irc: failed to load client certificate pair: %s, %s", viper.GetString(ApiName+".tls-client-cert"), viper.GetString(ApiName+".tls-client-key"))
			}
			t.Certificates = []tls.Certificate{cert}
			log.Info().Str("api", ApiName).Msgf("tls using client certificate: %s", viper.GetString(ApiName+".tls-client-cert"))
			log.Info().Str("api", ApiName).Msgf("tls using client key: %s", viper.GetString(ApiName+".tls-client-key"))
		}
	}
	var authMethod int
	switch viper.GetString(ApiName + ".auth-method") {
	case "none":
		authMethod = AuthMethodNone
	case "nickserv":
		authMethod = AuthMethodNickServ
	case "sasl":
		authMethod = AuthMethodSASL
	case "certfp":
		authMethod = AuthMethodCertFP
	default:
		return nil, errors.Wrapf(chatlib.ErrInvalidConfig, "irc: invalid auth method: %s", viper.GetString(ApiName+".auth-method"))
	}
	log.Info().Str("api", ApiName).Msgf("auth method: %s", viper.GetString(ApiName+".auth-method"))

	a, err := New(
		WithNetwork(viper.GetString(ApiName+".server"), viper.GetInt(ApiName+".port")),
		WithNick(viper.GetString(ApiName+".nick")),
		WithAuthMethod(authMethod),
		WithPassword(viper.GetString(ApiName+".auth-password")),
		WithChannels(viper.GetStringSlice(ApiName+".channels")),
		WithDialTimeout(viper.GetFloat64(ApiName+".dial-timeout")),
		WithKeepAlive(viper.GetFloat64(ApiName+".keepalive")),
		WithMessageBufferSize(viper.GetInt(ApiName+".msg-buffer-size")),
		WithTLS(t),
	)
	if err != nil {
		return nil, errors.Wrapf(fmt.Errorf("%s: %w", chatlib.ErrInvalidConfig, err), "irc: failed to initialize IRC")
	}
	// Make sure a server was specified
	if a.networkHost == "" {
		return nil, errors.WithMessage(chatlib.ErrInvalidConfig, "irc: no server specified")
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

	chatOpt := chatlib.CombineOptions(
		chatlib.WithAPI(a),
		chatlib.RegisterAction("005", "", "", "", a.actionOnReady),
		chatlib.RegisterAction("PRIVMSG", "!join (.*)", "!join #channel", "Join the specified channel", a.actionJoinChannel, chatlib.RoleAdmin),
		chatlib.RegisterAction("PRIVMSG", "!(part|leave)( (.*))?", "!part #channel", "leave the specified channel", a.actionLeaveChannel, chatlib.RoleAdmin),
		chatlib.RegisterAction("PRIVMSG", "!ping", "!ping", "ping the server and ask for a pong", a.actionPing),
	)

	return &chatOpt, nil
}

func Flags(cmd *cobra.Command) {
	// Enable
	cmd.Flags().Bool(ApiName+"-enable", true, "Enable IRC")
	// Server
	cmd.Flags().String(ApiName+"-server", "", "IRC server to connect to. Required")
	// Port
	cmd.Flags().Int(ApiName+"-port", 0, "IRC server port to connect to. If not specified, defaults to 6697 if TLS is enabled, otherwise 6667")
	// Nick
	cmd.Flags().String(ApiName+"-nick", "freyabot", "IRC nick to use")
	// AuthMethod
	cmd.Flags().String(ApiName+"-auth-method", "none", "IRC authentication method, one of: none, sasl, nickserv, certfp.")
	// AuthPassword
	cmd.Flags().String(ApiName+"-auth-password", "", "IRC authentication password. Required if auth-method is nickserv or sasl")
	// Channels
	cmd.Flags().StringSlice(ApiName+"-channels", []string{}, "IRC channels to join")
	// DialTimeoutSeconds
	cmd.Flags().Int(ApiName+"-dial-timeout", 10, "IRC dial timeout in seconds")
	// KeepAliveSeconds
	cmd.Flags().Int(ApiName+"-keepalive", 60, "IRC keepalive interval in seconds")
	// TLS
	cmd.Flags().Bool(ApiName+"-no-tls", false, "Disable TLS for IRC. Take note of the port you are connecting to and be sure to read the server's documentation")
	// TLSCaCert
	cmd.Flags().StringSlice(ApiName+"-tls-ca-certs", []string{}, "IRC TLS CA certificates")
	// TLSClientCert
	cmd.Flags().String(ApiName+"-tls-client-cert", "", "IRC TLS client certificate. Required if auth-method is certfp")
	// TLSClientKey
	cmd.Flags().String(ApiName+"-tls-client-key", "", "IRC TLS client key. Required if auth-method is certfp")
	// TLSInsecureSkipVerify
	cmd.Flags().Bool(ApiName+"-tls-insecure-skip-verify", false, "IRC TLS insecure skip verify")
	// MsgBufferSize
	cmd.Flags().Int(ApiName+"-msg-buffer-size", 100, "IRC message buffer size")
}
