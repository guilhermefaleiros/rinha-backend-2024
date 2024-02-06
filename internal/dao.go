package internal

import "database/sql"

type DAO struct {
	db *sql.DB
}

func (d *DAO) SaveClient(client *Client) error {
	return nil
}

func (d *DAO) GetClientById(id string) (*Client, error) {
	return nil, nil
}

func NewDAO(db *sql.DB) *DAO {
	return &DAO{
		db,
	}
}
