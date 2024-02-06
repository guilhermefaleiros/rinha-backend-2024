package internal

import (
	"database/sql"
	"github.com/gin-gonic/gin"
	"math"
	"net/http"
	"strconv"
	"time"
)

type TransactionRequest struct {
	Valor     float64 `json:"valor"`
	Descricao string  `json:"descricao"`
	Tipo      string  `json:"tipo"`
}

type BalanceResponse struct {
	Amount        int    `json:"total"`
	Limit         int    `json:"limite"`
	StatementDate string `json:"data_extrato"`
}

type TransactionResponse struct {
	Value       int    `json:"valor"`
	Type        string `json:"tipo"`
	Description string `json:"descricao"`
	DoneAt      string `json:"realizada_em"`
}

type StatementResponse struct {
	Balance      BalanceResponse       `json:"saldo"`
	Transactions []TransactionResponse `json:"ultimas_transacoes"`
}

type ApiHandler struct {
	db           *sql.DB
	cachedLimits map[int]int
}

func (h *ApiHandler) isInvalidRequest(request TransactionRequest) bool {
	return request.Valor == 0 ||
		request.Descricao == "" ||
		(request.Tipo != "c" && request.Tipo != "d") ||
		len(request.Descricao) > 10 ||
		request.Valor != float64(int64(request.Valor))

}

func (h *ApiHandler) InsertTransaction(r *gin.Context) {
	clientId, err := strconv.Atoi(r.Param("id"))
	if err != nil {
		r.JSON(http.StatusBadRequest, gin.H{"error": "invalid client ID"})
		return
	}

	if _, exists := h.cachedLimits[clientId]; !exists {
		r.JSON(http.StatusNotFound, gin.H{"error": "cliente não encontrado"})
		return
	}

	var request TransactionRequest
	if err := r.ShouldBindJSON(&request); err != nil {
		r.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if h.isInvalidRequest(request) {
		r.JSON(http.StatusUnprocessableEntity, gin.H{"error": "invalid request"})
		return
	}

	tx, err := h.db.Begin()
	if err != nil {
		r.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	defer func() {
		if p := recover(); p != nil {
			tx.Rollback()
			panic(p)
		} else if err != nil {
			tx.Rollback()
		} else {
			err = tx.Commit()
		}
	}()

	result, err := tx.Query("SELECT pg_advisory_xact_lock($1)", clientId)
	err = result.Close()
	if err != nil {
		r.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	stmt, err := tx.Prepare("SELECT valor FROM saldos WHERE cliente_id = $1")
	if err != nil {
		r.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	defer stmt.Close()

	var currentAmount int
	if err = stmt.QueryRow(clientId).Scan(&currentAmount); err != nil {
		r.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	newAmount := currentAmount
	if request.Tipo == "d" {
		newAmount -= int(request.Valor)
		if newAmount < 0 && int(math.Abs(float64(newAmount))) > h.cachedLimits[clientId] {
			r.JSON(http.StatusUnprocessableEntity, gin.H{"erro": "saldo insuficiente"})
			return
		}
	} else if request.Tipo == "c" {
		newAmount += int(request.Valor)
	}

	updateStmt, err := tx.Prepare("UPDATE saldos SET valor = $1 WHERE cliente_id = $2")
	if err != nil {
		r.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	defer updateStmt.Close()

	if _, err = updateStmt.Exec(newAmount, clientId); err != nil {
		r.JSON(http.StatusInternalServerError, gin.H{"erro": err.Error()})
		return
	}

	insertStmt, err := tx.Prepare("INSERT INTO transacoes (cliente_id, valor, descricao, tipo) VALUES ($1, $2, $3, $4)")
	if err != nil {
		r.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	defer insertStmt.Close()

	if _, err = insertStmt.Exec(clientId, request.Valor, request.Descricao, request.Tipo); err != nil {
		r.JSON(http.StatusInternalServerError, gin.H{"erro": err.Error()})
		return
	}

	r.JSON(http.StatusOK, gin.H{"limite": h.cachedLimits[clientId], "saldo": newAmount})
}

func (h *ApiHandler) GetStatement(r *gin.Context) {
	clientId, err := strconv.Atoi(r.Param("id"))

	if h.cachedLimits[clientId] == 0 {
		r.JSON(http.StatusNotFound, gin.H{"error": "cliente não encontrado"})
		return
	}

	if err != nil {
		r.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	limit := h.cachedLimits[clientId]

	transactions := make([]TransactionResponse, 0)
	rows, err := h.db.Query("SELECT valor, descricao, tipo, realizada_em FROM transacoes WHERE cliente_id = $1 ORDER BY realizada_em DESC LIMIT 10", clientId)
	for rows.Next() {
		var timestamp time.Time
		var transaction TransactionResponse
		rows.Scan(&transaction.Value, &transaction.Description, &transaction.Type, &timestamp)
		transaction.DoneAt = timestamp.Format(time.RFC3339Nano)
		transactions = append(transactions, transaction)
	}

	r.JSON(http.StatusOK, StatementResponse{
		Balance: BalanceResponse{
			Amount:        0,
			Limit:         limit,
			StatementDate: time.Now().UTC().Format("2006-01-02T15:04:05.000000Z"),
		},
		Transactions: transactions,
	})
	return

}

func NewApiHandler(db *sql.DB, cachedLimits map[int]int) *ApiHandler {

	return &ApiHandler{
		db,
		cachedLimits,
	}
}
