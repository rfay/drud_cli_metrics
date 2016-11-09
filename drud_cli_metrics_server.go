package main

import (
	"database/sql"
	"encoding/json"
	"log"
	"net/http"

	"github.com/gorilla/mux"
	_ "github.com/mattn/go-sqlite3"

	"fmt"
	"os"
	"strconv"
)

type Person struct {
	ID        int    `json:"id,omitempty"`
	Firstname string `json:"firstname,omitempty"`
	Lastname  string `json:"lastname,omitempty"`
}

var dbName = "foo.db"
var db = InitDB(dbName)

func GetPersonEndpoint(w http.ResponseWriter, req *http.Request) {
	params := mux.Vars(req)
	id, _ := strconv.Atoi(params["id"])
	json.NewEncoder(w).Encode(ReadItem(db, id))
}

func GetPeopleEndpoint(w http.ResponseWriter, req *http.Request) {
	json.NewEncoder(w).Encode(ReadAllItems(db))
}

func CreatePersonEndpoint(w http.ResponseWriter, req *http.Request) {
	//fmt.Fprintf(os.Stderr, "Params: %v\n", params);
	var person Person
	result := json.NewDecoder(req.Body).Decode(&person)
	if result != nil {
		fmt.Fprintf(os.Stderr, "result=%v\n", result)
		return
	}

	StoreItem(db, person)
	json.NewEncoder(w).Encode(ReadAllItems(db))
}

func UpdatePersonEndpoint(w http.ResponseWriter, req *http.Request) {
	var person Person
	params := mux.Vars(req)
	id, _ := strconv.Atoi(params["id"])
	err := json.NewDecoder(req.Body).Decode(&person)
	checkErr(err)
	person.ID = id

	StoreItem(db, person)

	json.NewEncoder(w).Encode(ReadAllItems(db))
}

func DeletePersonEndpoint(w http.ResponseWriter, req *http.Request) {
	params := mux.Vars(req)
	id, _ := strconv.Atoi(params["id"])
	fmt.Fprintf(os.Stderr, "Deleting id=%d\n", id)
	DeleteItem(db, id)
	json.NewEncoder(w).Encode(ReadAllItems(db))
}

func InitDB(filepath string) *sql.DB {
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
	CREATE TABLE IF NOT EXISTS items(
		ID INTEGER PRIMARY KEY,
		FirstName TEXT,
		LastName TEXT,
		InsertedDatetime DATETIME
	);
	`

	_, err := db.Exec(sql_table)
	if err != nil {
		panic(err)
	}
}

func StoreItem(db *sql.DB, item Person) {
	var items []Person
	items = append(items, item)
	StoreItems(db, items)
}

func StoreItems(db *sql.DB, items []Person) {
	sqlAddItemWithIdQuery := `
	INSERT OR REPLACE INTO items(
		ID,
		FirstName,
		LastName,
		InsertedDatetime
	) values(?, ?, ?, CURRENT_TIMESTAMP)
	`
	sqlAddItemNoIdQuery := `
	INSERT INTO items(
		FirstName,
		LastName,
		InsertedDatetime
	) values(?, ?, CURRENT_TIMESTAMP)`

	idStatement, err := db.Prepare(sqlAddItemWithIdQuery)
	if err != nil {
		panic(err)
	}
	defer idStatement.Close()

	noIdStatement, err := db.Prepare(sqlAddItemNoIdQuery)
	if err != nil {
		panic(err)
	}

	for _, item := range items {
		// If ID is provided, do upsert, otherwise do insert
		if item.ID != 0 {
			_, err := idStatement.Exec(item.ID, item.Firstname, item.Lastname)
			checkErr(err)
		} else {
			_, err := noIdStatement.Exec(item.Firstname, item.Lastname)
			checkErr(err)
		}

	}
}

func ReadAllItems(db *sql.DB) []Person {
	sql_readall := `
	SELECT ID, FirstName, LastName FROM items
	ORDER BY datetime(InsertedDatetime) DESC
	`

	rows, err := db.Query(sql_readall)
	if err != nil {
		panic(err)
	}
	defer rows.Close()

	var result []Person
	for rows.Next() {
		item := Person{}
		err2 := rows.Scan(&item.ID, &item.Firstname, &item.Lastname)
		if err2 != nil {
			panic(err2)
		}
		result = append(result, item)
	}
	return result
}

func ReadItem(db *sql.DB, id int) Person {
	sqlReadOne := `
	SELECT ID, FirstName, LastName FROM items
	WHERE ID = ?
	ORDER BY datetime(InsertedDatetime) DESC
	`

	var item Person
	db.QueryRow(sqlReadOne, id).Scan(&item.ID, &item.Firstname, &item.Lastname)

	return item
}

func DeleteItem(db *sql.DB, id int) {
	sql_delete := `
	DELETE FROM items
	WHERE ID = ?
	`

	fmt.Fprintf(os.Stderr, "DeleteItem deleting item=%d\n", id)
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
	CreateTable(db)
	router := mux.NewRouter()
	// Read all people and return
	router.HandleFunc("/people", GetPeopleEndpoint).Methods("GET")
	// Create a person - ID is automatically incremented
	router.HandleFunc("/people", CreatePersonEndpoint).Methods("POST")
	// Get a single person
	router.HandleFunc("/people/{id}", GetPersonEndpoint).Methods("GET")
	// Update a single person
	router.HandleFunc("/people/{id}", UpdatePersonEndpoint).Methods("POST")
	// Delete an item by id
	router.HandleFunc("/people/{id}", DeletePersonEndpoint).Methods("DELETE")

	// Listen on port
	log.Fatal(http.ListenAndServe(":12345", router))
}
