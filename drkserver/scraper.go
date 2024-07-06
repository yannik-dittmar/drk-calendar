package drkserver

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"os"
	"regexp"
	"time"

	"github.com/go-rod/rod"
	db "github.com/yannik.dittmar/drk-server-ical/database"
)

func ScrapeEvents(database *db.Database) {
	page := rod.New().MustConnect().MustPage("https://login.drkserver.org/")

	// Login
	fmt.Println("Logging in...")
	page.MustElement("#username").MustInput(os.Getenv("DRK_LOGIN_USERNAME"))
	page.MustElement("#password").MustInput(os.Getenv("DRK_LOGIN_PASSWORD"))
	page.MustElement("form > div > div > button").MustClick()
	page.MustWaitStable()

	// Setup filters
	// label:has(...) is used to select the element with the pointer-events
	fmt.Println("Setting up filters...")
	page.MustElement("label:has(#filter-my-invites)").MustClick()
	page.MustElement("label:has(#calendar-view-year)").MustClick()
	page.MustWaitStable()

	// Extract ids
	htmlTable := page.MustElement("#calendar-list-view-table-container > table > tbody").MustHTML()
	r, _ := regexp.Compile(`/v/events/(\d+)/details`)
	idMatches := r.FindAllStringSubmatch(htmlTable, -1)
	ids := make([]string, len(idMatches))
	for i, match := range idMatches {
		ids[i] = match[1]
	}

	// Retrieve em-light-auth by navigating to random event
	fmt.Println("Retrieving em-light-auth...")
	page.MustNavigate("https://portal.drkserver.org/v/events/1/details")
	page.MustWaitStable()

	cookieURL := url.URL{Scheme: "https", Host: "portal.drkserver.org"}
	cookies, err := page.Cookies([]string{cookieURL.String()})
	if err != nil {
		log.Fatal(err)
	}

	jar, err := cookiejar.New(nil)
	if err != nil { 
		log.Fatal(err)
	}
	for _, cookie := range cookies {
		jar.SetCookies(&cookieURL, []*http.Cookie{
			{
				Name: cookie.Name,
				Value: cookie.Value,
				Domain: cookie.Domain,
			},
		})
	}

	// Get events async
	fmt.Println("Getting events...")
	eventChan := make(chan db.Event)
	for _, id := range ids {
		go getEvent(id, jar, eventChan)
	}

	// Store events in database
	err = database.DeleteEvents()
	if err != nil {
		log.Fatal(err)
	}
	events := []db.Event{}
	for i := 0; i < len(ids); i++ {
		event := <-eventChan
		events = append(events, event)
	}
	err = database.InsertEvents(events)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Printf("Finished! There are currently %d events in the database.\n", len(events))
}

func getEvent(id string, cookies *cookiejar.Jar, output chan db.Event) {
	url := fmt.Sprintf("https://portal.drkserver.org/v/api/events/%s", id)
	client := &http.Client{
		Jar: cookies,
	}
	resp, err := client.Get(url)
	if err != nil {
		fmt.Println("Error getting event " + id, err)
		output <- db.Event{}
		return
	}
	if resp.StatusCode != 200 {
		fmt.Println("Error getting event " + id, resp.StatusCode)
		output <- db.Event{}
		return
	}

	defer resp.Body.Close()

	var eventJson map[string]interface{}
	err = json.NewDecoder(resp.Body).Decode(&eventJson)
	if err != nil {
		fmt.Println("Error decoding event " + id, err)
		output <- db.Event{}
		return
	}

	event := db.Event{}
	event.Id = int(eventJson["id"].(float64))
	event.Start, _ = time.Parse(time.RFC3339, eventJson["dateFrom"].(string))
	event.End, _ = time.Parse(time.RFC3339, eventJson["dateUpTo"].(string))
	event.Location = ""
	if len(eventJson["locations"].([]interface{})) > 0 {
		if eventJson["locations"].([]interface{})[0].(map[string]interface{})["freeAddressText"] != nil {
			event.Location = eventJson["locations"].([]interface{})[0].(map[string]interface{})["freeAddressText"].(string)
		} else if eventJson["locations"].([]interface{})[0].(map[string]interface{})["addressContact"] != nil {
			location := eventJson["locations"].([]interface{})[0].(map[string]interface{})["addressContact"].(map[string]interface{})["location"].(map[string]interface{})
			event.Location = location["street"].(string) + " " + location["streetnumber"].(string) + ", " + location["zipCode"].(string) + " " + location["city"].(string)
		}
	}
	event.Summary = eventJson["extendedDescription"].(string)
	descriptionString := ""

	if eventJson["description"] != nil {
		descriptionString += eventJson["description"].(map[string]interface{})["value2"].(string) + " - "
	}
	if eventJson["extendedDescription"] != nil {
		descriptionString += eventJson["extendedDescription"].(string) + "\n\n"
	}

	if eventJson["meetingpoint"] != nil && eventJson["meetingpointTime"] != nil {
		meetTime, _ := time.Parse(time.RFC3339, eventJson["meetingpointTime"].(string))
		descriptionString += "Treffpunkt: " + meetTime.Format("15:04") + " Uhr - " + eventJson["meetingpoint"].(string) + "\n"
	}

	if eventJson["dressCodes"] != nil {
		if len(eventJson["dressCodes"].([]interface{})) > 0 {
			descriptionString += "Kleiderordnung: " + eventJson["dressCodes"].([]interface{})[0].(map[string]interface{})["value2"].(string) + "\n"
		}
	}

	if eventJson["caterings"] != nil {
		if len(eventJson["caterings"].([]interface{})) > 0 {
			descriptionString += "Verpflegung: " + eventJson["caterings"].([]interface{})[0].(map[string]interface{})["value2"].(string) + "\n"
		}
	}

	event.Description = descriptionString
		
	output <- event
}