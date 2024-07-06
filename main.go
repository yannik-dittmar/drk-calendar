package main

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"strconv"
	"time"

	ics "github.com/arran4/golang-ical"
	"github.com/joho/godotenv"
	_ "github.com/mattn/go-sqlite3"
	database "github.com/yannik.dittmar/drk-server-ical/database"
	scraper "github.com/yannik.dittmar/drk-server-ical/drkserver"
)


func parseCalendarEvents(database *database.Database) (string, error) {
	events, err := database.GetEvents()
	if (err != nil) {
		return "", err
	}
	
	cal := ics.NewCalendar()
	for _, event := range events {
		icalEvent := cal.AddEvent(strconv.Itoa(event.Id))
		icalEvent.SetStartAt(event.Start)
		icalEvent.SetEndAt(event.End)
		icalEvent.SetLocation(event.Location)
		icalEvent.SetSummary(event.Summary)
		icalEvent.SetDescription(event.Description)
		icalEvent.SetURL(fmt.Sprintf("https://portal.drkserver.org/v/events/%d/details", event.Id))
	}

	return cal.Serialize(), nil
}


func main() {
	err := godotenv.Load()
	if err != nil {
		fmt.Println("Warning: Could not load .env file!")
	}

	if len(os.Getenv("TOKEN")) == 0 {
		log.Fatal("Error: No token provided!")
	}

	if len(os.Getenv("DRK_LOGIN_USERNAME")) == 0 || len(os.Getenv("DRK_LOGIN_PASSWORD")) == 0 {
		log.Fatal("Error: No DRK login credentials provided!")
	}

	database, err := database.NewDatabase()
	if err != nil {
		log.Fatal(err)
	}

	scrapeTicker := time.NewTicker(3 * time.Hour)
	go func() {
		scraper.ScrapeEvents(database)
		for {
			select {
			case <-scrapeTicker.C:
				scraper.ScrapeEvents(database)
			}
		}
	}()

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		token := r.URL.Query().Get("token")
		if token != os.Getenv("TOKEN") {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}

		fmt.Sprintln("Request from " + r.RemoteAddr + " accepted.")

		icalString, err := parseCalendarEvents(database)
		if err != nil {
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			fmt.Println(err)
			return
		}
		w.Header().Set("Content-Type", "text/calendar")
		w.Header().Set("Content-Disposition", "attachment; filename=calendar.ics")
		fmt.Fprint(w, icalString)
	})

	fmt.Println("Starting server on port 8080...")
	log.Fatal(http.ListenAndServe(":8080", nil))
}