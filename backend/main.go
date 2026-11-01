
package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"path/filepath"
	"strings"
	"time"

	_ "modernc.org/sqlite"
)

type Post struct {
	ID      int    `json:"id"`
	Title   string `json:"title"`
	Content string `json:"content"`
	Author  string `json:"author"`
	Created string `json:"created"`
}

var db *sql.DB

func main() {
	// 从 backend 目录定位到 database/bbs.db
	dbPath := filepath.Join("..", "database", "bbs.db")

	var err error
	db, err = sql.Open("sqlite", dbPath)
	if err != nil {
		log.Fatal("打开数据库失败：", err)
	}
	defer db.Close()

	if err = db.Ping(); err != nil {
		log.Fatal("连接数据库失败：", err)
	}

	// 创建帖子表；如果已存在则不重复创建
	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS posts (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			title TEXT NOT NULL,
			content TEXT NOT NULL,
			author TEXT NOT NULL,
			created TEXT NOT NULL
		)
	`)
	if err != nil {
		log.Fatal("创建帖子表失败：", err)
	}

	http.Handle("/", http.FileServer(http.Dir("../frontend")))
	http.HandleFunc("/api/posts", handlePosts)

	fmt.Println("BBS server running at http://localhost:8080")
	log.Fatal(http.ListenAndServe(":8080", nil))
}

func handlePosts(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")

	switch r.Method {
	case http.MethodGet:
		rows, err := db.Query(`
			SELECT id, title, content, author, created
			FROM posts
			ORDER BY id DESC
		`)
		if err != nil {
			http.Error(w, `{"error":"failed to query posts"}`, http.StatusInternalServerError)
			return
		}
		defer rows.Close()

		posts := []Post{}
		for rows.Next() {
			var p Post
			if err := rows.Scan(&p.ID, &p.Title, &p.Content, &p.Author, &p.Created); err != nil {
				http.Error(w, `{"error":"failed to read posts"}`, http.StatusInternalServerError)
				return
			}
			posts = append(posts, p)
		}
		if err := rows.Err(); err != nil {
			http.Error(w, `{"error":"failed to read posts"}`, http.StatusInternalServerError)
			return
		}

		json.NewEncoder(w).Encode(posts)

	case http.MethodPost:
		var input struct {
			Title   string `json:"title"`
			Content string `json:"content"`
			Author  string `json:"author"`
		}

		r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
		if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
			http.Error(w, `{"error":"invalid JSON"}`, http.StatusBadRequest)
			return
		}

		input.Title = strings.TrimSpace(input.Title)
		input.Content = strings.TrimSpace(input.Content)
		input.Author = strings.TrimSpace(input.Author)

		if input.Title == "" || input.Content == "" || input.Author == "" {
			http.Error(w, `{"error":"author, title and content are required"}`, http.StatusBadRequest)
			return
		}

		created := time.Now().Format("2006-01-02 15:04:05")
		result, err := db.Exec(`
			INSERT INTO posts (title, content, author, created)
			VALUES (?, ?, ?, ?)
		`, input.Title, input.Content, input.Author, created)
		if err != nil {
			http.Error(w, `{"error":"failed to create post"}`, http.StatusInternalServerError)
			return
		}

		id, err := result.LastInsertId()
		if err != nil {
			http.Error(w, `{"error":"failed to get post ID"}`, http.StatusInternalServerError)
			return
		}

		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(Post{
			ID: idToInt(id), Title: input.Title, Content: input.Content,
			Author: input.Author, Created: created,
		})

	default:
		w.Header().Set("Allow", "GET, POST")
		http.Error(w, `{"error":"method not allowed"}`, http.StatusMethodNotAllowed)
	}
}

func idToInt(id int64) int {
	return int(id)
}
