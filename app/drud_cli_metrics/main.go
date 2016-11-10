package main

// Microservice to accept logging from drud cli tool
// Many thanks for
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

func GetLogEndpoint(w http.ResponseWriter, req *http.Request) {
	params := mux.Vars(req)
	id, _ := strconv.Atoi(params["id"])
	item, err := ReadItem(pDb, id)
	if err != nil {
		w.WriteHeader(http.StatusNotFound)
	} else {
		json.NewEncoder(w).Encode(item)
	}
}

func GetAllLogsEndpoint(w http.ResponseWriter, req *http.Request) {
	json.NewEncoder(w).Encode(ReadAllItems(pDb))
}

func CreateLogEndpoint(w http.ResponseWriter, req *http.Request) {
	var person LogItem
	result := json.NewDecoder(req.Body).Decode(&person)
	if result != nil {
		fmt.Fprintf(os.Stderr, "result=%v\n", result)
		return
	}

	StoreItem(pDb, person)
	json.NewEncoder(w).Encode(ReadAllItems(pDb))
}

func UpdateLogEndpoint(w http.ResponseWriter, req *http.Request) {
	var log LogItem
	params := mux.Vars(req)
	id, _ := strconv.Atoi(params["id"])
	err := json.NewDecoder(req.Body).Decode(&log)
	checkErr(err)
	log.ID = id

	StoreItem(pDb, log)

	json.NewEncoder(w).Encode(ReadAllItems(pDb))
}

func DeleteLogEndpoint(w http.ResponseWriter, req *http.Request) {
	params := mux.Vars(req)
	id, _ := strconv.Atoi(params["id"])
	fmt.Fprintf(os.Stderr, "Deleting id=%d\n", id)
	DeleteItem(pDb, id)
	json.NewEncoder(w).Encode(ReadAllItems(pDb))
}

func LivenessEndpoint(w http.ResponseWriter, req *http.Request) {
	json.NewEncoder(w).Encode("alive")
}
func ReadinessEndpoint(w http.ResponseWriter, req *http.Request) {
	json.NewEncoder(w).Encode("ready")
}

func InitDB(filepath string) *sql.DB {
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

func CreateTable(db *sql.DB) {
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

func StoreItem(db *sql.DB, item LogItem) {
	var items []LogItem
	items = append(items, item)
	StoreItems(db, items)
}

func StoreItems(db *sql.DB, items []LogItem) {
	sqlAddItemWithIdQuery := `
	INSERT OR REPLACE INTO logs (
		ID,
		clientTimestamp,
		resultCode,
		machineId,
		info,
		InsertedDatetime
	) values(?, ?, ?, ?, ?, CURRENT_TIMESTAMP)
	`
	sqlAddItemNoIdQuery := `
	INSERT INTO logs (
		clientTimestamp,
		resultCode,
		machineId,
		info,
		InsertedDatetime
	) values(?, ?, ?, ?, CURRENT_TIMESTAMP)`

	idStatement, err := db.Prepare(sqlAddItemWithIdQuery)
	if err != nil {
		panic(err)
	}
	defer idStatement.Close()

	noIdStatement, err := db.Prepare(sqlAddItemNoIdQuery)
	if err != nil {
		panic(err)
	}
	defer noIdStatement.Close()

	for _, item := range items {
		// If ID is provided, do upsert, otherwise do insert
		if item.ID != 0 {
			_, err := idStatement.Exec(item.ID, item.clientTimestamp, item.resultCode, item.machineId, item.info)
			checkErr(err)
		} else {
			_, err := noIdStatement.Exec(item.clientTimestamp, item.resultCode, item.machineId, item.info)
			checkErr(err)
		}

	}
}

func ReadAllItems(db *sql.DB) []LogItem {
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

func ReadItem(db *sql.DB, id int) (LogItem, error) {
	sqlReadOne := `
	select * from logs
	where ID = ?
	`

	var item LogItem
	err := db.QueryRow(sqlReadOne, id).Scan(&item.ID, &item.clientTimestamp, &item.resultCode, &item.machineId, &item.info, &item.insertedTimestamp)
	return item, err
}

func DeleteItem(db *sql.DB, id int) {
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

	pDb = InitDB(*pDbFilepath)
	CreateTable(pDb)

	router := mux.NewRouter()
	// Read all command log and return
	router.HandleFunc("/v1.0/log", GetAllLogsEndpoint).Methods("GET")
	// Create a command log - ID is automatically incremented
	router.HandleFunc("/v1.0/log", CreateLogEndpoint).Methods("POST")
	// Get a single person
	router.HandleFunc("/v1.0/log/{id}", GetLogEndpoint).Methods("GET")
	// Update a single person
	router.HandleFunc("/v1.0/log/{id}", UpdateLogEndpoint).Methods("POST")
	// Delete an item by id
	router.HandleFunc("/v1.0/log/{id}", DeleteLogEndpoint).Methods("DELETE")
	// Readiness probe
	router.HandleFunc("/readiness", ReadinessEndpoint).Methods("GET")
	// Liveness probe
	router.HandleFunc("/healthz", LivenessEndpoint).Methods("GET")

	// Listen on port
	log.Fatal(http.ListenAndServe(fmt.Sprintf(":%d", *pServerPort), router))
}
