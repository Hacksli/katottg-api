package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"math"
	"net/http"
	"os"
	"strconv"

	_ "github.com/go-sql-driver/mysql"
)

var db *sql.DB
var tableName string

const defaultPerPage = 20
const minPerPage = 5

type Response struct {
	FoundRows int                      `json:"foundRows"`
	Total     int                      `json:"total"`
	Items     []map[string]interface{} `json:"items"`
	Links     map[int]string           `json:"links"`
	Filters   map[string]interface{}   `json:"filters"`
}

func handler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Content-Type", "application/json; charset=utf-8")

	text := r.URL.Query().Get("text")
	if text == "" {
		json.NewEncoder(w).Encode(map[string]string{"status": "error", "message": "text parameter required"})
		return
	}

	// Page limit
	perPage := defaultPerPage
	if pl := r.URL.Query().Get("pagelimit"); pl != "" {
		if v, err := strconv.Atoi(pl); err == nil && v > minPerPage {
			perPage = v
		}
	}

	// Page (1-based from client, convert to 0-based internally)
	page := 0
	if p := r.URL.Query().Get("page"); p != "" {
		if v, err := strconv.Atoi(p); err == nil && v > 0 {
			page = v - 1
		}
	}

	likeParam := "%" + text + "%"

	// Get total count
	var total int
	countQuery := fmt.Sprintf("SELECT COUNT(*) FROM `%s` WHERE text LIKE ?", tableName)
	if err := db.QueryRow(countQuery, likeParam).Scan(&total); err != nil {
		json.NewEncoder(w).Encode(map[string]string{"status": "error", "message": err.Error()})
		return
	}

	// Calculate pagination
	totalPages := int(math.Ceil(float64(total) / float64(perPage)))
	if totalPages < 1 {
		totalPages = 1
	}
	if page >= totalPages {
		page = totalPages - 1
	}
	offset := page * perPage

	// Fetch items
	query := fmt.Sprintf("SELECT * FROM `%s` WHERE text LIKE ? ORDER BY id ASC LIMIT ? OFFSET ?", tableName)
	rows, err := db.Query(query, likeParam, perPage, offset)
	if err != nil {
		json.NewEncoder(w).Encode(map[string]string{"status": "error", "message": err.Error()})
		return
	}
	defer rows.Close()

	columns, _ := rows.Columns()
	var items []map[string]interface{}

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
		items = append(items, row)
	}

	if items == nil {
		items = []map[string]interface{}{}
	}

	// Build links (1-based page numbers)
	links := make(map[int]string)
	baseURL := fmt.Sprintf("?text=%s", text)
	for i := 0; i < totalPages; i++ {
		links[i+1] = fmt.Sprintf("%s&page=%d", baseURL, i+1)
	}

	// Build response
	var nextPage interface{} = nil
	if page+1 < totalPages {
		nextPage = page + 2 // 1-based
	}

	resp := Response{
		FoundRows: len(items),
		Total:     total,
		Items:     items,
		Links:     links,
		Filters: map[string]interface{}{
			"page":     page + 1, // 1-based
			"nextpage": nextPage,
		},
	}

	json.NewEncoder(w).Encode(resp)
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

	err = db.QueryRow("SHOW TABLES").Scan(&tableName)
	if err != nil {
		log.Fatal("Cannot find table:", err)
	}
	fmt.Println("Using table:", tableName)

	port := ":3000"
	if p := os.Getenv("PORT"); p != "" {
		port = ":" + p
	}
	fmt.Println("Server running on", port)
	http.HandleFunc("/", handler)
	log.Fatal(http.ListenAndServe(port, nil))
}
