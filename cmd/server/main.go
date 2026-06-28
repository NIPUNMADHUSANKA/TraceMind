package main

import (
	"log"
	"os"
	"os/signal"
	"syscall"
	"tracemind/internal/api"
	"tracemind/internal/queue"
	"tracemind/internal/store"
	"tracemind/internal/worker"

	"github.com/gofiber/fiber/v2"
	"github.com/joho/godotenv"
)

func main() {
	if err := godotenv.Load("../../.env"); err != nil {
		log.Println("No .env file found")
	}

	app := fiber.New()

	dbConnection, err := store.NewPostgresStore(os.Getenv("DATABASE_URL"))
	if err != nil {
		log.Printf("Database connection issue: %s", err.Error())
	} else {
		log.Println("Database connection successful")
	}
	var dbConn store.PostgresStore = *dbConnection

	q := queue.NewQueue(100)
	stopCh := make(chan struct{})
	worker.StartWorker(q, dbConn, stopCh)

	app.Get("/", func(c *fiber.Ctx) error {
		return c.JSON(fiber.Map{
			"status":  "ok",
			"message": "TraceMind Fiber app is running",
		})
	})

	apiGroup := app.Group("/api")
	apiGroup.Post("/ingest", api.IngestHandler(dbConn, q))
	apiGroup.Get("/incidents", api.IncidentsHandler(dbConn))
	apiGroup.Get("/incidents/:id", api.IncidentGetHandler(dbConn))
	apiGroup.Get("health/ingestion", api.HealthHandler(q, dbConn))

	go func() {
		c := make(chan os.Signal, 1)
		signal.Notify(c, os.Interrupt, syscall.SIGTERM)
		<-c
		log.Println("shutting down")
		close(stopCh)
		app.Shutdown()
	}()

	log.Printf("listening :%s", os.Getenv("PORT"))
	app.Listen(":" + os.Getenv("PORT"))

}
