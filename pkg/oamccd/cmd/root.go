package cmd

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"

	log "github.com/Sirupsen/logrus"
	"github.com/go-redis/redis"
	"github.com/gorilla/mux"
	"github.com/nsyszr/admincenter/pkg/controlchannel"
	"github.com/spf13/cobra"
	"github.com/streadway/amqp"
)

var flagPort int
var flagSecure bool
var flagServerCert string
var flagServerKey string
var flagRedisAddr string
var flagRedisPassword string
var flagRedisDB int
var flagAMQPURL string
var flagClientAuthEnabled bool
var flagClientAuthCACert string

func init() {
	rootCmd.Flags().IntVarP(&flagPort, "port", "l", 9443, "HTTPS server listening port")

	rootCmd.Flags().BoolVarP(&flagSecure, "secure", "s", false, "Enable HTTPS")
	rootCmd.Flags().StringVarP(&flagServerCert, "server-cert", "c", "server.crt", "HTTPS server certificate")
	rootCmd.Flags().StringVarP(&flagServerKey, "server-key", "k", "server.key", "HTTPS server private key")

	rootCmd.Flags().StringVarP(&flagRedisAddr, "redis-addr", "", "localhost:6379", "Redis server address")
	rootCmd.Flags().StringVarP(&flagRedisPassword, "redis-password", "", "", "Redis server password")
	rootCmd.Flags().IntVarP(&flagRedisDB, "redis-db", "", 0, "Redis server DB")

	rootCmd.Flags().StringVarP(&flagAMQPURL, "amqp-url", "", "amqp://guest:guest@localhost:5672/", "AMQP broker URL")

	rootCmd.Flags().BoolVarP(&flagClientAuthEnabled, "client-auth", "", false, "Enable certificate-based client authentication")
	rootCmd.Flags().StringVarP(&flagClientAuthCACert, "client-auth-ca-cert", "", "ca.crt", "CA certificate for client authentication")
}

var rootCmd = &cobra.Command{
	// Use:   "run [realm, ...] ([command] | -f file)",
	Short: "Run OAM Control-Channel Daemon",
	Args:  cobra.MinimumNArgs(0),
	Run: func(cmd *cobra.Command, args []string) {
		log.SetLevel(log.DebugLevel)

		log.Info("Starting OAM Control Channel WebSocket Server")

		// Setup connection to Redis
		db := redis.NewClient(&redis.Options{
			Addr:     flagRedisAddr,
			Password: flagRedisPassword,
			DB:       flagRedisDB,
		})
		defer db.Close()

		// Setup connection to AMQP
		amqpConn, err := amqp.Dial(flagAMQPURL)
		if err != nil {
			log.Error("Failed to connect to AMQP:", err)
			return
		}
		defer amqpConn.Close()

		// Create new control channel server
		r := mux.NewRouter()
		_, err = controlchannel.NewServer(db, amqpConn, r)
		if err != nil {
			log.Error("Failed to create new control channel server:", err)
			return
		}

		// Setup HTTP server with TLS config
		tlsConfig := &tls.Config{}

		// Setup certificate-based client authentication
		if flagClientAuthEnabled {
			caCert, err := ioutil.ReadFile(flagClientAuthCACert)
			if err != nil {
				log.Fatal(err)
			}
			caCertPool := x509.NewCertPool()
			caCertPool.AppendCertsFromPEM(caCert)

			tlsConfig.ClientCAs = caCertPool
			// NoClientCert
			// RequestClientCert
			// RequireAnyClientCert
			// VerifyClientCertIfGiven
			// RequireAndVerifyClientCert
			tlsConfig.ClientAuth = tls.RequireAndVerifyClientCert

			tlsConfig.BuildNameToCertificate()
		}

		httpServer := &http.Server{
			Addr:      fmt.Sprintf(":%d", flagPort),
			Handler:   r,
			TLSConfig: tlsConfig,
		}

		// Start the HTTP server
		if flagSecure {
			if err := httpServer.ListenAndServeTLS(flagServerCert, flagServerKey); err != nil {
				log.Error("Failed to start HTTP server:", err)
			}
		} else {
			if err := httpServer.ListenAndServe(); err != nil {
				log.Error("Failed to start HTTP server:", err)
			}
		}
	},
}

// Execute is the app entry point und called by main
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}
