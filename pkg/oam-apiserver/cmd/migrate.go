package cmd

import (
	"os"

	"github.com/jmoiron/sqlx"
	colorable "github.com/mattn/go-colorable"
	"github.com/nsyszr/admincenter/pkg/oam-apiserver/config"
	migrate "github.com/rubenv/sql-migrate"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

var migrateCmd = &cobra.Command{
	Use:   "migrate",
	Short: "Parent command for applying the SQL migrations",
	Run: func(cmd *cobra.Command, args []string) {
		// Setup logging
		log.SetLevel(log.DebugLevel)
		log.SetFormatter(&log.TextFormatter{
			ForceColors: true,
		})
		log.SetOutput(colorable.NewColorableStdout())

		log.Info("Running OAM API-Server SQL migration...")

		log.WithFields(log.Fields{"databaseUrl": config.FlagDatabaseURL}).
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

		// Init db migrations
		migrations := &migrate.FileMigrationSource{
			Dir: "db/migrations",
		}

		// Exec db migrations
		n, err := migrate.Exec(db.DB, "postgres", migrations, migrate.Up)
		if err != nil {
			log.Error("Failed to apply database migrations: ", err)
			os.Exit(1)
		}
		log.Infof("Applied %d migrations!", n)
	},
}

func init() {
	rootCmd.AddCommand(migrateCmd)

	migrateCmd.Flags().StringVarP(&config.FlagDatabaseURL, "database-url", "", "postgres://u4oam:pw4oam@localhost:5432/oam?sslmode=disable", "Database connection string")
}
