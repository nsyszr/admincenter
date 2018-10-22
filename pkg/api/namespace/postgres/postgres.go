package postgres

import (
	"database/sql"

	"github.com/nsyszr/admincenter/pkg/api/namespace"
)

type DAO struct {
	db *sql.DB
}

func NewDAO(db *sql.DB) *DAO {
	return &DAO{
		db: db,
	}
}

func (dao *DAO) FindAll() ([]*namespace.Namespace, error) {
	return nil, nil
}

func (dao *DAO) GetByID(id int64) (*namespace.Namespace, error) {
	return nil, nil
}

func (dao *DAO) GetByName(name string) (*namespace.Namespace, error) {
	return nil, nil
}

func (dao *DAO) Create(entity *namespace.Namespace) (*namespace.Namespace, error) {
	return nil, nil
}

func (dao *DAO) Delete(id int64) error {
	return nil
}
