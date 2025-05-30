package pomegranate

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net/url"
	"os"
	"strings"

	// register the pq driver with the sql package.
	_ "github.com/lib/pq"
)

// Connect calls sql.Open for you, specifying the Postgres driver and printing
// the DB name and host to stdout so you can check that you're connecting to the
// right place before continuing.  dial MUST be in URL form.
func Connect(dial string) (*sql.DB, error) {
	// Failure to set the DATABASE_URL env var or provide the dburl command line
	// flag could result in an empty dburl here.  Catch that.
	if dial == "" {
		return nil, errors.New("empty database url provided")
	}
	parsedUrl, err := url.Parse(dial)
	if err != nil {
		return nil, err
	}
	// trim leading slash
	dbname := strings.Trim(parsedUrl.Path, "/")
	fmt.Printf("Connecting to database '%s' on host '%s'\n", dbname, parsedUrl.Host)
	return sql.Open("postgres", dial)
}

type Database interface {
	ExecContext(context.Context, string, ...interface{}) (sql.Result, error)
	QueryContext(context.Context, string, ...interface{}) (*sql.Rows, error)
	QueryRowContext(context.Context, string, ...interface{}) *sql.Row
}

// deprecated use GetMigrationStateContext
func GetMigrationState(db Database) ([]MigrationRecord, error) {
	return GetMigrationStateContext(context.TODO(), db)
}

// GetMigrationStateContext returns the stack of migration records stored in the
// database's migration_state table.  If that table does not exist, it returns
// an empty list.
func GetMigrationStateContext(ctx context.Context, db Database) ([]MigrationRecord, error) {
	// first see if the migration_state table exists
	var exists bool
	err := db.QueryRowContext(ctx, `
      SELECT EXISTS (
         SELECT 1 
         FROM   pg_tables
         WHERE  schemaname = 'public'
         AND    tablename = 'migration_state'
       );`).Scan(&exists)
	if err != nil {
		return nil, err
	}

	if !exists {
		return []MigrationRecord{}, nil
	}
	rows, err := db.QueryContext(ctx, "SELECT name, time, who FROM migration_state ORDER BY name")
	if err != nil {
		return nil, fmt.Errorf("get past migrations: %v", err)
	}
	defer rows.Close()
	pastMigrations := []MigrationRecord{}
	for rows.Next() {
		var pm MigrationRecord
		if err := rows.Scan(&pm.Name, &pm.Time, &pm.Who); err != nil {
			return nil, fmt.Errorf("get past migrations: %v", err)
		}
		pastMigrations = append(pastMigrations, pm)
	}
	return pastMigrations, nil
}

// deprecated use GetMigrationLogContext
func GetMigrationLog(db Database) ([]MigrationLogRecord, error) {
	return GetMigrationLogContext(context.TODO(), db)
}

// GetMigrationLogContext returns the complete history of all migrations, forward and backward.  If the
// migration_log table does not exist, it returns an empty list of MigrationLogRecords
func GetMigrationLogContext(ctx context.Context, db Database) ([]MigrationLogRecord, error) {
	var exists bool
	err := db.QueryRowContext(ctx, `
      SELECT EXISTS (
         SELECT 1 
         FROM   pg_tables
         WHERE  schemaname = 'public'
         AND    tablename = 'migration_log'
       );`).Scan(&exists)
	if err != nil {
		return nil, err
	}

	if !exists {
		return []MigrationLogRecord{}, nil
	}
	rows, err := db.QueryContext(ctx, "SELECT id, time, name, op, who FROM migration_log ORDER BY id")
	if err != nil {
		return nil, fmt.Errorf("get migration log: %v", err)
	}
	defer rows.Close()
	records := []MigrationLogRecord{}
	for rows.Next() {
		var r MigrationLogRecord
		if err := rows.Scan(&r.ID, &r.Time, &r.Name, &r.Op, &r.Who); err != nil {
			return nil, fmt.Errorf("get migration log: %v", err)
		}
		records = append(records, r)
	}
	return records, nil
}

// deprecated use MigrateBackwardToContext
func MigrateBackwardTo(name string, db Database, allMigrations []Migration, confirm bool) error {
	return MigrateBackwardToContext(context.TODO(), name, db, allMigrations, confirm)
}

// MigrateBackwardToContext will run backward migrations starting with the most recent
// in state, and going through the one provided in `name`.
func MigrateBackwardToContext(ctx context.Context, name string, db Database, allMigrations []Migration, confirm bool) error {
	if len(allMigrations) == 0 {
		return errors.New("no migrations provided")
	}
	state, err := GetMigrationStateContext(ctx, db)
	if err != nil {
		return fmt.Errorf("could not get migration state: %v", err)
	}
	// if nothing in state, nothing to do. error
	if len(state) == 0 {
		return errors.New("state is empty. cannot migrate back")
	}
	toRun, err := getMigrationsToReverse(name, state, allMigrations)
	if err != nil {
		return err
	}
	// get confirmation on the list of backward migrations we're going to run
	if confirm {
		if err := getConfirm(toRun, "Backward", os.Stdin); err != nil {
			return err
		}
	}
	// run the migrations
	for _, mig := range toRun {
		err = runMigrationSQLContext(ctx, db, mig.Name, mig.BackwardSQL)
		if err != nil {
			return err
		}
	}
	return nil
}

// deprecated use MigrateForwardToContext
func MigrateForwardTo(name string, db Database, allMigrations []Migration, confirm bool) error {
	return MigrateForwardToContext(context.TODO(), name, db, allMigrations, confirm)
}

// MigrateForwardToContext will run all forward migrations that have not yet been run, up to and including
// the one specified by `name`.  To run all un-run migrations, set `name` to an empty string.
func MigrateForwardToContext(ctx context.Context, name string, db Database, allMigrations []Migration, confirm bool) error {
	state, err := GetMigrationStateContext(ctx, db)
	if err != nil {
		return fmt.Errorf("could not get migration state: %v", err)
	}

	toRun, err := getForwardMigrationsToRun(name, state, allMigrations)
	if err != nil {
		return err
	}
	if len(toRun) == 0 {
		fmt.Println("No migrations to run")
		return nil
	}
	if confirm {
		if err := getConfirm(toRun, "Forward", os.Stdin); err != nil {
			return err
		}
	}
	// run migrations
	for _, mig := range toRun {
		err = runMigrationSQLContext(ctx, db, mig.Name, mig.ForwardSQL)
		if err != nil {
			return err
		}
	}
	return nil
}

func runMigrationSQLContext(ctx context.Context, db Database, name string, sqlToRun []string) error {
	fmt.Printf("Running %s... ", name)
	for _, sql := range sqlToRun {
		_, err := db.ExecContext(ctx, sql)
		if err != nil {
			fmt.Println("Failure :(")
			return fmt.Errorf("error running migration: %v", err)
		}
	}
	fmt.Println("Success!")
	return nil
}

// deprecated use FakeMigrateForwardToContext
func FakeMigrateForwardTo(name string, db Database, allMigrations []Migration, confirm bool) error {
	return FakeMigrateForwardToContext(context.TODO(), name, db, allMigrations, confirm)
}

// FakeMigrateForwardToContext will record all forward migrations that have not yet been run in the
// migration_state table, up to and including the one specified by `name`, without actually running
// their ForwardSQL. To fake all un-run migrations, set `name` to an empty string.
func FakeMigrateForwardToContext(ctx context.Context, name string, db Database, allMigrations []Migration, confirm bool) error {
	state, err := GetMigrationStateContext(ctx, db)
	if err != nil {
		return fmt.Errorf("could not get migration state: %v", err)
	}

	toRun, err := getForwardMigrationsToRun(name, state, allMigrations)
	if err != nil {
		return err
	}
	if len(toRun) == 0 {
		fmt.Println("No migrations to fake")
		return nil
	}
	if confirm {
		if err := getConfirm(toRun, "Forward", os.Stdin); err != nil {
			return err
		}
	}
	for _, m := range toRun {
		fmt.Printf("Faking %s... ", m.Name)
		_, err := db.ExecContext(ctx, "INSERT INTO migration_state (name) VALUES ($1)", m.Name)
		if err != nil {
			fmt.Println("Failure :(")
			return fmt.Errorf("error faking migration: %v", err)
		}
		fmt.Println("Success!")
	}
	return nil
}
