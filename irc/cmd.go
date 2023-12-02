package irc

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"os"

	"github.com/gregseb/freyabot/chat"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

const ConfigPrefix = "irc"

func Init() (*chat.Option, error) {
	if !viper.GetBool(ConfigPrefix + ".enable") {
		return nil, nil
	}
	var t *tls.Config
	if !viper.GetBool(ConfigPrefix + ".no-tls") {
		t = &tls.Config{}
		t.ServerName = viper.GetString(ConfigPrefix + ".server")
		t.InsecureSkipVerify = viper.GetBool(ConfigPrefix + ".tls-insecure-skip-verify")
		if len(viper.GetStringSlice(ConfigPrefix+".tls-ca-certs")) > 0 {
			t.RootCAs = x509.NewCertPool()
			for _, ca := range viper.GetStringSlice(ConfigPrefix + ".tls-ca-certs") {
				if ca != "" {
					if _, err := os.Stat(ca); os.IsNotExist(err) {
						return nil, errors.Wrapf(fmt.Errorf("%s: %w", chat.ErrInvalidConfig, err), "irc: CA certificate does not exist: %s", ca)
					}
					if caCert, err := os.ReadFile(ca); err != nil {
						return nil, errors.Wrapf(fmt.Errorf("%s: %w", chat.ErrInvalidConfig, err), "irc: failed to read CA certificate: %s", ca)
					} else {
						t.RootCAs.AppendCertsFromPEM(caCert)
					}
				}
			}
		}
		if viper.GetString(ConfigPrefix+".tls-client-cert") != "" && viper.GetString(ConfigPrefix+".tls-client-key") != "" {
			cert, err := tls.LoadX509KeyPair(viper.GetString(ConfigPrefix+".tls-client-cert"), viper.GetString(ConfigPrefix+".tls-client-key"))
			if err != nil {
				return nil, errors.Wrapf(fmt.Errorf("%s: %w", chat.ErrInvalidConfig, err), "irc: failed to load client certificate pair: %s, %s", viper.GetString(ConfigPrefix+".tls-client-cert"), viper.GetString(ConfigPrefix+".tls-client-key"))
			}
			t.Certificates = []tls.Certificate{cert}
		}
	}
	var authMethod int
	authStr := viper.GetString(ConfigPrefix + ".auth-method")
	if authStr == "none" {
		authMethod = AuthMethodNone
	} else if authStr == "nickserv" {
		authMethod = AuthMethodNickServ
	} else if authStr == "sasl" {
		authMethod = AuthMethodSASL
	} else if authStr == "certfp" {
		authMethod = AuthMethodCertFP
	} else {
		return nil, errors.Wrapf(chat.ErrInvalidConfig, "irc: invalid auth method: %s", authStr)
	}

	chatOpt, err := New(
		WithNetwork(viper.GetString(ConfigPrefix+".server"), viper.GetInt(ConfigPrefix+".port")),
		WithNick(viper.GetString(ConfigPrefix+".nick")),
		WithAuthMethod(authMethod),
		WithPassword(viper.GetString(ConfigPrefix+".auth-password")),
		WithChannels(viper.GetStringSlice(ConfigPrefix+".channels")),
		WithDialTimeout(viper.GetFloat64(ConfigPrefix+".dial-timeout")),
		WithKeepAlive(viper.GetFloat64(ConfigPrefix+".keepalive")),
		WithMessageBufferSize(viper.GetInt(ConfigPrefix+".msg-buffer-size")),
		WithTLS(t),
	)
	return &chatOpt, err
}

func Flags(cmd *cobra.Command) {
	// Enable
	cmd.Flags().Bool(ConfigPrefix+"-enable", true, "Enable IRC")
	// Server
	cmd.Flags().String(ConfigPrefix+"-server", "irc.rizon.net", "IRC server to connect to")
	// Port
	cmd.Flags().Int(ConfigPrefix+"-port", 6697, "IRC server port to connect to")
	// Nick
	cmd.Flags().String(ConfigPrefix+"-nick", "freyabot", "IRC nick to use")
	// AuthMethod
	cmd.Flags().String(ConfigPrefix+"-auth-method", "none", "IRC authentication method, one of: none, sasl, nickserv, certfp.")
	// AuthPassword
	cmd.Flags().String(ConfigPrefix+"-auth-password", "", "IRC authentication password. Required if auth-method is nickserv or sasl")
	// Channels
	cmd.Flags().StringSlice(ConfigPrefix+"-channels", []string{"#freyabot"}, "IRC channels to join")
	// DialTimeoutSeconds
	cmd.Flags().Int(ConfigPrefix+"-dial-timeout", 10, "IRC dial timeout in seconds")
	// KeepAliveSeconds
	cmd.Flags().Int(ConfigPrefix+"-keepalive", 60, "IRC keepalive interval in seconds")
	// TLS
	cmd.Flags().Bool(ConfigPrefix+"-no-tls", false, "Disable TLS for IRC")
	// TLSCaCert
	cmd.Flags().StringSlice(ConfigPrefix+"-tls-ca-certs", []string{}, "IRC TLS CA certificates")
	// TLSClientCert
	cmd.Flags().String(ConfigPrefix+"-tls-client-cert", "", "IRC TLS client certificate. Required if auth-method is certfp")
	// TLSClientKey
	cmd.Flags().String(ConfigPrefix+"-tls-client-key", "", "IRC TLS client key. Required if auth-method is certfp")
	// TLSInsecureSkipVerify
	cmd.Flags().Bool(ConfigPrefix+"-tls-insecure-skip-verify", false, "IRC TLS insecure skip verify")
	// MsgBufferSize
	cmd.Flags().Int(ConfigPrefix+"-msg-buffer-size", 100, "IRC message buffer size")
}
