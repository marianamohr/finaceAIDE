package main

import (
	"fmt"
	"log"
	"os"

	"github.com/marianamohr/financas/migrations"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"
)

func main() {
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		log.Fatal("DATABASE_URL is required")
	}

	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{
		Logger: gormlogger.Default.LogMode(gormlogger.Info),
	})
	if err != nil {
		log.Fatalf("connect db: %v", err)
	}

	if err := migrations.Run(db); err != nil {
		log.Fatalf("migrations: %v", err)
	}

	fmt.Println("migrations applied successfully")
}
