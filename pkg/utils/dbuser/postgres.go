package dbuser

import (
	"fmt"
)

func NewPosgresUser(username, password, accessType string) DatabaseUser {
	return &PostgresUser{
		Username:   username,
		Password:   password,
		AccessType: accessType,
	}
}

type PostgresUser struct {
	Username   string
	Password   string
	AccessType string
}

func (u *PostgresUser) CreateUser() {
	fmt.Println("Creating a postgrses user")
}
func (u *PostgresUser) UpdateUser() {
	fmt.Println("Updating a postgres user")
}
func (u *PostgresUser) DeleteUser() {}

func (u *PostgresUser) DoesUserExist() (err error) {
	return nil
}
