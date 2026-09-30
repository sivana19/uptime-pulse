package main

import (
	"database/sql"
	"log"
	"net/http"
	"os"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	_ "github.com/lib/pq"

	"uptime-pulse/internal/monitor"
	"uptime-pulse/internal/storage"
)

type CreateMonitorRequest struct {
	URL string `json:"url" binding:"required,url"`
}

func main() {
	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		dbURL = "postgres://postgres:postgres@localhost:5432/expense_tracker?sslmode=disable"
	}

	db, err := sql.Open("postgres", dbURL)
	if err != nil {
		log.Fatalf("open db: %v", err)
	}
	defer db.Close()

	store := storage.NewStorage(db)
	if err := store.InitSchema(); err != nil {
		log.Fatalf("init schema: %v", err)
	}

	scanner := monitor.NewScanner(3 * time.Second)
	router := gin.Default()

	// 1. Register a new URL to track
	router.POST("/monitors", func(c *gin.Context) {
		var req CreateMonitorRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "valid 'url' field is required"})
			return
		}

		id, err := store.CreateMonitor(req.URL)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		c.JSON(http.StatusCreated, gin.H{"id": id, "url": req.URL})
	})

	// 2. List active monitors
	router.GET("/monitors", func(c *gin.Context) {
		monitors, err := store.GetAllMonitors()
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, monitors)
	})

	// 3. View metrics & uptime history
	router.GET("/monitors/:id/metrics", func(c *gin.Context) {
		idStr := c.Param("id")
		id, err := strconv.ParseInt(idStr, 10, 64)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid monitor ID"})
			return
		}

		summary, err := store.GetMetrics(id)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, summary)
	})

	// 4. Manual scan trigger
	router.POST("/monitors/scan", func(c *gin.Context) {
		monitors, err := store.GetAllMonitors()
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		urls := make([]string, len(monitors))
		for i, m := range monitors {
			urls[i] = m.URL
		}

		results := scanner.ScanAllConcurrent(urls)

		for _, res := range results {
			for _, m := range monitors {
				if m.URL == res.URL {
					_ = store.SaveCheckResult(m.ID, res)
				}
			}
		}

		c.JSON(http.StatusOK, gin.H{
			"total_scanned": len(results),
			"results":       results,
		})
	})

	// Background worker: automatically runs every 60 seconds
	go func() {
		ticker := time.NewTicker(60 * time.Second)
		defer ticker.Stop()

		for range ticker.C {
			monitors, err := store.GetAllMonitors()
			if err != nil || len(monitors) == 0 {
				continue
			}

			urls := make([]string, len(monitors))
			for i, m := range monitors {
				urls[i] = m.URL
			}

			results := scanner.ScanAllConcurrent(urls)
			for _, res := range results {
				for _, m := range monitors {
					if m.URL == res.URL {
						_ = store.SaveCheckResult(m.ID, res)
					}
				}
			}
			log.Printf("[Worker] Completed background health check for %d targets", len(results))
		}
	}()

	log.Println("Uptime Pulse running on :8081")
	if err := router.Run(":8081"); err != nil {
		log.Fatalf("server error: %v", err)
	}
}
