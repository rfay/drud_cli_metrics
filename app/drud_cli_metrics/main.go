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
	ID                int    `json:"id,omitempty"`
	clientTimestamp   int    `json:"client_timestamp,omitempty"`
	resultCode        int    `json:"result_code,omitempty"`
	machineId         string `json:"machine_id,omitempty"`
	info              string `json:info,omitempty"`
	insertedTimestamp string `json:"result_code,omitempty"`
}

// Each of these is a pointer
var pDb *sql.DB
var pDbFilepath = flag.String("path", "/var/lib/sqlite3/drud_cli_metrics.db", "Full path to the sqlite3 database file which will be created if it does not exist.")
var pServerPort = flag.Int("port", 12345, "Port on which service should listen")

func getLogEndpoint(w http.ResponseWriter, req *http.Request) {
	params := mux.Vars(req)
	id, _ := strconv.Atoi(params["id"])
	item, err := readItem(pDb, id)
	if err != nil {
		w.WriteHeader(http.StatusNotFound)
	} else {
		json.NewEncoder(w).Encode(item)
	}
}

func getAllLogsEndpoint(w http.ResponseWriter, req *http.Request) {
	json.NewEncoder(w).Encode(readAllItems(pDb))
}

func createLogEndpoint(w http.ResponseWriter, req *http.Request) {
	var logItem LogItem
	log.Printf("createLogEndpoint: Incoming body=%v", req.Body)

	err := json.NewDecoder(req.Body).Decode(&logItem)
	if err != nil {
		log.Printf("json.NewDecoder() error result=%v\n", err)
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	log.Printf("createLogEndpoint: Incoming logItem=%v", logItem)

	count, _ := storeItem(pDb, logItem)
	if count != 1 {
		w.WriteHeader(http.StatusInternalServerError)
	}
	json.NewEncoder(w).Encode(readAllItems(pDb))
}

func updateLogEndpoint(w http.ResponseWriter, req *http.Request) {
	var log LogItem
	params := mux.Vars(req)
	id, _ := strconv.Atoi(params["id"])
	err := json.NewDecoder(req.Body).Decode(&log)
	checkErr(err)
	log.ID = id

	storeItem(pDb, log)

	json.NewEncoder(w).Encode(readAllItems(pDb))
}

func deleteLogEndpoint(w http.ResponseWriter, req *http.Request) {
	params := mux.Vars(req)
	id, _ := strconv.Atoi(params["id"])
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
		InsertedDatetime DATETIME
	);
	`

	_, err := db.Exec(sql_table)
	if err != nil {
		panic(err)
	}
}

func storeItem(db *sql.DB, item LogItem) (int64, error) {
	log.Printf("storeItem: Incoming item=%v", item)
	var res sql.Result
	if item.ID != 0 {
		upsertQuery := `INSERT OR REPLACE INTO logs (
		ID,
		clientTimestamp,
		resultCode,
		machineId,
		info,
		InsertedDatetime
	        ) values(?, ?, ?, ?, ?, CURRENT_TIMESTAMP)`

		stmt, err := db.Prepare(upsertQuery)
		if err != nil {
			log.Fatal(err)
		}
		res, err = stmt.Exec(item.ID, item.clientTimestamp, item.resultCode, item.machineId, item.info)
		if err != nil {
			log.Fatal(err)
		}
	} else {
		insertQuery := `insert into logs (
		clientTimestamp,
		resultCode,
		machineId,
		info,
		InsertedDatetime
	        ) values(?, ?, ?, ?, CURRENT_TIMESTAMP)`

		stmt, err := db.Prepare(insertQuery)
		if err != nil {
			log.Fatal(err)
		}
		res, err = stmt.Exec(item.clientTimestamp, item.resultCode, item.machineId, item.info)
		if err != nil {
			log.Fatal(err)
		}

	}

	return res.RowsAffected()

}

func readAllItems(db *sql.DB) []LogItem {
	sql_readall := `
	select * from logs
	order by datetime(InsertedDatetime)
	`

	rows, err := db.Query(sql_readall)
	if err != nil {
		panic(err)
	}
	defer rows.Close()

	var result []LogItem
	for rows.Next() {
		item := LogItem{}
		err2 := rows.Scan(&item.ID, &item.clientTimestamp, &item.resultCode, &item.machineId, &item.info, &item.insertedTimestamp)
		if err2 != nil {
			panic(err2)
		}
		result = append(result, item)
	}
	return result
}

func readItem(db *sql.DB, id int) (LogItem, error) {
	sqlReadOne := `
	select * from logs
	where ID = ?
	`

	var item LogItem
	err := db.QueryRow(sqlReadOne, id).Scan(&item.ID, &item.clientTimestamp, &item.resultCode, &item.machineId, &item.info, &item.insertedTimestamp)
	return item, err
}

func deleteItem(db *sql.DB, id int) {
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
	router.HandleFunc("/v1.0/log", getAllLogsEndpoint).Methods("GET")
	// Create a command log - ID is automatically incremented
	router.HandleFunc("/v1.0/log", createLogEndpoint).Methods("POST")
	// Get a single log
	router.HandleFunc("/v1.0/log/{id}", getLogEndpoint).Methods("GET")
	// Update a single log
	router.HandleFunc("/v1.0/log/{id}", updateLogEndpoint).Methods("POST")
	// Delete an item by id
	router.HandleFunc("/v1.0/log/{id}", deleteLogEndpoint).Methods("DELETE")
	// Readiness probe
	router.HandleFunc("/readiness", livenessEndpoint).Methods("GET")
	// Liveness probe
	router.HandleFunc("/healthz", livenessEndpoint).Methods("GET")

	// Listen on port
	log.Fatal(http.ListenAndServe(fmt.Sprintf(":%d", *pServerPort), router))
}
