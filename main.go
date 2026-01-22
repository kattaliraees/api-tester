package main

import (
	_ "embed"
	"encoding/json"
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

//go:embed index.html
var indexHTML []byte

type DeviceState struct {
	ID    string
	Value bool
}

type GPSLocation struct {
	ID  string
	Lat float64
	Lon float64
}

// SSE Event Structure
type SSEMessage struct {
	Type    string `json:"type"`
	Message string `json:"message"`
}

// Broker manages SSE clients
type Broker struct {
	Notifier       chan []byte
	newClients     chan chan []byte
	closingClients chan chan []byte
	clients        map[chan []byte]bool
}

func NewBroker() *Broker {
	broker := &Broker{
		Notifier:       make(chan []byte, 1),
		newClients:     make(chan chan []byte),
		closingClients: make(chan chan []byte),
		clients:        make(map[chan []byte]bool),
	}
	go broker.listen()
	return broker
}

func (broker *Broker) listen() {
	for {
		select {
		case s := <-broker.newClients:
			broker.clients[s] = true
			log.Printf("Client added. Total: %d", len(broker.clients))
		case s := <-broker.closingClients:
			delete(broker.clients, s)
			log.Printf("Client removed. Total: %d", len(broker.clients))
		case event := <-broker.Notifier:
			for clientMessageChan := range broker.clients {
				select {
				case clientMessageChan <- event:
				default:
					// Drop message if client is blocked
				}
			}
		}
	}
}

func (broker *Broker) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	messageChan := make(chan []byte)
	broker.newClients <- messageChan

	defer func() {
		broker.closingClients <- messageChan
	}()

	notify := r.Context().Done()

	for {
		select {
		case <-notify:
			return
		case msg := <-messageChan:
			fmt.Fprintf(w, "data: %s\n\n", msg)
			w.(http.Flusher).Flush()
		}
	}
}

var (
	gpsLocations = make(map[string]GPSLocation)
	gpsMutex     sync.Mutex
	devices      = make(map[string]bool)
	mutex        sync.Mutex
	broker       *Broker
)

func broadcast(msgType, msgContent string) {
	msg := SSEMessage{Type: msgType, Message: msgContent}
	jsonMsg, _ := json.Marshal(msg)
	broker.Notifier <- jsonMsg
}

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

	logMsg := fmt.Sprintf("Location update received for %s %.6f, %.6f", id, lat, lon)
	log.Println(logMsg)
	broadcast("gps", logMsg)

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

	var logMsg string
	if parsed {
		logMsg = fmt.Sprintf("Attendance registered for %s", id)
	} else {
		logMsg = fmt.Sprintf("Attendance unregistered for %s", id)
	}
	log.Println(logMsg)
	broadcast("update", logMsg)

	fmt.Fprintf(w, "Device %s set to %v\n", id, parsed)
}

func getOutboundIP() net.IP {
	conn, err := net.Dial("udp", "8.8.8.8:80")
	if err != nil {
		// Fallback if no internet
		return net.IPv4(127, 0, 0, 1)
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

	broker = NewBroker()

	http.HandleFunc("/update", updateHandler)
	http.HandleFunc("/gps", gpsHandler)
	http.Handle("/events", broker)

	// Serve embedded index.html at root
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		w.Write(indexHTML)
	})

	ip := getOutboundIP()
	log.Printf("Server running on %s:8080\n", ip.String())
	log.Fatal(http.ListenAndServe(":8080", nil))
}
