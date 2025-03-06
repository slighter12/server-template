package main

import (
	"log"

	"server-template/internal/domain/entity"

	"github.com/yanun0323/gem"
	"gorm.io/gen"
)

func main() {
	models := []any{
		entity.User{},
	}

	sql := gem.New(&gem.Config{
		Tool:              gem.Goose,
		OutputPath:        "./database/migrations/postgres",
		KeepDroppedColumn: true,
	})

	sql.AddModels(models...)

	if err := sql.Generate(); err != nil {
		log.Fatalf("run migrator, err: %+v", err)
	}

	gen := gen.NewGenerator(gen.Config{
		OutPath: "./internal/repository/gen/query",
	})

	gen.ApplyBasic(models...)

	gen.Execute()
}
