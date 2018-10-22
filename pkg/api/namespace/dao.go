package namespace

// Namespace represents a namespace
type Namespace struct {
	ID   int64
	Name string
}

// DAO interface contains all namespace data operations
type DAO interface {
	FindAll() ([]*Namespace, error)
	GetByID(id int64) (*Namespace, error)
	GetByName(name string) (*Namespace, error)
	Create(entity *Namespace) (*Namespace, error)
	Delete(id int64) error
}
