package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"net/http"

	_ "github.com/go-sql-driver/mysql"
)

var db *sql.DB

func handler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Content-Type", "application/json; charset=utf-8")

	text := r.URL.Query().Get("text")
	if text == "" {
		json.NewEncoder(w).Encode(map[string]string{"status": "error"})
		return
	}

	rows, err := db.Query("SELECT * FROM katottg WHERE text LIKE ? LIMIT 40", "%"+text+"%")
	if err != nil {
		json.NewEncoder(w).Encode(map[string]string{"status": "error", "message": err.Error()})
		return
	}
	defer rows.Close()

	columns, _ := rows.Columns()
	var results []map[string]interface{}

	for rows.Next() {
		values := make([]interface{}, len(columns))
		pointers := make([]interface{}, len(columns))
		for i := range values {
			pointers[i] = &values[i]
		}
		rows.Scan(pointers...)
		row := make(map[string]interface{})
		for i, col := range columns {
			val := values[i]
			if b, ok := val.([]byte); ok {
				row[col] = string(b)
			} else {
				row[col] = val
			}
		}
		results = append(results, row)
	}

	if results == nil {
		results = []map[string]interface{}{}
	}

	json.NewEncoder(w).Encode(map[string]interface{}{"status": "done", "data": results})
}

func main() {
	dsn := "bdcontro_katottg:f99y4i!CL)@tcp(bdcontro.mysql.tools:3306)/bdcontro_katottg?charset=utf8mb4"
	var err error
	db, err = sql.Open("mysql", dsn)
	if err != nil {
		log.Fatal(err)
	}
	if err = db.Ping(); err != nil {
		log.Fatal("DB ping failed:", err)
	}
	fmt.Println("Server running on :3000")
	http.HandleFunc("/", handler)
	log.Fatal(http.ListenAndServe(":3000", nil))
}
