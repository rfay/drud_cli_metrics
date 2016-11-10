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

type LogItem struct {
	ID               int64  `json:"id,omitempty"`
	ResultCode       int64  `json:"result_code"`
	MachineId        string `json:"machine_id,omitempty"`
	Info             string `json:"info,omitempty"`
	ClientTimestamp  int64  `json:"client_timestamp,omitempty"`
	InsertedDatetime string `json:"inserted_datetime,omitempty"`
}

// Each of these is a pointer
var pDb *sql.DB
var pDbFilepath = flag.String("path", "/var/lib/sqlite3/drud_cli_metrics.db", "Full path to the sqlite3 database file which will be created if it does not exist.")
var pServerPort = flag.Int("port", 12345, "Port on which service should listen")

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

func getAllLogsEndpoint(w http.ResponseWriter, req *http.Request) {
	logItems := readAllItems(pDb)

	json.NewEncoder(w).Encode(logItems)
}

func createLogEndpoint(w http.ResponseWriter, req *http.Request) {
	var logItem LogItem

	err := json.NewDecoder(req.Body).Decode(&logItem)
	if err != nil {
		log.Printf("json.NewDecoder() error result=%v\n", err)
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	count, insertId := storeItem(pDb, logItem)
	if count != 1 {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	// This extra read is extraneous, and we don't actually have to report
	// the exact result, but it's useful for the short term.
	logItem, _ = readItem(pDb, insertId)
	json.NewEncoder(w).Encode(logItem)
}

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

func deleteLogEndpoint(w http.ResponseWriter, req *http.Request) {
	params := mux.Vars(req)
	id, _ := strconv.ParseInt(params["id"], 10, 64)
	fmt.Fprintf(os.Stderr, "Deleting id=%d\n", id)
	deleteItem(pDb, id)
	json.NewEncoder(w).Encode(readAllItems(pDb))
}

func livenessEndpoint(w http.ResponseWriter, req *http.Request) {
	json.NewEncoder(w).Encode("alive")
}

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

func createLogsTable(db *sql.DB) {
	// create table if not exists
	sql_table := `
	create table if not exists logs (
		ID INTEGER PRIMARY KEY,
		clientTimestamp INTEGER,
		resultCode INTEGER,
		machineId TEXT,
		info TEXT,
		insertedDatetime DATETIME
	);
	`

	_, err := db.Exec(sql_table)
	if err != nil {
		panic(err)
	}
}

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
		res, err = stmt.Exec(item.ID, item.ClientTimestamp, item.ResultCode, item.MachineId, item.Info)
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
		res, err = stmt.Exec(item.ClientTimestamp, item.ResultCode, item.MachineId, item.Info)
		if err != nil {
			log.Fatal(err)
		}

	}

	affectedRowCount, _ := res.RowsAffected()
	insertId, _ := res.LastInsertId()
	return affectedRowCount, insertId
}

func readAllItems(db *sql.DB) []LogItem {
	sql_readall := `
	SELECT id, clientTimestamp, resultCode, machineId, info, insertedDatetime FROM logs
	ORDER BY datetime(insertedDatetime)
	`

	rows, err := db.Query(sql_readall)
	if err != nil {
		panic(err)
	}
	defer rows.Close()

	var result []LogItem
	for rows.Next() {
		logItem := LogItem{}
		err2 := rows.Scan(&logItem.ID, &logItem.ClientTimestamp, &logItem.ResultCode, &logItem.MachineId, &logItem.Info, &logItem.InsertedDatetime)
		if err2 != nil {
			panic(err2)
		}
		result = append(result, logItem)
	}
	return result
}

func readItem(db *sql.DB, id int64) (LogItem, error) {
	sqlReadOne := `
	select * from logs
	where ID = ?
	`

	var item LogItem
	err := db.QueryRow(sqlReadOne, id).Scan(&item.ID, &item.ClientTimestamp, &item.ResultCode, &item.MachineId, &item.Info, &item.InsertedDatetime)
	return item, err
}

func deleteItem(db *sql.DB, id int64) {
	sql_delete := `
	delete from logs
	where ID = ?
	`
	stmt, err := db.Prepare(sql_delete)
	checkErr(err)
	res, err := stmt.Exec(id)
	checkErr(err)

	_, err = res.RowsAffected()
	checkErr(err)
}

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
