package dbuser

import "fmt"

func NewMysqlUser(username, password, accessType string) DatabaseUser {
	return &MysqlUser{
		Username:   username,
		Password:   password,
		AccessType: accessType,
	}
}

type MysqlUser struct {
	Username   string
	Password   string
	AccessType string
}

func (u *MysqlUser) CreateUser() {
	fmt.Println("Creating a mysql user")
}
func (u *MysqlUser) UpdateUser() {
	fmt.Println("Creating a mysql user")
}
func (u *MysqlUser) DeleteUser() {}
func (u *MysqlUser) DoesUserExist() (err error) {
	return nil
}
