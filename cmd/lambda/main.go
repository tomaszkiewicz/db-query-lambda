package main

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log"
	"strings"

	"github.com/aws/aws-lambda-go/lambda"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/service/rds/rdsutils"
	_ "github.com/go-sql-driver/mysql"
	_ "github.com/lib/pq"
	"github.com/spf13/viper"
	"github.com/tomaszkiewicz/db-query-lambda/pkg/payload"
)

func handleRequest(ctx context.Context, req payload.QueryRequest) (*payload.QueryResponse, error) {
	dbEngine := viper.GetString("rds-engine")
	dbHost := viper.GetString("rds-host")
	dbUser := viper.GetString("rds-user")
	dbPort := viper.GetInt("rds-port")
	dbPasswordInitial := viper.GetString("rds-password-initial")
	dbSwitchToIamAuth := viper.GetBool("rds-switch-to-iam-auth")
	region := viper.GetString("aws-default-region")
	dbName := viper.GetString("rds-database")
	if req.Database != "" {
		dbName = req.Database
	}

	log.Println("getting RDS IAM auth token")

	awsCred := credentials.NewEnvCredentials()                                                           // lambda gets credentials from ENV variables
	token, err := rdsutils.BuildAuthToken(fmt.Sprintf("%s:%d", dbHost, dbPort), region, dbUser, awsCred) // specifying port is very important as it cannot authenticate without that and docs say nothing about it
	if err != nil {
		log.Println("unable to generate database token", err)
		return nil, err
	}

	log.Println("creating database connection")

	var db *sql.DB
	// creating connection to the specific database engine using token
	db, err = createDatabaseConnection(dbEngine, dbUser, token, dbHost, dbPort, dbName)
	if err != nil {
		log.Println("unable to open db connection using iam auth", err)

		// fallback to use initial password

		// creating connection to the specific database engine using initial password
		db, err = createDatabaseConnection(dbEngine, dbUser, dbPasswordInitial, dbHost, dbPort, dbName)
		if err != nil {
			log.Println("unable to open db connection using initial password", err)
			return nil, err
		}
		log.Println("connected to the database using initial password")

		if dbSwitchToIamAuth {
			// TODO implement granting permissions to use IAM auth for the user
		}
	} else {
		log.Println("connected to the database using IAM auth")
	}

	defer db.Close()

	log.Println("querying database")

	results, err := queryDatabase(db, req.Query)
	if err != nil {
		log.Println("unable to query dabase", err)
		return nil, err
	}

	log.Printf("got %d rows from database", len(results))

	return &payload.QueryResponse{
		Rows: results,
	}, nil
}

func createDatabaseConnection(engine string, user string, password string, host string, port int, database string) (db *sql.DB, err error) {
	switch engine {
	case "mysql":
		dsn := fmt.Sprintf("%s:%s@tcp(%s:%d)/%s?tls=true&allowCleartextPasswords=1", user, password, host, port, database) // cleartext passwords are required so mysql can validate token
		db, err = sql.Open("mysql", dsn)
	case "postgres":
		dsn := fmt.Sprintf("user=%s password=%s host=%s port=%d dbname=%s sslmode=verify-full", user, password, host, port, database)
		log.Println(dsn)
		db, err = sql.Open("postgres", dsn)
	default:
		return nil, errors.New("invalid database engine specified")
	}

	if err == nil {
		log.Println("checking connection and auth")
		_, err = queryDatabase(db, "SELECT 1")
	}
	return
}

func queryDatabase(db *sql.DB, query string) ([]map[string]string, error) {
	// based on https://kylewbanks.com/blog/query-result-to-map-in-golang
	// what we need here is to handle dynamic structure of results that may come from the database

	rows, err := db.Query(query)
	if err != nil {
		return nil, err
	}

	cols, err := rows.Columns()
	if err != nil {
		return nil, err
	}

	var results []map[string]string

	for rows.Next() {
		// Create a slice of interface{}'s to represent each column,
		// and a second slice to contain pointers to each item in the columns slice.
		columns := make([]interface{}, len(cols))
		columnPointers := make([]interface{}, len(cols))
		for i := range columns {
			columnPointers[i] = &columns[i]
		}

		// Scan the result into the column pointers...
		if err := rows.Scan(columnPointers...); err != nil {
			return nil, err
		}

		// Create our map, and retrieve the value for each column from the pointers slice,
		// storing it in the map with the name of the column as the key.
		m := make(map[string]string)
		for i, colName := range cols {
			val := columnPointers[i].(*interface{})
			m[colName] = fmt.Sprintf("%s", *val)
		}

		results = append(results, m)
	}

	return results, nil
}

func main() {
	viper.SetEnvKeyReplacer(strings.NewReplacer("-", "_"))
	viper.AutomaticEnv()

	lambda.Start(handleRequest)
}
