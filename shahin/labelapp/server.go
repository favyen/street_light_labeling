package main

import (
	gimage "github.com/mitroadmaps/gomapinfer/image"

	"encoding/json"
	"image/jpeg"
	"io/ioutil"
	"log"
	"math/rand"
	"net/http"
	"os"
	"strconv"
	"sync"
)

const ImageDir = "training_v1/"
const JsonPath = "training_v1.json"

func jsonResponse(w http.ResponseWriter, x interface{}) {
	bytes, err := json.Marshal(x)
	if err != nil {
		panic(err)
	}
	w.Header().Set("Content-Type", "application/json")
	w.Write(bytes)
}

func populateDatabase() {
	log.Printf("read points")
	var points map[string][][2]int
	bytes, err := ioutil.ReadFile(JsonPath)
	if err != nil {
		panic(err)
	}
	if err := json.Unmarshal(bytes, &points); err != nil {
		panic(err)
	}

	log.Printf("insert points for %d files", len(points))
	db.Exec("DELETE FROM orig_points")
	db.Exec("DELETE FROM files")
	tx, err := db.db.Begin()
	checkErr(err)
	var fnames []string
	for fname := range points {
		fnames = append(fnames, fname)
	}
	for i, idx := range rand.Perm(len(fnames)) {
		if i % 100 == 0 {
			log.Printf("... %d/%d", i, len(fnames))
		}
		fname := fnames[idx]
		_, err := tx.Exec("INSERT INTO files (fname) VALUES (?)", fname)
		checkErr(err)
		for _, p := range points[fname] {
			_, err := tx.Exec("INSERT INTO orig_points (fname, x, y) VALUES (?, ?, ?)", fname, p[0], p[1])
			checkErr(err)
		}
	}
	err = tx.Commit()
	checkErr(err)
}

func main() {
	if len(os.Args) >= 2 {
		if os.Args[1] == "populate" {
			populateDatabase()
		} else if os.Args[1] == "dump" {
			points := make(map[string][][2]int)
			rows := db.Query("SELECT fname, x, y FROM label_points")
			for rows.Next() {
				var fname string
				var x, y int
				rows.Scan(&fname, &x, &y)
				points[fname] = append(points[fname], [2]int{x, y})
			}
			bytes, err := json.Marshal(points)
			if err != nil {
				panic(err)
			}
			if err := ioutil.WriteFile(os.Args[2], bytes, 0644); err != nil {
				panic(err)
			}
		}
		return
	}

	var mu sync.Mutex
	var fnames []string
	var curIdx int = 0

	rows := db.Query("SELECT fname FROM files ORDER BY id")
	for rows.Next() {
		var fname string
		rows.Scan(&fname)
		fnames = append(fnames, fname)
	}
	log.Printf("loaded %d filenames", len(fnames))

	var maxID *int
	db.QueryRow("SELECT IFNULL(MAX(f.id), 0) FROM files AS f, label_points AS p WHERE f.fname = p.fname").Scan(&maxID)
	if maxID != nil {
		db.QueryRow("SELECT COUNT(*) FROM files WHERE id <= ?", *maxID).Scan(&curIdx)
		log.Printf("initialize curidx = %d", curIdx)
	}

	fileServer := http.FileServer(http.Dir("static/"))
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/" {
			w.Header().Set("Cache-Control", "no-cache")
		}
		fileServer.ServeHTTP(w, r)
	})
	http.HandleFunc("/get1", func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		fname := fnames[curIdx]
		mu.Unlock()
		pix := gimage.ReadImage(ImageDir + fname)
		rows := db.Query("SELECT x, y FROM orig_points WHERE fname = ?", fname)
		for rows.Next() {
			var x, y int
			rows.Scan(&x, &y)
			gimage.DrawRectangle(pix, x-32, y-32, x+32, y+32, 1, [3]uint8{255, 0, 0})
		}
		rows = db.Query("SELECT x, y FROM label_points WHERE fname = ?", fname)
		for rows.Next() {
			var x, y int
			rows.Scan(&x, &y)
			gimage.FillRectangle(pix, x-5, y-5, x+5, y+5, [3]uint8{255, 255, 0})
		}
		w.Header().Set("Content-Type", "image/jpeg")
		jpeg.Encode(w, gimage.AsImage(pix), nil)
	})
	http.HandleFunc("/get2", func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		fname := fnames[curIdx]
		mu.Unlock()
		http.ServeFile(w, r, ImageDir + fname)
	})
	http.HandleFunc("/next", func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		curIdx = (curIdx+1) % len(fnames)
		mu.Unlock()
	})
	http.HandleFunc("/prev", func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		if curIdx > 0 {
			curIdx--
		}
		mu.Unlock()
	})
	http.HandleFunc("/submit", func(w http.ResponseWriter, r *http.Request) {
		r.ParseForm()
		x, _ := strconv.Atoi(r.PostForm.Get("x"))
		y, _ := strconv.Atoi(r.PostForm.Get("y"))
		mu.Lock()
		fname := fnames[curIdx]
		db.Exec("INSERT INTO label_points (fname, x, y) VALUES (?, ?, ?)", fname, x, y)
		mu.Unlock()
	})
	http.HandleFunc("/clear", func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		fname := fnames[curIdx]
		db.Exec("DELETE FROM label_points WHERE fname = ?", fname)
		mu.Unlock()
	})
	log.Println("ready")
	if err := http.ListenAndServe(":8080", nil); err != nil {
		panic(err)
	}
}
