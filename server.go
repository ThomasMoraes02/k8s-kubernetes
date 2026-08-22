package main

import (
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"time"
)

var startedAt = time.Now()

func main() {
	http.HandleFunc("/healthz", Healthz)
	http.HandleFunc("/configmap", ConfigMap)
	http.HandleFunc("/", Hello)
	http.ListenAndServe(":80", nil)
}

func Hello(w http.ResponseWriter, r *http.Request) {

	name := os.Getenv("NAME")
	age := os.Getenv("AGE")

	fmt.Fprintf(w, "Hello, I'm %s, Age: %s\n", name, age)

	w.Write([]byte("<h1>Hello Full Cycle!</h1>"))
}

func ConfigMap(w http.ResponseWriter, r *http.Request) {
	data, err := ioutil.ReadFile("myfamily/family.txt")
	if err != nil {
		log.Fatalf("Error reading file: %v", err)
	}

	fmt.Fprintf(w, "My family: %s\n", string(data))
}

func Healthz(w http.ResponseWriter, r *http.Request) {
  	duration := time.Since(startedAt)
 
  	if duration.Seconds() < 10 {
    	w.WriteHeader(500)
    	w.Write([]byte(fmt.Sprintf("Duration: %v", duration.Seconds())))
  	} else {
    	w.WriteHeader(200)
    	w.Write([]byte("ok"))
  	}
}

// Comentada para conseguir usar o HPA
func HealthzOld(w http.ResponseWriter, r *http.Request) {
	duration := time.Since(startedAt)

	if duration.Seconds() < 10 || duration.Seconds() > 30 {
		w.WriteHeader(500)
		w.Write([]byte(fmt.Sprintf("Duration: %v", duration.Seconds())))
	} else {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(fmt.Sprintf("ok - uptime: %s", duration)))
	}
}