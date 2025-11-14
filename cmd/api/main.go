// cmd/api/main.go

package main

import (
	"context"
	"errors"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/Kovalyovv/chat-app/internal/store" // <-- ЗАМЕНИ kova/chat-app на твой модуль из go.mod!
	"github.com/gin-gonic/gin"
)

func main() {
	log.Println("Starting chat application...")

	// Получаем строку подключения к БД из переменных окружения
	dbUrl := os.Getenv("DB_SOURCE")
	if dbUrl == "" {
		log.Fatal("DB_SOURCE environment variable is not set")
	}

	// Создаем хранилище и подключаемся к БД
	db, err := store.New(dbUrl)
	if err != nil {
		log.Fatalf("failed to connect to database: %v", err)
	}
	defer db.Close()

	// Проверяем соединение с БД
	if err := db.Ping(context.Background()); err != nil {
		log.Fatalf("failed to ping database: %v", err)
	}
	log.Println("Database connection established")

	// --- Настройка Gin ---
	// Включаем release mode, чтобы логи Gin не засоряли консоль
	// gin.SetMode(gin.ReleaseMode)
	router := gin.Default() // gin.Default() идет с middleware для логгирования и recovery

	// Простой health-check эндпоинт
	router.GET("/", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"status": "API is running",
		})
	})

	// Получаем порт из переменных окружения
	port := os.Getenv("API_PORT")
	if port == "" {
		port = "8080"
	}

	srv := &http.Server{
		Addr:    ":" + port,
		Handler: router,
	}

	// --- Graceful Shutdown ---

	go func() {
		log.Printf("Server is listening on port %s", port)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("listen: %s\n", err)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	log.Println("Shutting down server...")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		log.Fatalf("Server forced to shutdown: %v", err)
	}

	log.Println("Server exiting")
}
