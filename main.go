package main

import (
	"fmt"
	"github.com/gin-gonic/gin"
	"log"
	"net/http"
	"sort"
	"strconv"
	"sync"
	"time"
)

type Event struct {
	ID               string
	Name             string
	Date             time.Time
	TotalTickets     int
	AvailableTickets int
}

type Ticket struct {
	ID      string
	EventID string
}

type TicketService struct {
	events      sync.Map
	mu          sync.Mutex
	uuidCounter int
	cache       [10]*Event
}

type EventDeserializer struct {
	Name         string    `json:"name"`
	Date         time.Time `json:"date"`
	TotalTickets int       `json:"totalTickets"`
}

func (ts *TicketService) generateUUID() string {
	ts.mu.Lock()
	defer ts.mu.Unlock()
	ts.uuidCounter++
	return strconv.Itoa(ts.uuidCounter)
}

func (ts *TicketService) CreateEvent(name string, date time.Time, totalTickets int) (*Event, error) {
	event := &Event{
		ID:               ts.generateUUID(), // Generate a unique ID for the event
		Name:             name,
		Date:             date,
		TotalTickets:     totalTickets,
		AvailableTickets: totalTickets,
	}
	ts.events.Store(event.ID, event)
	return event, nil
}

func (ts *TicketService) ListEvents() []*Event {
	var events []*Event
	ts.events.Range(func(key, value interface{}) bool {
		event := value.(*Event)
		events = append(events, event)
		return true
	})
	return events
}

func (ts *TicketService) BookTickets(eventID string, numTickets int) ([]string, error) {
	eventIDInt, _ := strconv.Atoi(eventID)
	cacheIndex := eventIDInt % 10
	var event *Event

	if ts.cache[cacheIndex] != nil && ts.cache[cacheIndex].ID == eventID {
		event = ts.cache[cacheIndex]
	} else {
		ts.mu.Lock()
		eventInterface, ok := ts.events.Load(eventID)
		event = eventInterface.(*Event)
		ts.mu.Unlock()
		if !ok {
			return nil, fmt.Errorf("event not found")
		}
		ts.mu.Lock()
		ts.cache[cacheIndex] = event
		ts.mu.Unlock()
	}

	if event.AvailableTickets < numTickets {
		return nil, fmt.Errorf("not enough tickets available")
	}
	var ticketIDs []string
	fmt.Println(numTickets)
	for i := 0; i < numTickets; i++ {
		ticketID := ts.generateUUID()
		ticketIDs = append(ticketIDs, ticketID)
	}

	ts.mu.Lock()
	event.AvailableTickets -= numTickets
	ts.events.Store(eventID, event)
	ts.mu.Unlock()

	return ticketIDs, nil
}

func main() {
	router := gin.Default()

	ticketService := TicketService{}
	concurrencyThreshold := 2
	semaphore := make(chan struct{}, concurrencyThreshold)

	router.POST("/api/v1/events", func(c *gin.Context) {
		semaphore <- struct{}{}
		defer func() { <-semaphore }()

		log.Printf("Received POST request to create event from %s\n", c.ClientIP())

		var eventData EventDeserializer
		err := c.BindJSON(&eventData)
		if err != nil {
			c.String(http.StatusBadRequest, "Error parsing request body")
			return
		}

		event, err := ticketService.CreateEvent(eventData.Name, eventData.Date, eventData.TotalTickets)
		if err != nil {
			c.String(http.StatusInternalServerError, err.Error())
			return
		}

		c.JSON(http.StatusOK, event)
	})

	router.GET("/api/v1/events", func(c *gin.Context) {
		semaphore <- struct{}{}
		defer func() { <-semaphore }()

		log.Printf("Received GET request to list events from %s\n", c.ClientIP())

		events := ticketService.ListEvents()
		sort.Slice(events, func(i, j int) bool {
			firstID, _ := strconv.Atoi(events[i].ID)
			secondID, _ := strconv.Atoi(events[j].ID)
			return firstID < secondID
		})
		c.JSON(http.StatusOK, events)
	})

	router.POST("/api/v1/events/:id/book", func(c *gin.Context) {
		semaphore <- struct{}{}
		defer func() { <-semaphore }()

		log.Printf("Received POST request to book tickets from %s\n", c.ClientIP())

		eventID := c.Param("id")

		numTickets, err := strconv.Atoi(c.DefaultQuery("tickets", "1"))
		if err != nil {
			c.String(http.StatusBadRequest, "Invalid number of tickets")
			return
		}

		ticketIDs, err := ticketService.BookTickets(eventID, numTickets)

		if err != nil {
			c.String(http.StatusInternalServerError, err.Error())
			return
		}

		var tickets []Ticket
		for _, ticketID := range ticketIDs {
			ticketResponse := Ticket{
				EventID: eventID,
				ID:      ticketID,
			}
			tickets = append(tickets, ticketResponse)
		}

		c.JSON(http.StatusOK, tickets)
	})

	router.Use(gin.LoggerWithFormatter(func(param gin.LogFormatterParams) string {
		return fmt.Sprintf("[%s] %s - %s %s %d %s\n",
			param.TimeStamp.Format(time.RFC3339),
			param.ClientIP,
			param.Method,
			param.Path,
			param.StatusCode,
			param.ErrorMessage,
		)
	}))

	// Run the server
	err := router.Run(":8000")
	if err != nil {
		log.Fatal("Server failed to start: ", err)
	}
}
