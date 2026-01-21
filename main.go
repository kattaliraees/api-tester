package main

import (
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"strconv"
	"sync"
	"time"
)

type DeviceState struct {
	ID    string
	Value bool
}

type GPSLocation struct {
	ID  string
	Lat float64
	Lon float64
}

var (
	gpsLocations = make(map[string]GPSLocation)
	gpsMutex     sync.Mutex
	devices      = make(map[string]bool)
	mutex        sync.Mutex
)

func gpsHandler(w http.ResponseWriter, r *http.Request) {
	// log.Printf("Received GPS request: %v", r.URL.Query())
	id := r.URL.Query().Get("id")
	latStr := r.URL.Query().Get("lat")
	lonStr := r.URL.Query().Get("lon")

	if id == "" {
		http.Error(w, "Missing id param", http.StatusBadRequest)
		return
	}

	lat, err := strconv.ParseFloat(latStr, 64)
	if err != nil {
		http.Error(w, "Invalid lat param", http.StatusBadRequest)
		return
	}

	lon, err := strconv.ParseFloat(lonStr, 64)
	if err != nil {
		http.Error(w, "Invalid lon param", http.StatusBadRequest)
		return
	}

	gpsMutex.Lock()
	gpsLocations[id] = GPSLocation{ID: id, Lat: lat, Lon: lon}
	gpsMutex.Unlock()

	log.Printf("GPS Update: Device %s at %.6f, %.6f\n", id, lat, lon)

	fmt.Fprintf(w, "GPS updated for %s: %.6f, %.6f\n", id, lat, lon)
}

func updateHandler(w http.ResponseWriter, r *http.Request) {
	// log.Printf("Received Update request: %v", r.URL.Query())
	id := r.URL.Query().Get("id")
	val := r.URL.Query().Get("value")

	if id == "" {
		http.Error(w, "Missing id param", http.StatusBadRequest)
		return
	}

	if val == "" {
		http.Error(w, "Missing value param", http.StatusBadRequest)
		return
	}

	parsed, err := strconv.ParseBool(val)
	if err != nil {
		http.Error(w, "Invalid boolean value", http.StatusBadRequest)
		return
	}

	mutex.Lock()
	devices[id] = parsed
	mutex.Unlock()

	log.Printf("Device %s -> %v\n", id, parsed)

	fmt.Fprintf(w, "Device %s set to %v\n", id, parsed)
}

func getOutboundIP() net.IP {
	conn, err := net.Dial("udp", "8.8.8.8:80")
	if err != nil {
		log.Fatal(err)
	}
	defer conn.Close()

	localAddr := conn.LocalAddr().(*net.UDPAddr)

	return localAddr.IP
}

func main() {
	currentTime := time.Now().Format("2006-01-02_15:04:05")
	logFileName := fmt.Sprintf("server_%s.log", currentTime)
	f, err := os.OpenFile(logFileName, os.O_RDWR|os.O_CREATE|os.O_APPEND, 0666)
	if err != nil {
		log.Fatalf("error opening file: %v", err)
	}
	defer f.Close()
	wrt := io.MultiWriter(os.Stdout, f)
	log.SetOutput(wrt)
	log.SetFlags(log.LstdFlags)

	http.HandleFunc("/update", updateHandler)
	http.HandleFunc("/gps", gpsHandler)

	ip := getOutboundIP()
	log.Printf("Server running on %s:8080\n", ip.String())
	log.Fatal(http.ListenAndServe(":8080", nil))
}
