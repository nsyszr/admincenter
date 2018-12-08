package cmd

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"

	"github.com/gorilla/mux"
	"github.com/jmoiron/sqlx"
	colorable "github.com/mattn/go-colorable"
	"github.com/nsyszr/admincenter/pkg/api/device"
	"github.com/nsyszr/admincenter/pkg/middleware"
	"github.com/nsyszr/admincenter/pkg/oam-apiserver/config"
	"github.com/ory/herodot"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

var serveCmd = &cobra.Command{
	Use:   "serve",
	Short: "Parent command for starting the HTTP server",
	Run: func(cmd *cobra.Command, args []string) {
		// Setup logging
		log.SetLevel(log.DebugLevel)
		log.SetFormatter(&log.TextFormatter{
			ForceColors: true,
		})
		log.SetOutput(colorable.NewColorableStdout())

		log.Info("Starting OAM API-Server...")

		log.WithFields(log.Fields{"databaseUrl": config.FlagDatabaseURL, "port": config.FlagPort}).
			Debug("Application settings")

		// Connect to PostgreSQL database
		db, err := sqlx.Open("postgres", config.FlagDatabaseURL)
		if err != nil {
			log.Error(err)
			os.Exit(1)
		}
		defer db.Close()

		// Check the database connection
		if err := db.Ping(); err != nil {
			log.Error("Database connection failed: ", err)
			os.Exit(1)
		}

		// Create a new HTTP router
		r := mux.NewRouter()

		// Handle API
		deviceManager := device.NewSQLManager(db)
		deviceHandler := device.NewHandler(deviceManager, herodot.NewJSONWriter(nil))
		deviceHandler.RegisterRoutes(r.PathPrefix("/api/v1").Subrouter())

		//apiServer := api.NewServer(daoForAccounts, daoForAPISecurity)
		//apiServer.Handle(r.PathPrefix("/api/v1").Subrouter())

		// Catch SIGINT
		stop := make(chan os.Signal, 1)
		signal.Notify(stop, os.Interrupt)

		// Create the http server and start it in the background
		h := &http.Server{Addr: fmt.Sprintf(":%d", config.FlagPort), Handler: middleware.WithLogging(r)}

		go func() {
			log.Infof("Listening on http://0.0.0.0%s\n", fmt.Sprintf(":%d", config.FlagPort))

			if err := h.ListenAndServe(); err != nil {
				log.Error(err)
				os.Exit(1)
			}
		}()

		log.Info("OAM API-Server started")
		<-stop

		// App received SIGINT signal. Shutdown now!
		log.Info("Shutting down OAM API-Server...")
		h.Shutdown(context.Background())
		log.Info("OAM API-Server stopped")
	},
}

func init() {
	rootCmd.AddCommand(serveCmd)

	serveCmd.Flags().IntVarP(&config.FlagPort, "port", "", 8080, "HTTP server listening port")
	serveCmd.Flags().StringVarP(&config.FlagDatabaseURL, "database-url", "", "postgres://u4oam:pw4oam@localhost:5432/oam?sslmode=disable", "Database connection string")
}
