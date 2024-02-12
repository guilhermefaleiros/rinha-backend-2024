package internal

import (
	"database/sql"
	"github.com/gin-gonic/gin"
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

	var request TransactionRequest
	if err := r.ShouldBindJSON(&request); err != nil {
		r.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if h.isInvalidRequest(request) {
		r.JSON(http.StatusUnprocessableEntity, gin.H{"error": "invalid request"})
		return
	}

	stmt, err := h.db.Prepare("SELECT process_transaction($1, $2, $3, $4)")
	if err != nil {
		r.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	var currentAmount int
	if err = stmt.QueryRow(clientId, request.Valor, request.Descricao, request.Tipo).Scan(&currentAmount); err != nil {
		if err.Error() == "pq: 422" {
			r.JSON(http.StatusUnprocessableEntity, gin.H{"error": err.Error()})
		} else if err.Error() == "pq: 404" {
			r.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		} else {
			r.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		}
		return
	}
	if currentAmount == -10000000 {
		r.JSON(http.StatusNotFound, gin.H{"error": "invalid request"})
		return
	}
	if currentAmount == -100000001 {
		r.JSON(http.StatusUnprocessableEntity, gin.H{"error": "invalid request"})
		return
	}
	r.JSON(http.StatusOK, gin.H{"limite": h.cachedLimits[clientId], "saldo": currentAmount})
	return
}

func (h *ApiHandler) GetStatement(r *gin.Context) {
	clientId, err := strconv.Atoi(r.Param("id"))
	if err != nil {
		r.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// Estrutura para armazenar o resultado de uma única linha
	var row struct {
		Limit       int
		Balance     int
		Value       sql.NullInt32 // Use sql.NullInt32 para lidar com possíveis nulos
		Description sql.NullString
		Type        sql.NullString
		DoneAt      sql.NullTime
	}

	transactions := make([]TransactionResponse, 0)
	rows, err := h.db.Query("SELECT c.limite, c.saldo, t.valor, t.descricao, t.tipo, t.realizada_em FROM clientes c LEFT JOIN LATERAL (SELECT valor, descricao, tipo, realizada_em FROM transacoes WHERE cliente_id = c.id ORDER BY id DESC LIMIT 10) t ON true WHERE c.id = $1;", clientId)
	if err != nil {
		r.JSON(http.StatusNotFound, gin.H{"error": "cliente não encontrado"})
		return
	}
	defer rows.Close()

	var limit, balance int
	for rows.Next() {
		err := rows.Scan(&row.Limit, &row.Balance, &row.Value, &row.Description, &row.Type, &row.DoneAt)
		if err != nil {
			r.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		if limit == 0 && balance == 0 {
			limit = row.Limit
			balance = row.Balance
		}

		if row.Value.Valid {
			transactions = append(transactions, TransactionResponse{
				Value:       int(row.Value.Int32),
				Description: row.Description.String,
				Type:        row.Type.String,
				DoneAt:      row.DoneAt.Time.Format(time.RFC3339Nano),
			})
		}
	}

	r.JSON(http.StatusOK, StatementResponse{
		Balance: BalanceResponse{
			Amount:        balance,
			Limit:         limit,
			StatementDate: time.Now().UTC().Format("2006-01-02T15:04:05.000000Z"),
		},
		Transactions: transactions,
	})

}

func NewApiHandler(db *sql.DB, cachedLimits map[int]int) *ApiHandler {

	return &ApiHandler{
		db,
		cachedLimits,
	}
}
