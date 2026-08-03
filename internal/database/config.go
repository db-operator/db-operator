package database

type Engine string

const (
	Postgres Engine = "postgres"
	MySQL    Engine = "mysql"
	Dummy    Engine = "dummy"
)

type Config struct {
	Engine Engine

	Host string
	Port int

	Username string
	Password string

	Database string
}
