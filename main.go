package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"math/rand"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"
)

var (
	listenAddr string
)

type application struct {
	logger      *log.Logger
	waitGroup   *sync.WaitGroup
	workerGroup map[int]*Worker
}

type Job struct {
	id int
}

type Worker struct {
	job    Job
	ticker *time.Ticker
	closed chan bool
	output string
}

func (app *application) routes() *http.ServeMux {

	mux := http.NewServeMux()
	mux.HandleFunc("/", app.index)
	mux.HandleFunc("/register", app.register)
	mux.HandleFunc("/unregister", app.unregister)
	mux.HandleFunc("/status", app.status)
	return mux
}

func (app *application) createServer() *http.Server {

	server := &http.Server{
		Addr:         listenAddr,
		Handler:      app.routes(),
		ErrorLog:     app.logger,
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 10 * time.Second,
		IdleTimeout:  15 * time.Second,
	}

	return server
}

func (app *application) shutdown(server *http.Server, quit chan os.Signal, done chan<- bool) {

	signal.Notify(quit, os.Interrupt, syscall.SIGTERM)
	<-quit
	app.logger.Println("Server is shutting down...")
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer func() {
		cancel()
	}()

	app.logger.Printf("\nShutdown with timeout: %s\n", 30*time.Second)

	server.SetKeepAlivesEnabled(true)

	if err := server.Shutdown(ctx); err != nil {
		app.logger.Fatalf("Could not gracefully shutdown the server: %v\n", err)
	}

	close(done)
}

func (app *application) index(w http.ResponseWriter, r *http.Request) {

	message := "Index " + strings.TrimPrefix(r.URL.Path, "/index/")

	app.logger.Println(message)

	w.Write([]byte(message))
}

func (app *application) register(w http.ResponseWriter, r *http.Request) {

	job := Job{rand.Intn(999)}

	worker := Worker{
		job:    job,
		ticker: time.NewTicker(10 * time.Second),
		closed: make(chan bool),
	}

	app.workerGroup[job.id] = &worker
	fmt.Println("ADD workerGroup map contents:", app.workerGroup)

	go worker.start(app.waitGroup)

	message := "Register " + strings.TrimPrefix(r.URL.Path, "/register/")

	app.logger.Println(message)

	w.Write([]byte(message))
}

func (app *application) unregister(w http.ResponseWriter, r *http.Request) {

	id := r.URL.Query().Get("id")
	jobId, _ := strconv.Atoi(id)

	if worke_p, ok := app.workerGroup[jobId]; ok {
		delete(app.workerGroup, jobId)
		fmt.Println("DELETE workerGroup map contents:", app.workerGroup)
		worke_p.closed <- true
	}

	message := "Unregister " + strings.TrimPrefix(r.URL.Path, "/unregister/") + "__" + id

	app.logger.Println(message)

	w.Write([]byte(message))
}

func (app *application) status(w http.ResponseWriter, r *http.Request) {

	id := r.URL.Query().Get("id")
	jobId, _ := strconv.Atoi(id)

	message := "Status " + strings.TrimPrefix(r.URL.Path, "/status/")

	if worke_p, ok := app.workerGroup[jobId]; ok {

		message = "Status " + strings.TrimPrefix(r.URL.Path, "/status/") + "\n" + "job_id:" + id + " with content: \n" + worke_p.output
	}

	app.logger.Println(message)

	w.Write([]byte(message))
}

func handle() string {

	cmd := exec.Command("ls", "-lah")
	out, err := cmd.CombinedOutput()
	if err != nil {
		log.Fatalf("cmd.Run() failed with %s\n", err)
	}
	return string(out)
}

func (w *Worker) start(waitGroup *sync.WaitGroup) {

	for {
		select {
		case <-w.ticker.C:
			waitGroup.Add(1)

			currentTimeDate := time.Now()
			currentTime := currentTimeDate.Format("15:04:05")
			output := handle()
			fmt.Printf("job id : %d \ntime : %s\ncombined out:\n%s\n", w.job.id, currentTime, output)
			w.output = currentTime + "\n" + output

			waitGroup.Done()

		case <-w.closed:
			w.ticker.Stop()
			fmt.Println("finishing... " + strconv.Itoa(w.job.id))
			return
		}
	}
}

func main() {

	flag.StringVar(&listenAddr, "listen-addr", ":5000", "server listen address")
	flag.Parse()

	logger := log.New(os.Stdout, "http: ", log.LstdFlags)
	waitGroup := sync.WaitGroup{}

	app := &application{
		logger:      logger,
		waitGroup:   &waitGroup,
		workerGroup: make(map[int]*Worker),
	}

	done := make(chan bool, 1)
	quit := make(chan os.Signal, 1)

	server := app.createServer()
	go app.shutdown(server, quit, done)

	logger.Println("Server is ready at", listenAddr)
	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		logger.Fatalf("Could not listen on %s: %v\n", listenAddr, err)
	}
	waitGroup.Wait()
	<-done
	logger.Println("Server stopped")

}
