package dbuser

// DatabaseUser is interface for CRUD operate of different types of databases
type DatabaseUser interface {
	CreateUser()
	UpdateUser()
	DeleteUser()
	DoesUserExist() error
}

const ERR_USER_NOT_FOUND = "USER NOT FOUND"
