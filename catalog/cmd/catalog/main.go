package main

import (
	"log"
	"time"

	"github.com/joho/godotenv"
	"github.com/kelseyhightower/envconfig"
	"github.com/tinrab/retry"
	"github.com/trunglq04/go-grpc-graphql/catalog"
)

type Config struct {
	DatabaseURL string `envconfig:"DATABASE_URL"`
}

func main() {
	_ = godotenv.Load(".env")

	var cfg Config
	err := envconfig.Process("", &cfg)
	if err != nil {
		log.Fatal(err)
	}

	log.Println("database url:", cfg.DatabaseURL)

	var r catalog.Repository
	retry.ForeverSleep(2*time.Second, func(_ int) error {
		r, err = catalog.NewElasticRepository(cfg.DatabaseURL)
		if err != nil {
			log.Println(err)
		}
		return err
	})
	defer func() {
		if r != nil {
			r.Close()
		}
	}()

	log.Println("Listening on port 8080")
	s := catalog.NewService(r)
	log.Fatal(catalog.ListenGRPC(s, 8080))
}
