package main

import (
	gimage "github.com/mitroadmaps/gomapinfer/image"

	"encoding/json"
	"fmt"
	"image/jpeg"
	"io/ioutil"
	"log"
	"math"
	"net/http"
	"os"
	"strconv"
	"sort"
	"sync"
)

const Size int = 128

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
	var points [][2]float64
	bytes, err := ioutil.ReadFile("../lights.json")
	if err != nil {
		panic(err)
	}
	if err := json.Unmarshal(bytes, &points); err != nil {
		panic(err)
	}

	log.Printf("insert %d points", len(points))
	goodCells := make(map[[2]int]bool)
	db.Exec("DELETE FROM points")
	tx, err := db.db.Begin()
	checkErr(err)
	for i, p := range points {
		if i % 100 == 0 {
			log.Printf("... %d/%d", i, len(points))
		}

		cell := [2]int{int(math.Floor(p[0]/8192)), int(math.Floor(p[1]/8192))}
		if _, ok := goodCells[cell]; !ok {
			fname := fmt.Sprintf("/mnt/signify/la/sat-jpg/la_%d_%d_sat.jpg", cell[0], cell[1])
			_, err := os.Stat(fname)
			goodCells[cell] = err == nil
		}
		if !goodCells[cell] {
			continue
		}

		x, y := int(p[0])-cell[0]*8192, int(p[1])-cell[1]*8192
		_, err := tx.Exec("INSERT INTO points (x, y, tx, ty, nx, ny) VALUES (?, ?, ?, ?, 0, 0)", x, y, cell[0], cell[1])
		checkErr(err)
	}
	err = tx.Commit()
	checkErr(err)
}

type Point struct {
	ID int
	X int
	Y int
	Tx int
	Ty int
	Nx int
	Ny int
}

func (p Point) Done() bool {
	return p.Nx != 0 || p.Ny != 0
}

func main() {
	if len(os.Args) >= 2 && os.Args[1] == "populate" {
		populateDatabase()
		return
	}

	var points []Point
	cellCounts := make(map[[2]int]int)
	rows := db.Query("SELECT id, x, y, tx, ty, nx, ny FROM points ORDER BY RANDOM()")
	for rows.Next() {
		var p Point
		rows.Scan(&p.ID, &p.X, &p.Y, &p.Tx, &p.Ty, &p.Nx, &p.Ny)
		cell := [2]int{p.Tx, p.Ty}
		if cellCounts[cell] > 10 {
			//continue
		}
		cellCounts[cell]++
		points = append(points, p)
	}
	log.Printf("loaded %d points", len(points))

	sort.Slice(points, func(i, j int) bool {
		if points[i].Tx != points[j].Tx {
			return points[i].Tx < points[j].Tx
		} else if points[i].Ty != points[j].Ty {
			return points[i].Ty < points[j].Ty
		} else if points[i].X != points[j].X {
			return points[i].X < points[j].X
		} else {
			return points[i].Y < points[j].Y
		}
	})

	var mu sync.Mutex
	var curIdx int
	for i, p := range points {
		if p.Done() {
			curIdx = i+1
		}
	}
	var curPix [][][3]uint8

	// caller must have the lock
	setCurIdx := func(idx int) {
		if curPix == nil || points[curIdx].Tx != points[idx].Tx || points[curIdx].Ty != points[idx].Ty {
			// load new satellite image
			fname := fmt.Sprintf("/mnt/signify/la/sat-jpg/la_%d_%d_sat.jpg", points[idx].Tx, points[idx].Ty)
			log.Printf("load %s", fname)
			curPix = gimage.ReadImage(fname)
			log.Printf("ready")
		}
		curIdx = idx
	}
	setCurIdx(curIdx)

	getCenter := func(p Point) (int, int) {
		cx, cy := p.X, p.Y
		if cx < Size {
			cx = Size
		} else if cx > 8192-Size {
			cx = 8192-Size
		}
		if cy < Size {
			cy = Size
		} else if cy > 8192-Size {
			cy = 8192-Size
		}
		return cx, cy
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
		p := points[curIdx]
		pix := curPix
		mu.Unlock()
		cx, cy := getCenter(p)
		crop := gimage.Crop(pix, cx-Size, cy-Size, cx+Size, cy+Size)
		ox, oy := p.X-(cx-Size), p.Y-(cy-Size)
		gimage.DrawRectangle(crop, ox-32, oy-32, ox+32, oy+32, 1, [3]uint8{255, 0, 0})
		if p.Done() {
			ox, oy := p.Nx-(cx-Size), p.Ny-(cy-Size)
			gimage.FillRectangle(crop, ox-2, oy-2, ox+2, oy+2, [3]uint8{255, 255, 0})
		}
		w.Header().Set("Content-Type", "image/jpeg")
		jpeg.Encode(w, gimage.AsImage(crop), nil)
	})
	http.HandleFunc("/get2", func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		p := points[curIdx]
		pix := curPix
		mu.Unlock()
		cx, cy := getCenter(p)
		crop := gimage.Crop(pix, cx-Size, cy-Size, cx+Size, cy+Size)
		w.Header().Set("Content-Type", "image/jpeg")
		jpeg.Encode(w, gimage.AsImage(crop), nil)
	})
	http.HandleFunc("/next", func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		setCurIdx(curIdx+1)
		mu.Unlock()
	})
	http.HandleFunc("/prev", func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		if curIdx > 0 {
			setCurIdx(curIdx-1)
		}
		mu.Unlock()
	})
	http.HandleFunc("/submit", func(w http.ResponseWriter, r *http.Request) {
		r.ParseForm()
		x, _ := strconv.Atoi(r.PostForm.Get("x"))
		y, _ := strconv.Atoi(r.PostForm.Get("y"))
		mu.Lock()
		cx, cy := getCenter(points[curIdx])
		points[curIdx].Nx = cx-Size+x
		points[curIdx].Ny = cy-Size+y
		db.Exec("UPDATE points SET nx = ?, ny = ? WHERE id = ?", points[curIdx].Nx, points[curIdx].Ny, points[curIdx].ID)
		setCurIdx(curIdx+1)
		mu.Unlock()
	})
	if err := http.ListenAndServe(":8080", nil); err != nil {
		panic(err)
	}
}
