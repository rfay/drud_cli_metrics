package main

// Microservice to accept logging from drud cli tool
// Many thanks for
//   database/sql tutorial: http://go-database-sql.org/
//   sqlite3: https://siongui.github.io/2016/01/09/go-sqlite-example-basic-usage/
//   sqlite3: https://astaxie.gitbooks.io/build-web-application-with-golang/content/en/05.3.html
//   json API: https://www.thepolyglotdeveloper.com/2016/07/create-a-simple-restful-api-with-golang/

import (
	"database/sql"
	"encoding/json"
	"log"
	"net/http"

	"github.com/gorilla/mux"
	_ "github.com/mattn/go-sqlite3"

	"flag"
	"fmt"
	"os"
	"strconv"
)

// LogItem is used for json and database interactions
type LogItem struct {
	ID               int64  `json:"id,omitempty"`
	ResultCode       int64  `json:"result_code"`
	MachineID        string `json:"machine_id,omitempty"`
	Info             string `json:"info,omitempty"`
	ClientTimestamp  int64  `json:"client_timestamp,omitempty"`
	InsertedDatetime string `json:"inserted_datetime,omitempty"`
}

// Each of these is a pointer
var pDb *sql.DB
var pDbFilepath = flag.String("path", "/var/lib/sqlite3/drud_cli_metrics.db", "Full path to the sqlite3 database file which will be created if it does not exist.")
var pServerPort = flag.Int("port", 12345, "Port on which service should listen")

// getLogEndpoint it the implementation for GET /v1.0/logentry/{id}
func getLogEndpoint(w http.ResponseWriter, req *http.Request) {
	params := mux.Vars(req)
	id, _ := strconv.ParseInt(params["id"], 10, 64)

	item, err := readItem(pDb, id)
	if err != nil {
		w.WriteHeader(http.StatusNotFound)
	} else {
		json.NewEncoder(w).Encode(item)
	}
}

// getAllLogsEndpoint is the implementation for GET /v1.0/logentry
//
// It currently returns all log entries, but needs to be limited or provide
// a query.
func getAllLogsEndpoint(w http.ResponseWriter, _ *http.Request) {
	logItems := readAllItems(pDb)

	json.NewEncoder(w).Encode(logItems)
}

// createLogEndpoint is the implentation for POST /v1.0/logentry
//
// based on json provided, it will write a new logentry into the db
func createLogEndpoint(w http.ResponseWriter, req *http.Request) {
	var logItem LogItem

	err := json.NewDecoder(req.Body).Decode(&logItem)
	if err != nil {
		log.Printf("json.NewDecoder() error result=%v\n", err)
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	count, insertID := storeItem(pDb, logItem)
	if count != 1 {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	// This extra read is extraneous, and we don't actually have to report
	// the exact result, but it's useful for the short term.
	logItem, _ = readItem(pDb, insertID)
	json.NewEncoder(w).Encode(logItem)
}

// updateLogEndpoint is the implementation for POST /v1.0/logentry/{id}
// and it will update an existing item
func updateLogEndpoint(w http.ResponseWriter, req *http.Request) {
	var logItem LogItem
	params := mux.Vars(req)
	id, _ := strconv.ParseInt(params["id"], 10, 64)
	err := json.NewDecoder(req.Body).Decode(&logItem)
	checkErr(err)
	logItem.ID = id

	storeItem(pDb, logItem)

	json.NewEncoder(w).Encode(readAllItems(pDb))
}

// deleteLogEndpoint is the implementation of DELETE /v1.0/logentry/{id}
func deleteLogEndpoint(w http.ResponseWriter, req *http.Request) {
	params := mux.Vars(req)
	id, _ := strconv.ParseInt(params["id"], 10, 64)
	fmt.Fprintf(os.Stderr, "Deleting id=%d\n", id)
	affected := deleteItem(pDb, id)
	if affected == 0 {
		w.WriteHeader(http.StatusNotFound)
		return
	} else {
		// TODO: Remove this as only useful for debugging interactions
		json.NewEncoder(w).Encode(readAllItems(pDb))
	}
}

// livenessEndpoint is a trivial function to be used by kubernetes for
// determining whether a service is ready
func livenessEndpoint(w http.ResponseWriter, _ *http.Request) {
	json.NewEncoder(w).Encode("alive")
}

// initDB creates a sqlite3 file if necessary with the filepath provided
func initDB(filepath string) *sql.DB {
	if _, err := os.Stat(filepath); os.IsNotExist(err) {
		log.Printf("DB File %s does not exist so will be created", filepath)
	} else {
		log.Printf("DB File %s already exists", filepath)
	}

	db, err := sql.Open("sqlite3", filepath)
	if err != nil {
		panic(err)
	}
	if db == nil {
		panic("db nil")
	}
	return db
}

// createLogsTable creates the logs table if it doesn't already exist
func createLogsTable(db *sql.DB) {
	// create table if not exists
	sqlTable := `
	create table if not exists logs (
		ID INTEGER PRIMARY KEY,
		clientTimestamp INTEGER,
		resultCode INTEGER,
		machineId TEXT,
		info TEXT,
		insertedDatetime DATETIME
	);
	`

	_, err := db.Exec(sqlTable)
	if err != nil {
		panic(err)
	}
}

// storeItem writes an item to the database (or updates if it if already exists)
func storeItem(db *sql.DB, item LogItem) (int64, int64) {
	//log.Printf("storeItem: Incoming item=%v", item)
	var res sql.Result
	if item.ID != 0 {
		upsertQuery := `insert or replace into logs (
		ID,
		clientTimestamp,
		resultCode,
		machineId,
		info,
		insertedDatetime
	        ) values(?, ?, ?, ?, ?, CURRENT_TIMESTAMP)`

		stmt, err := db.Prepare(upsertQuery)
		if err != nil {
			log.Fatal(err)
		}
		res, err = stmt.Exec(item.ID, item.ClientTimestamp, item.ResultCode, item.MachineID, item.Info)
		if err != nil {
			log.Fatal(err)
		}
	} else {
		insertQuery := `insert into logs (
		clientTimestamp,
		resultCode,
		machineId,
		info,
		insertedDatetime
	        ) values(?, ?, ?, ?, CURRENT_TIMESTAMP)`

		stmt, err := db.Prepare(insertQuery)
		if err != nil {
			log.Fatal(err)
		}
		res, err = stmt.Exec(item.ClientTimestamp, item.ResultCode, item.MachineID, item.Info)
		if err != nil {
			log.Fatal(err)
		}

	}

	affectedRowCount, _ := res.RowsAffected()
	insertID, _ := res.LastInsertId()
	return affectedRowCount, insertID
}

// readAllItems gets all existing items from the logs table
// It probably shouldn't do that.
func readAllItems(db *sql.DB) []LogItem {
	sqlReadAll := `
	SELECT id, clientTimestamp, resultCode, machineId, info, insertedDatetime FROM logs
	ORDER BY datetime(insertedDatetime)
	`

	rows, err := db.Query(sqlReadAll)
	if err != nil {
		panic(err)
	}
	defer rows.Close()

	var result []LogItem
	for rows.Next() {
		logItem := LogItem{}
		err2 := rows.Scan(&logItem.ID, &logItem.ClientTimestamp, &logItem.ResultCode, &logItem.MachineID, &logItem.Info, &logItem.InsertedDatetime)
		if err2 != nil {
			panic(err2)
		}
		result = append(result, logItem)
	}
	return result
}

// readItem returns one item from the database based on the id passed in.
// It returns both item and err so it's possible for the caller to find out
// if anything was actually found.
func readItem(db *sql.DB, id int64) (LogItem, error) {
	sqlReadOne := `
	select * from logs
	where ID = ?
	`

	var item LogItem
	err := db.QueryRow(sqlReadOne, id).Scan(&item.ID, &item.ClientTimestamp, &item.ResultCode, &item.MachineID, &item.Info, &item.InsertedDatetime)
	return item, err
}

// deleteItem deletes an item by id, returning number of affected rows
func deleteItem(db *sql.DB, id int64) int64 {
	sqlDelete := `
	delete from logs
	where ID = ?
	`
	stmt, err := db.Prepare(sqlDelete)
	checkErr(err)
	res, err := stmt.Exec(id)
	checkErr(err)

	affected, _ := res.RowsAffected()
	return affected
}

// checkErr just panics if error is set
func checkErr(err error) {
	if err != nil {
		panic(err)
	}
}

func main() {
	flag.Parse()

	log.Printf("Will listen on port=%d\n", *pServerPort)
	log.Printf("Sqlite DB file fullpath=%s\n", *pDbFilepath)

	pDb = initDB(*pDbFilepath)
	defer pDb.Close()

	createLogsTable(pDb)

	router := mux.NewRouter()
	// Read all command log and return
	router.HandleFunc("/v1.0/logitem", getAllLogsEndpoint).Methods("GET")
	// Create a command log - ID is automatically incremented
	router.HandleFunc("/v1.0/logitem", createLogEndpoint).Methods("POST")
	// Get a single log
	router.HandleFunc("/v1.0/logitem/{id}", getLogEndpoint).Methods("GET")
	// Update a single log
	router.HandleFunc("/v1.0/logitem/{id}", updateLogEndpoint).Methods("POST")
	// Delete an item by id
	router.HandleFunc("/v1.0/logitem/{id}", deleteLogEndpoint).Methods("DELETE")
	// Readiness probe
	router.HandleFunc("/readiness", livenessEndpoint).Methods("GET")
	// Liveness probe - just reuse. Could easily be the same URI instead of separate
	router.HandleFunc("/healthz", livenessEndpoint).Methods("GET")

	// Listen on port
	log.Fatal(http.ListenAndServe(fmt.Sprintf(":%d", *pServerPort), router))
}
