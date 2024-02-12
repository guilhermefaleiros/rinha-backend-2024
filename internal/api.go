package internal

import (
	"database/sql"
	"github.com/gin-gonic/gin"
	_ "github.com/lib/pq"
	"os"
	"time"
)

func StartAPI() {
	r := gin.Default()

	db, err := sql.Open("postgres", "host=localhost port=5432 user=admin password=123 dbname=rinha sslmode=disable")
	db.SetMaxOpenConns(30)
	db.SetConnMaxLifetime(5 * time.Minute)

	time.Sleep(5 * time.Second)

	cachedLimits := make(map[int]int)

	rows, err := db.Query("SELECT id, limite FROM clientes")
	if err != nil {
		panic(err)
	}
	for rows.Next() {
		var id, limit int
		err = rows.Scan(&id, &limit)
		if err != nil {
			panic(err)
		}
		cachedLimits[id] = limit
	}

	handler := NewApiHandler(db, cachedLimits)

	r.POST("/clientes/:id/transacoes", handler.InsertTransaction)
	r.GET("/clientes/:id/extrato", handler.GetStatement)

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	err = r.Run(":" + port)
	if err != nil {
		panic(err)
	}
}
