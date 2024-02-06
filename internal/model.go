package internal

type Client struct {
	ID    int    `json:"id"`
	Name  string `json:"nome"`
	Limit int    `json:"limite"`
}
