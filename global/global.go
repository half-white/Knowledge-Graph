package global

import (
	"github.com/neo4j/neo4j-go-driver/neo4j"
	"gorm.io/gorm"
)

var (
	DB    neo4j.Driver
	Mysql *gorm.DB
)
