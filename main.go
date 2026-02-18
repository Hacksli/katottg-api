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
const maxPerPage = 100

type PageLink struct {
	Page    interface{} `json:"page"`
	Current string      `json:"current,omitempty"`
	Class   string      `json:"class,omitempty"`
}

type Response struct {
	FoundRows int                      `json:"foundRows"`
	Total     int                      `json:"total"`
	Items     []map[string]interface{} `json:"items"`
	Links     []PageLink               `json:"links"`
	Filters   map[string]interface{}   `json:"filters"`
}

// getPagesBar replicates the PHP SysMethods::getPagesBar logic
// currentPage is 0-based internally
func getPagesBar(total, pageLimit, currentPage int) []PageLink {
	totalPages := int(math.Ceil(float64(total) / float64(pageLimit)))
	var res []PageLink

	if totalPages > 7 {
		// First page
		if currentPage != 0 {
			res = append(res, PageLink{Page: 1})
		} else {
			res = append(res, PageLink{Page: 1, Current: "true"})
			res = append(res, PageLink{Page: 2})
		}

		if currentPage > 3 {
			res = append(res, PageLink{Page: "...", Class: ""})
		}

		// Previous before current but not first
		if currentPage > 1 && currentPage != totalPages {
			res = append(res, PageLink{Page: currentPage})
		}

		// Current if not first and not last
		if currentPage != 0 && currentPage != totalPages-1 {
			res = append(res, PageLink{Page: currentPage + 1, Current: "true"})
		}

		// Next after current but not last
		if currentPage > 0 && currentPage != totalPages-1 {
			res = append(res, PageLink{Page: currentPage + 2})
		}

		// Last page
		if currentPage != totalPages-2 {
			if currentPage != totalPages-1 {
				res = append(res, PageLink{Page: "...", Class: ""})
			}
			if currentPage != totalPages-1 {
				res = append(res, PageLink{Page: totalPages})
			} else {
				res = append(res, PageLink{Page: totalPages, Current: "true"})
			}
		}
	} else {
		for i := 0; i < totalPages; i++ {
			link := PageLink{Page: i + 1, Current: "false"}
			if i == currentPage {
				link.Current = "true"
			}
			res = append(res, link)
		}
	}

	return res
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
	if perPage > maxPerPage {
		perPage = maxPerPage
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

	// Build links using getPagesBar (same logic as PHP)
	links := getPagesBar(total, perPage, page)

	// Build filters (nextpage logic from PHP)
	var nextPage interface{} = nil
	if page+1 < len(links) && links[page+1].Page != "..." {
		nextPage = page + 2
	} else if page+1 < totalPages {
		nextPage = page + 2
	}

	resp := Response{
		FoundRows: len(items),
		Total:     total,
		Items:     items,
		Links:     links,
		Filters: map[string]interface{}{
			"page":     page + 1,
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
